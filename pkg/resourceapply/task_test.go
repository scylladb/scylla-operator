package resourceapply

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/render"
	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/managerclient"
	"github.com/scylladb/scylla-operator/pkg/util/uuid"
)

func TestApplyTask(t *testing.T) {
	t.Parallel()

	testUUID := uuid.NewFromUint64(100, 200)
	newTask := func() *managerclient.Task {
		return &managerclient.Task{
			Name:       "task",
			Type:       "repair",
			Schedule:   &managerclient.Schedule{},
			Properties: make(map[string]interface{}),
		}
	}
	registeredTask1 := func() *managerclient.Task {
		rt := &managerclient.Task{
			Name:       "task",
			Type:       "repair",
			Schedule:   &managerclient.Schedule{},
			Properties: make(map[string]interface{}),
		}
		rt, err := CalculateAndSetHash(rt)
		if err != nil {
			t.Fatal(err)
		}
		rt.ID = testUUID.String()
		return rt
	}
	registeredTask2 := func() *managerclient.Task {
		rt := &managerclient.Task{
			Name: "task",
			Type: "repair",
			Schedule: &managerclient.Schedule{
				Timezone: "UTC",
			},
			Properties: make(map[string]interface{}),
		}
		rt, err := CalculateAndSetHash(rt)
		if err != nil {
			t.Fatal(err)
		}
		rt.ID = testUUID.String()
		return rt
	}
	tt := []struct {
		name           string
		requiredTask   *managerclient.Task
		tasks          []*managerclient.Task
		handler        http.Handler
		expectedChange bool
		expectedError  error
		expectedTask   *managerclient.Task
	}{
		{
			name:         "create task",
			requiredTask: newTask(),
			tasks:        nil,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", testUUID.String())
				w.WriteHeader(http.StatusCreated)
			}),
			expectedChange: true,
			expectedError:  nil,
			expectedTask:   registeredTask1(),
		},
		{
			name:         "update task",
			requiredTask: newTask(),
			tasks:        []*managerclient.Task{registeredTask2()},
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				render.Respond(w, r, registeredTask1())
			}),
			expectedChange: true,
			expectedError:  nil,
			expectedTask:   registeredTask1(),
		},
		{
			name:           "task will not change",
			requiredTask:   newTask(),
			tasks:          []*managerclient.Task{registeredTask1()},
			handler:        nil,
			expectedChange: false,
			expectedError:  nil,
			expectedTask:   registeredTask1(),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server, client := NewServerAndClient(t, tc.handler)
			defer server.Close()

			task, changed, err := ApplyTask(context.Background(), client, tc.requiredTask, tc.tasks)
			if err != tc.expectedError {
				t.Fatalf("expected: %v, got: %v", tc.expectedError, err)
			}
			if changed != tc.expectedChange {
				t.Fatal("expected the task to change")
			}
			if changed != tc.expectedChange {
				t.Fatalf("expected: %v, got: %v", tc.expectedChange, changed)
			}
			if diff := cmp.Diff(task, tc.expectedTask); diff != "" {
				t.Fatalf("Expected task is different from synced task exp: %v, sync: %v", tc.expectedTask, task)
			}
		})
	}
}
