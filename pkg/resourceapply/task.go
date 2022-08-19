package resourceapply

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/managerclient"
	hashutil "github.com/scylladb/scylla-operator/pkg/util/hash"
	"k8s.io/klog/v2"
)

func ApplyTask(
	ctx context.Context,
	client *managerclient.Client,
	requiredTask *managerclient.Task,
	tasks []*managerclient.Task,
) (*managerclient.Task, bool, error) {
	requiredTask, err := CalculateAndSetHash(requiredTask)
	if err != nil {
		return requiredTask, false, err
	}

	for _, task := range tasks {
		if task.Name == requiredTask.Name && task.ClusterID == requiredTask.ClusterID && task.Type == requiredTask.Type {
			// The Task is already registered in manager.
			// It must have the same ID to be recognized during update.
			requiredTask.ID = task.ID

			diff, err := diffTasks(task, requiredTask)
			if err != nil {
				return requiredTask, false, err
			}
			if diff {
				err := client.UpdateTask(ctx, requiredTask.ClusterID, requiredTask)
				if err != nil {
					return requiredTask, true, fmt.Errorf("updating task %v of type %v, clusterID %v, %v",
						requiredTask.Name, requiredTask.Type, requiredTask.ClusterID, err)
				}
				klog.V(2).InfoS("updated task", "type", requiredTask.Type,
					"task", requiredTask.Name, "cluster", requiredTask.ClusterID)
				return requiredTask, true, nil
			}

			return requiredTask, false, nil
		}
	}

	id, err := client.CreateTask(ctx, requiredTask.ClusterID, requiredTask)
	requiredTask.ID = id.String()
	if err != nil {
		return requiredTask, true, fmt.Errorf("creating task %v of type %v, clusterID %v, %v",
			requiredTask.Name, requiredTask.Type, requiredTask.ClusterID, err)
	}

	klog.V(2).InfoS("created task", "type", requiredTask.Type,
		"task", requiredTask.Name, "cluster", requiredTask.ClusterID)
	return requiredTask, true, nil
}

const TaskHash = "scylla-manager-operator:hash-of-task"

func CalculateAndSetHash(t *managerclient.Task) (*managerclient.Task, error) {
	h, err := hashutil.HashObjects(t)
	if err != nil {
		return t, fmt.Errorf("task %v cannot be hashed: %v", t.Name, err)
	}

	if p, ok := t.Properties.(map[string]interface{}); ok {
		p[TaskHash] = h
		t.Properties = p
		return t, nil
	}

	return t, fmt.Errorf("hash cannot be set in task %v", t.Name)
}

func diffTasks(t1, t2 *managerclient.Task) (bool, error) {
	if p1, ok := t1.Properties.(map[string]interface{}); ok {
		if h1, ok := p1[TaskHash]; ok {
			if p2, ok := t2.Properties.(map[string]interface{}); ok {
				if h2, ok := p2[TaskHash]; ok {
					return h1 != h2, nil
				}
			}
		}
	}
	return true, fmt.Errorf("tasks %v and %v cannot be compared", t1.Name, t2.Name)
}
