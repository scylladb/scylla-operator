// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"fmt"

	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

type taskInfo struct {
	ClusterID uuid.UUID
	TaskType  TaskType
	TaskID    uuid.UUID
	TaskName  string
}

func newTaskInfoFromTask(t *Task) taskInfo {
	return taskInfo{
		ClusterID: t.ClusterID,
		TaskType:  t.Type,
		TaskID:    t.ID,
		TaskName:  t.Name,
	}
}

func (ti taskInfo) idKey() taskInfo {
	ti.TaskID = uuid.Nil
	return ti
}

func (ti taskInfo) String() string {
	return fmt.Sprintf("%s/%s", ti.TaskType, ti.TaskID)
}

type resolver struct {
	taskInfo map[uuid.UUID]taskInfo
	id       map[taskInfo]uuid.UUID
}

func newResolver() resolver {
	return resolver{
		taskInfo: make(map[uuid.UUID]taskInfo),
		id:       make(map[taskInfo]uuid.UUID),
	}
}

func (r resolver) Put(ti taskInfo) {
	old := r.taskInfo[ti.TaskID]
	delete(r.id, old.idKey())

	r.taskInfo[ti.TaskID] = ti
	if ti.TaskName != "" {
		r.id[ti.idKey()] = ti.TaskID
	}
}

func (r resolver) Remove(taskID uuid.UUID) {
	ti, ok := r.taskInfo[taskID]
	if !ok {
		return
	}
	delete(r.taskInfo, taskID)
	delete(r.id, ti.idKey())
}

func (r resolver) FindByID(taskID uuid.UUID) (taskInfo, bool) {
	ti, ok := r.taskInfo[taskID]
	return ti, ok
}

func (r resolver) FillTaskID(ti *taskInfo) bool {
	v := *ti
	v.TaskID = uuid.Nil

	taskID, ok := r.id[v.idKey()]
	if ok {
		ti.TaskID = taskID
	}
	return ok
}
