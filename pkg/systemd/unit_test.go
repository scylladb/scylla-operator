package systemd

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
)

func TestUnitManager_WriteStatus(t *testing.T) {
	tt := []struct {
		name            string
		status          *unitManagerStatus
		expectedContent []byte
	}{
		{
			name:   "nil status writes null",
			status: nil,
			expectedContent: []byte(strings.TrimLeft(`
null
`, "\n")),
		},
		{
			name: "testing values are serialized correctly",
			status: &unitManagerStatus{
				ManagedUnits: []string{
					"foo",
					"bar",
				},
			},
			expectedContent: []byte(strings.TrimLeft(`
managedUnits:
    - foo
    - bar
`, "\n")),
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var err error

			tmpDir := t.TempDir()
			m := NewUnitManagerWithPath("test", tmpDir)

			err = m.WriteStatus(tc.status)
			if err != nil {
				t.Fatal(err)
			}

			statusFilePath := m.getStatusPath()
			got, err := os.ReadFile(statusFilePath)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, tc.expectedContent) {
				t.Fatalf("expected and got content differ:\n%s", cmp.Diff(string(tc.expectedContent), string(got)))
			}
		})
	}
}

func TestUnitManager_ReadStatus(t *testing.T) {
	tt := []struct {
		name           string
		content        []byte
		expectedStatus *unitManagerStatus
	}{
		{
			name: "null deserializes as empty status",
			content: []byte(strings.TrimLeft(`
null
`, "\n")),
			expectedStatus: &unitManagerStatus{},
		},
		{
			name: "testing values with no indents are deserialized correctly (yaml.v2 backward compatibility)",
			content: []byte(strings.TrimLeft(`
managedUnits:
- foo
- bar
`, "\n")),
			expectedStatus: &unitManagerStatus{
				ManagedUnits: []string{
					"foo",
					"bar",
				},
			},
		},
		{
			name: "testing values are deserialized correctly",
			content: []byte(strings.TrimLeft(`
managedUnits:
    - foo
    - bar
`, "\n")),
			expectedStatus: &unitManagerStatus{
				ManagedUnits: []string{
					"foo",
					"bar",
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var err error

			tmpDir := t.TempDir()
			m := NewUnitManagerWithPath("test", tmpDir)

			statusFilePath := m.getStatusPath()
			err = os.WriteFile(statusFilePath, tc.content, 0666)
			if err != nil {
				t.Fatal(err)
			}

			got, err := m.ReadStatus()
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, tc.expectedStatus) {
				t.Fatalf("expected and got status differ:\n%s", cmp.Diff(tc.expectedStatus, got))
			}
		})
	}
}

type NoopEnsureControl struct {
	unitStatuses map[string]UnitStatus
}

func NewNoopEnsureControl(unitStatuses []UnitStatus) *NoopEnsureControl {
	unitStatusesMap := make(map[string]UnitStatus, len(unitStatuses))
	for _, unitStatus := range unitStatuses {
		unitStatusesMap[unitStatus.Name] = unitStatus
	}

	return &NoopEnsureControl{
		unitStatuses: unitStatusesMap,
	}
}

var _ EnsureControlInterface = &NoopEnsureControl{}

func (f *NoopEnsureControl) DaemonReload(ctx context.Context) error {
	return nil
}

func (f *NoopEnsureControl) EnableUnit(ctx context.Context, unitFile string) error {
	return nil
}

func (f *NoopEnsureControl) StartUnit(ctx context.Context, unitFile string) error {
	return nil
}

func (f *NoopEnsureControl) DisableAndStopUnit(ctx context.Context, unitFile string) error {
	return nil
}

func (f *NoopEnsureControl) GetUnitStatuses(ctx context.Context, unitFiles []string) ([]UnitStatus, error) {
	var unitStatuses []UnitStatus

	for _, unitFile := range unitFiles {
		unitStatus, ok := f.unitStatuses[unitFile]
		if !ok {
			unitStatus = UnitStatus{
				Name:        unitFile,
				LoadState:   "not-found",
				ActiveState: "inactive",
			}
		}

		unitStatuses = append(unitStatuses, unitStatus)
	}

	return unitStatuses, nil
}

func Test_unitManager_EnsureUnits(t *testing.T) {
	tt := []struct {
		name                        string
		existingUnits               []*NamedUnit
		existingUnitStatuses        []UnitStatus
		status                      *unitManagerStatus
		desiredUnits                []*NamedUnit
		expectedUnits               []*NamedUnit
		expectedStatus              *unitManagerStatus
		expectedProgressingMessages []string
		expectedErrFunc             func(string) error
		expectedEvents              []string
	}{
		{
			name:                 "writing first unit succeeds",
			existingUnits:        nil,
			existingUnitStatuses: nil,
			status:               nil,
			desiredUnits: []*NamedUnit{
				{
					FileName: "foo.mount",
					Data:     []byte("bar"),
				},
			},
			expectedStatus: &unitManagerStatus{
				ManagedUnits: []string{
					"foo.mount",
				},
			},
			expectedUnits: []*NamedUnit{
				{
					FileName: "foo.mount",
					Data:     []byte("bar"),
				},
			},
			expectedProgressingMessages: []string{
				`Awaiting unit "foo.mount" to be in active state "active".`,
			},
			expectedErrFunc: nil,
			expectedEvents: []string{
				"Normal MountCreated Mount unit foo.mount has been created",
			},
		},
		{
			name: "old units get pruned but unmanaged units stay",
			existingUnits: []*NamedUnit{
				{
					FileName: "foreign.mount",
					Data:     []byte("foreign"),
				},
				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
				{
					FileName: "old.mount",
					Data:     []byte("old"),
				},
			},
			existingUnitStatuses: []UnitStatus{
				{
					Name:        "foreign.mount",
					LoadState:   "loaded",
					ActiveState: "active",
				},
				{
					Name:        "managed.mount",
					LoadState:   "loaded",
					ActiveState: "active",
				},
				{
					Name:        "old.mount",
					LoadState:   "loaded",
					ActiveState: "active",
				},
			},
			status: &unitManagerStatus{
				ManagedUnits: []string{
					"managed.mount",
					"old.mount",
				},
			},
			desiredUnits: []*NamedUnit{
				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
				{
					FileName: "new.mount",
					Data:     []byte("new"),
				},
			},
			expectedUnits: []*NamedUnit{
				{
					FileName: "foreign.mount",
					Data:     []byte("foreign"),
				},
				{
					FileName: "new.mount",
					Data:     []byte("new"),
				},
				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
			},
			expectedStatus: &unitManagerStatus{
				ManagedUnits: []string{
					"managed.mount",
					"new.mount",
				},
			},
			expectedProgressingMessages: []string{
				`Awaiting unit "new.mount" to be in active state "active".`,
			},
			expectedErrFunc: nil,
			expectedEvents: []string{
				"Normal MountDeleted Mount unit old.mount has been deleted",
				"Normal MountCreated Mount unit new.mount has been created",
			},
		},
		{
			name: "reconciling existing, unmanaged unit results in an error",
			existingUnits: []*NamedUnit{
				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
			},
			existingUnitStatuses: []UnitStatus{
				{
					Name:        "foreign.mount",
					LoadState:   "loaded",
					ActiveState: "active",
				},
				{
					Name:        "managed.mount",
					LoadState:   "loaded",
					ActiveState: "active",
				},
			},
			status: &unitManagerStatus{
				ManagedUnits: []string{
					"managed.mount",
				},
			},
			desiredUnits: []*NamedUnit{
				{
					FileName: "foreign.mount",
					Data:     []byte("managed"),
				},
				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
			},
			expectedUnits: []*NamedUnit{
				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
			},
			expectedStatus: &unitManagerStatus{
				ManagedUnits: []string{
					"managed.mount",
				},
			},
			expectedProgressingMessages: nil,
			expectedErrFunc: func(_ string) error {
				return apimachineryutilerrors.NewAggregate([]error{
					fmt.Errorf(`required unit "foreign.mount" already exists and is not managed by us`),
				})
			},
			expectedEvents: nil,
		},
		{
			name: "reconciling unit in a failed state propagates an error",
			existingUnits: []*NamedUnit{
				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
				{
					FileName: "failed.mount",
					Data:     []byte("failed"),
				},
			},
			existingUnitStatuses: []UnitStatus{
				{
					Name:        "managed.mount",
					LoadState:   "loaded",
					ActiveState: "active",
				},
				{
					Name:        "failed.mount",
					LoadState:   "loaded",
					ActiveState: "failed",
				},
			},
			status: &unitManagerStatus{
				ManagedUnits: []string{
					"managed.mount",
					"failed.mount",
				},
			},
			desiredUnits: []*NamedUnit{
				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
				{
					FileName: "failed.mount",
					Data:     []byte("failed"),
				},
			},
			expectedUnits: []*NamedUnit{
				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
				{
					FileName: "failed.mount",
					Data:     []byte("failed"),
				},
			},
			expectedStatus: &unitManagerStatus{
				ManagedUnits: []string{
					"managed.mount",
					"failed.mount",
				},
			},
			expectedProgressingMessages: []string{
				`Awaiting unit "failed.mount" to be in active state "active".`,
			},
			expectedErrFunc: func(_ string) error {
				return apimachineryutilerrors.NewAggregate([]error{
					fmt.Errorf(`unit "failed.mount" is in a "failed" active state`),
				})
			},
			expectedEvents: nil,
		},
		{
			name: "errors coming from reconciling unmanaged and failed units are aggregated",
			existingUnits: []*NamedUnit{

				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
				{
					FileName: "failed.mount",
					Data:     []byte("failed"),
				},
			},
			existingUnitStatuses: []UnitStatus{
				{
					Name:        "foreign.mount",
					LoadState:   "loaded",
					ActiveState: "active",
				},
				{
					Name:        "managed.mount",
					LoadState:   "loaded",
					ActiveState: "active",
				},
				{
					Name:        "failed.mount",
					LoadState:   "loaded",
					ActiveState: "failed",
				},
			},
			status: &unitManagerStatus{
				ManagedUnits: []string{
					"managed.mount",
					"failed.mount",
				},
			},
			desiredUnits: []*NamedUnit{
				{
					FileName: "foreign.mount",
					Data:     []byte("managed"),
				},
				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
				{
					FileName: "failed.mount",
					Data:     []byte("failed"),
				},
			},
			expectedUnits: []*NamedUnit{
				{
					FileName: "managed.mount",
					Data:     []byte("managed"),
				},
				{
					FileName: "failed.mount",
					Data:     []byte("failed"),
				},
			},
			expectedStatus: &unitManagerStatus{
				ManagedUnits: []string{
					"managed.mount",
					"failed.mount",
				},
			},
			expectedProgressingMessages: []string{
				`Awaiting unit "failed.mount" to be in active state "active".`,
			},
			expectedErrFunc: func(_ string) error {
				return apimachineryutilerrors.NewAggregate([]error{
					fmt.Errorf(`required unit "foreign.mount" already exists and is not managed by us`),
					fmt.Errorf(`unit "failed.mount" is in a "failed" active state`),
				})
			},
			expectedEvents: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx, ctxCancel := context.WithCancel(context.Background())
			defer ctxCancel()

			var err error

			recorder := record.NewFakeRecorder(10)

			tmpDir := t.TempDir()
			m := NewUnitManagerWithPath("test", tmpDir)

			for _, existingUnit := range tc.existingUnits {
				existingUnitPath := m.GetUnitPath(existingUnit.FileName)
				err = os.WriteFile(existingUnitPath, existingUnit.Data, 0666)
				if err != nil {
					t.Fatal(err)
				}
			}

			if tc.status != nil {
				err = m.WriteStatus(tc.status)
				if err != nil {
					t.Fatal(err)
				}
			}

			var expectedErr error
			if tc.expectedErrFunc != nil {
				expectedErr = tc.expectedErrFunc(tmpDir)
			}

			control := NewNoopEnsureControl(tc.existingUnitStatuses)

			var progressingMessages []string
			progressingMessages, err = m.EnsureUnits(ctx, nil, recorder, tc.desiredUnits, control)
			if !reflect.DeepEqual(progressingMessages, tc.expectedProgressingMessages) {
				t.Fatalf("expected and got progressing messages differ:\n%s", cmp.Diff(tc.expectedProgressingMessages, progressingMessages))
			}
			if !reflect.DeepEqual(err, expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s", cmp.Diff(expectedErr, err, cmpopts.EquateErrors()))
			}
			if err != nil {
				return
			}

			// Verify expected status.
			status, err := m.ReadStatus()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(status, tc.expectedStatus) {
				t.Fatalf("expected and got status differ:\n%s", cmp.Diff(tc.expectedStatus, status))
			}

			// Verify expected units.
			for _, expectedUnit := range tc.expectedUnits {
				expectedUnitPath := m.GetUnitPath(expectedUnit.FileName)
				data, err := os.ReadFile(expectedUnitPath)
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(data, expectedUnit.Data) {
					t.Fatalf("expected and got data for unit %q differ:\n%s", expectedUnit.FileName, cmp.Diff(string(expectedUnit.Data), string(data)))
				}
			}

			entries, err := os.ReadDir(tmpDir)
			if err != nil {
				t.Fatal(err)
			}

			unexpectedEntries := oslices.Filter(entries, func(entry os.DirEntry) bool {
				return !entry.IsDir() &&
					entry.Name() != m.getStatusName() &&
					!oslices.Contains(
						tc.expectedUnits,
						func(v *NamedUnit) bool {
							return v.FileName == entry.Name()
						},
					)
			})

			if len(unexpectedEntries) != 0 {
				unexpectedEntryNames := oslices.ConvertSlice(unexpectedEntries, func(entry os.DirEntry) string {
					return entry.Name()
				})
				t.Errorf("Unexpected files were created: %q", strings.Join(unexpectedEntryNames, ","))
			}

			close(recorder.Events)
			var gotEvents []string
			for e := range recorder.Events {
				gotEvents = append(gotEvents, e)
			}

			if !reflect.DeepEqual(gotEvents, tc.expectedEvents) {
				t.Errorf("expected %v, got %v, diff:\n%s", tc.expectedEvents, gotEvents, cmp.Diff(tc.expectedEvents, gotEvents))
			}
		})
	}
}
