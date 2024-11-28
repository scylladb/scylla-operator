package systemd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

var ErrNotExist = errors.New("unit does not exist")

type SystemdControl struct {
	conn *dbus.Conn
}

func transformSystemdError(err error) error {
	var godbusErr godbus.Error
	if errors.As(err, &godbusErr) {
		switch godbusErr.Name {
		case "org.freedesktop.systemd1.NoSuchUnit":
			return ErrNotExist
		default:
			return err
		}
	}

	return err
}

func newSystemdControl(conn *dbus.Conn) (*SystemdControl, error) {
	return &SystemdControl{
		conn: conn,
	}, nil
}
func NewSystemdSystemControl(ctx context.Context) (*SystemdControl, error) {
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't create dbus connection to systemd: %w", err)
	}

	return newSystemdControl(conn)
}

func NewSystemdUserControl(ctx context.Context) (*SystemdControl, error) {
	conn, err := dbus.NewUserConnectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't create dbus connection to user's systemd: %w", err)
	}

	return newSystemdControl(conn)
}

func (c *SystemdControl) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *SystemdControl) DaemonReload(ctx context.Context) error {
	err := c.conn.ReloadContext(ctx)
	if err != nil {
		return fmt.Errorf("can't reload systemd configs: %w", err)
	}

	return nil
}

func (c *SystemdControl) EnableUnits(ctx context.Context, unitFiles []string) error {
	_, _, err := c.conn.EnableUnitFilesContext(ctx, unitFiles, false, false)
	if err != nil {
		return fmt.Errorf("can't enable units %q: %w", strings.Join(unitFiles, ", "), transformSystemdError(err))
	}

	return nil
}

func (c *SystemdControl) DisableUnits(ctx context.Context, unitFiles []string) error {
	_, err := c.conn.DisableUnitFilesContext(ctx, unitFiles, false)
	if err != nil {
		return fmt.Errorf("can't disable units %q: %w", strings.Join(unitFiles, ", "), transformSystemdError(err))
	}

	return nil
}

func (c *SystemdControl) EnableUnit(ctx context.Context, unitFile string) error {
	return c.EnableUnits(ctx, []string{unitFile})
}

func (c *SystemdControl) DisableUnit(ctx context.Context, unitFile string) error {
	return c.DisableUnits(ctx, []string{unitFile})
}

func (c *SystemdControl) StartUnit(ctx context.Context, unitFile string) error {
	_, err := c.conn.StartUnitContext(ctx, unitFile, "replace", nil)
	if err != nil {
		return fmt.Errorf("can't start unit %q: %w", unitFile, transformSystemdError(err))
	}

	return nil
}

func (c *SystemdControl) StopUnit(ctx context.Context, unitFile string) error {
	_, err := c.conn.StopUnitContext(ctx, unitFile, "replace", nil)
	if err != nil {
		return fmt.Errorf("can't stop unit %q: %w", unitFile, transformSystemdError(err))
	}

	return nil
}

func (c *SystemdControl) DisableAndStopUnit(ctx context.Context, unitFile string) error {
	err := c.DisableUnit(ctx, unitFile)
	if err != nil {
		return err
	}

	err = c.StopUnit(ctx, unitFile)
	if err != nil {
		return err
	}

	return nil
}

func (c *SystemdControl) GetUnitStatuses(ctx context.Context, unitFiles []string) ([]UnitStatus, error) {
	dbusUnitStatuses, err := c.conn.ListUnitsContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't list units %q: %w", strings.Join(unitFiles, ", "), transformSystemdError(err))
	}

	unitNameSet := sets.New(unitFiles...)
	dbusUnitStatuses = slices.Filter(dbusUnitStatuses, func(us dbus.UnitStatus) bool {
		return unitNameSet.Has(us.Name)
	})

	unitStatuses := slices.ConvertSlice(dbusUnitStatuses, func(us dbus.UnitStatus) UnitStatus {
		return UnitStatus{
			Name:        us.Name,
			LoadState:   us.LoadState,
			ActiveState: us.ActiveState,
		}
	})

	getUnitName := func(us UnitStatus) string {
		return us.Name
	}
	missingUnitNames := unitNameSet.Difference(sets.New(slices.ConvertSlice(unitStatuses, getUnitName)...)).UnsortedList()

	var unitPropertiesErrs []error
	for _, name := range missingUnitNames {
		var info map[string]interface{}

		info, err = c.conn.GetUnitPropertiesContext(ctx, name)
		if err != nil {
			unitPropertiesErrs = append(unitPropertiesErrs, fmt.Errorf("can't get properties of unit %q: %w", name, transformSystemdError(err)))
			continue
		}

		us := UnitStatus{
			Name:        name,
			LoadState:   loadStateNotFound,
			ActiveState: activeStateInactive,
		}

		if loadStateString, ok := info["LoadState"].(string); ok {
			us.LoadState = loadStateString
		}
		if activeStateString, ok := info["ActiveState"].(string); ok {
			us.ActiveState = activeStateString
		}

		unitStatuses = append(unitStatuses, us)
	}

	err = utilerrors.NewAggregate(unitPropertiesErrs)
	if err != nil {
		return nil, err
	}

	return unitStatuses, nil
}
