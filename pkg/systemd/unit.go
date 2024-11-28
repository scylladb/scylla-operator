package systemd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type NamedUnit struct {
	FileName string
	Data     []byte
}

type unitManagerStatus struct {
	ManagedUnits []string `yaml:"managedUnits"`
}

func newUnitManagerStatus() *unitManagerStatus {
	return &unitManagerStatus{}
}

type UnitManager struct {
	rootPath string
	manager  string
}

func NewUnitManagerWithPath(manager, rootPath string) *UnitManager {
	return &UnitManager{
		rootPath: rootPath,
		manager:  manager,
	}
}

func NewUnitManager(manager string) *UnitManager {
	return NewUnitManagerWithPath(manager, "/etc/systemd/system/")
}

func (m *UnitManager) GetUnitPath(name string) string {
	return filepath.Join(m.rootPath, name)
}

func (m *UnitManager) getStatusName() string {
	return fmt.Sprintf(".%s.unit-manager-status.yaml", m.manager)
}

func (m *UnitManager) getStatusPath() string {
	return path.Join(m.rootPath, m.getStatusName())
}

func (m *UnitManager) ReadStatus() (*unitManagerStatus, error) {
	statusFile := m.getStatusPath()
	data, err := os.ReadFile(statusFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return newUnitManagerStatus(), nil
		}
		return nil, fmt.Errorf("can't open status file %q: %w", statusFile, err)
	}

	status := newUnitManagerStatus()
	err = yaml.Unmarshal(data, status)
	if err != nil {
		return nil, fmt.Errorf("can't decode unit manager status: %w", err)
	}

	return status, nil
}

func (m *UnitManager) WriteStatus(status *unitManagerStatus) error {
	data, err := yaml.Marshal(status)
	if err != nil {
		return fmt.Errorf("can't encode unit manager status: %w", err)
	}

	statusFile := m.getStatusPath()
	err = os.WriteFile(statusFile, data, 0666)
	if err != nil {
		return fmt.Errorf("can't write status file %q: %w", statusFile, err)
	}

	return nil
}

const (
	loadStateNotFound = "not-found"

	activeStateActive   = "active"
	activeStateInactive = "inactive"
	activeStateFailed   = "failed"
)

type UnitStatus struct {
	Name        string
	LoadState   string
	ActiveState string
}

type EnsureControlInterface interface {
	DaemonReload(ctx context.Context) error
	EnableUnit(ctx context.Context, unitFile string) error
	StartUnit(ctx context.Context, unitFile string) error
	DisableAndStopUnit(ctx context.Context, unitFile string) error
	GetUnitStatuses(ctx context.Context, unitFiles []string) ([]UnitStatus, error)
}

// EnsureUnits will make sure to remove any unit that is no longer desired and create/update those that are.
func (m *UnitManager) EnsureUnits(ctx context.Context, nc *scyllav1alpha1.NodeConfig, recorder record.EventRecorder, requiredUnits []*NamedUnit, control EnsureControlInterface) ([]string, error) {
	var progressingMessages []string

	status, err := m.ReadStatus()
	if err != nil {
		return progressingMessages, fmt.Errorf("can't list managed units: %w", err)
	}

	klog.V(4).InfoS(
		"Checking if units need pruning",
		"Existing", len(status.ManagedUnits),
		"Desired", len(requiredUnits),
	)

	// Reload unit definitions for pruning in case that we didn't make it to daemon reload when writing them.
	err = control.DaemonReload(ctx)
	if err != nil {
		return progressingMessages, fmt.Errorf("can't reload systemd: %w", err)
	}

	for _, existingUnitName := range status.ManagedUnits {
		isRequired := false
		for _, requiredUnit := range requiredUnits {
			if existingUnitName == requiredUnit.FileName {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		klog.V(2).InfoS("Disabling and stopping unit because it's no longer required", "Name", existingUnitName)
		err = control.DisableAndStopUnit(ctx, existingUnitName)
		if err != nil {
			if errors.Is(err, ErrNotExist) {
				klog.V(2).InfoS("Skipped disabling and stopping unit that doesn't exist", "Name", existingUnitName)
			} else {
				return progressingMessages, fmt.Errorf("can't disable unit %q: %w", existingUnitName, err)
			}
		}

		klog.V(2).InfoS("Removing unit because it's no longer required", "Name", existingUnitName)
		existingUnitPath := m.GetUnitPath(existingUnitName)
		err = os.Remove(existingUnitPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return progressingMessages, fmt.Errorf("can't prune unit %q: %w", existingUnitName, err)
		}
		recorder.Eventf(
			nc,
			corev1.EventTypeNormal,
			"MountDeleted",
			"Mount unit %s has been deleted",
			existingUnitName,
		)
	}

	unitStatuses, err := control.GetUnitStatuses(ctx, slices.ConvertSlice(requiredUnits, func(unit *NamedUnit) string {
		return unit.FileName
	}))
	if err != nil {
		return progressingMessages, fmt.Errorf("can't get unit statuses: %w", err)
	}

	foundUnitStatuses := slices.FilterOut(unitStatuses, func(status UnitStatus) bool {
		return status.LoadState == loadStateNotFound
	})
	existingUnitsSet := sets.New(slices.ConvertSlice(foundUnitStatuses, func(status UnitStatus) string {
		return status.Name
	})...)

	managedUnitsSet := sets.New(status.ManagedUnits...)

	// Do not try to reconcile units which are not managed by us.
	var managedRequiredUnits []*NamedUnit
	var errs []error
	for _, requiredUnit := range requiredUnits {
		if existingUnitsSet.Has(requiredUnit.FileName) && !managedUnitsSet.Has(requiredUnit.FileName) {
			errs = append(errs, fmt.Errorf("required unit %q already exists and is not managed by us", requiredUnit.FileName))
			continue
		}

		managedRequiredUnits = append(managedRequiredUnits, requiredUnit)
	}

	// First save the updated list of managed units,
	// so we can clean up in the next run, if we were interrupted.
	status.ManagedUnits = slices.ConvertSlice(managedRequiredUnits, func(unit *NamedUnit) string {
		return unit.FileName
	})

	err = m.WriteStatus(status)
	if err != nil {
		return progressingMessages, fmt.Errorf("can't write status: %w", err)
	}

	for _, managedRequiredUnit := range managedRequiredUnits {
		klog.V(4).InfoS("Ensuring unit", "Name", managedRequiredUnit.FileName)
		managedRequiredUnitPath := m.GetUnitPath(managedRequiredUnit.FileName)

		exists := true
		_, err = os.Stat(managedRequiredUnitPath)
		if err != nil {
			if os.IsNotExist(err) {
				exists = false
			} else {
				return progressingMessages, fmt.Errorf("can't stat unit %q: %w", managedRequiredUnitPath, err)
			}
		}

		err = os.WriteFile(managedRequiredUnitPath, managedRequiredUnit.Data, 0666)
		if err != nil {
			return progressingMessages, fmt.Errorf("can't write unit %q: %w", managedRequiredUnitPath, err)
		}

		if !exists {
			klog.V(2).InfoS("Mount unit has been created", "Name", managedRequiredUnit.FileName)
			recorder.Eventf(
				nc,
				corev1.EventTypeNormal,
				"MountCreated",
				"Mount unit %s has been created",
				managedRequiredUnit.FileName,
			)
		}
	}

	// Reload unit definitions to enable and start them.
	err = control.DaemonReload(ctx)
	if err != nil {
		return progressingMessages, fmt.Errorf("can't reload systemd: %w", err)
	}

	for _, managedRequiredUnit := range managedRequiredUnits {
		klog.V(2).InfoS("Enabling unit", "Name", managedRequiredUnit.FileName)
		err = control.EnableUnit(ctx, managedRequiredUnit.FileName)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't enable unit %q: %w", managedRequiredUnit.FileName, err))
			continue
		}

		unitStatuses, err = control.GetUnitStatuses(ctx, []string{managedRequiredUnit.FileName})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get unit statuses for unit %q: %w", managedRequiredUnit.FileName, err))
			continue
		}
		if len(unitStatuses) == 0 {
			errs = append(errs, fmt.Errorf("missing status for unit %q", managedRequiredUnit.FileName))
			continue
		}
		unitStatus := unitStatuses[0]

		switch unitStatus.ActiveState {
		case activeStateActive:
			// Unit is already in an "active" active state, no condition to set.

		case activeStateFailed:
			// Append error to propagate a degraded condition but do not break early to retry starting the unit.
			errs = append(errs, fmt.Errorf("unit %q is in a %q active state", managedRequiredUnit.FileName, activeStateFailed))
			fallthrough

		default:
			progressingMessages = append(progressingMessages, fmt.Sprintf("Awaiting unit %q to be in active state %q.", managedRequiredUnit.FileName, activeStateActive))
		}

		klog.V(2).InfoS("Starting unit", "Name", managedRequiredUnit.FileName)
		err = control.StartUnit(ctx, managedRequiredUnit.FileName)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't start unit %q: %w", managedRequiredUnit.FileName, err))
			continue
		}
	}

	return progressingMessages, apierrors.NewAggregate(errs)
}
