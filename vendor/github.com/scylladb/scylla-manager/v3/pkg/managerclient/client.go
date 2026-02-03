// Copyright (C) 2017 ScyllaDB

package managerclient

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	api "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"github.com/hbollon/go-edlib"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/util/pointer"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla-manager/client/operations"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla-manager/models"
)

var disableOpenAPIDebugOnce sync.Once

// Client provides means to interact with Scylla Manager.
type Client struct {
	operations operations.ClientService
}

// DefaultTransport specifies default HTTP transport to be used in NewClient if
// nil transport is provided.
var DefaultTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,

	TLSClientConfig: DefaultTLSConfig(),
}

// DefaultTLSConfig specifies default TLS configuration used when creating a new
// client.
var DefaultTLSConfig = func() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
	}
}

// Option allows decorating underlying HTTP client in NewClient.
type Option func(*http.Client)

func NewClient(rawURL string, opts ...Option) (Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Client{}, err
	}

	disableOpenAPIDebugOnce.Do(func() {
		middleware.Debug = false
	})

	httpClient := &http.Client{
		Transport: DefaultTransport,
	}
	for _, o := range opts {
		o(httpClient)
	}

	r := api.NewWithClient(u.Host, u.Path, []string{u.Scheme}, httpClient)
	// debug can be turned on by SWAGGER_DEBUG or DEBUG env variable
	// we change that to SCTOOL_DUMP_HTTP
	r.Debug, _ = strconv.ParseBool(os.Getenv("SCTOOL_DUMP_HTTP"))

	return Client{operations: operations.New(r, strfmt.Default)}, nil
}

// CreateCluster creates a new cluster.
func (c *Client) CreateCluster(ctx context.Context, cluster *Cluster) (string, error) {
	resp, err := c.operations.PostClusters(&operations.PostClustersParams{
		Context: ctx,
		Cluster: cluster,
	})
	if err != nil {
		return "", err
	}

	clusterID, err := uuidFromLocation(resp.Location)
	if err != nil {
		return "", errors.Wrap(err, "cannot parse response")
	}

	return clusterID.String(), nil
}

// GetCluster returns a cluster for a given ID.
func (c *Client) GetCluster(ctx context.Context, clusterID string) (*Cluster, error) {
	resp, err := c.operations.GetClusterClusterID(&operations.GetClusterClusterIDParams{
		Context:   ctx,
		ClusterID: clusterID,
	})
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

// UpdateCluster updates cluster.
func (c *Client) UpdateCluster(ctx context.Context, cluster *Cluster) error {
	_, err := c.operations.PutClusterClusterID(&operations.PutClusterClusterIDParams{ // nolint: errcheck
		Context:   ctx,
		ClusterID: cluster.ID,
		Cluster:   cluster,
	})
	return err
}

// DeleteCluster removes cluster.
func (c *Client) DeleteCluster(ctx context.Context, clusterID string) error {
	_, err := c.operations.DeleteClusterClusterID(&operations.DeleteClusterClusterIDParams{ // nolint: errcheck
		Context:   ctx,
		ClusterID: clusterID,
	})
	return err
}

// DeleteClusterSecrets removes cluster secrets.
func (c *Client) DeleteClusterSecrets(ctx context.Context, clusterID string, cqlCreds, alternatorCreds, sslUserCert bool) error {
	ok := false
	p := &operations.DeleteClusterClusterIDParams{
		Context:   ctx,
		ClusterID: clusterID,
	}
	if cqlCreds {
		p.CqlCreds = &cqlCreds
		ok = true
	}
	if alternatorCreds {
		p.AlternatorCreds = &alternatorCreds
		ok = true
	}
	if sslUserCert {
		p.SslUserCert = &sslUserCert
		ok = true
	}

	if !ok {
		return nil
	}

	_, err := c.operations.DeleteClusterClusterID(p) // nolint: errcheck
	return err
}

// ListClusters returns clusters.
func (c *Client) ListClusters(ctx context.Context) (ClusterSlice, error) {
	resp, err := c.operations.GetClusters(&operations.GetClustersParams{
		Context: ctx,
	})
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

// ClusterStatus returns health check progress.
func (c *Client) ClusterStatus(ctx context.Context, clusterID string) (ClusterStatus, error) {
	resp, err := c.operations.GetClusterClusterIDStatus(&operations.GetClusterClusterIDStatusParams{
		Context:   ctx,
		ClusterID: clusterID,
	})
	if err != nil {
		return nil, err
	}

	return ClusterStatus(resp.Payload), nil
}

// GetRepairTarget fetches information about repair target.
func (c *Client) GetRepairTarget(ctx context.Context, clusterID string, t *Task) (*RepairTarget, error) {
	resp, err := c.operations.GetClusterClusterIDTasksRepairTarget(&operations.GetClusterClusterIDTasksRepairTargetParams{
		Context:    ctx,
		ClusterID:  clusterID,
		TaskFields: makeTaskUpdate(t),
	})
	if err != nil {
		return nil, err
	}

	return &RepairTarget{RepairTarget: *resp.Payload}, nil
}

// GetBackupTarget fetches information about repair target.
func (c *Client) GetBackupTarget(ctx context.Context, clusterID string, t *Task) (*BackupTarget, error) {
	resp, err := c.operations.GetClusterClusterIDTasksBackupTarget(&operations.GetClusterClusterIDTasksBackupTargetParams{
		Context:    ctx,
		ClusterID:  clusterID,
		TaskFields: makeTaskUpdate(t),
	})
	if err != nil {
		return nil, err
	}

	return &BackupTarget{BackupTarget: *resp.Payload}, nil
}

// GetRestoreTarget fetches information about restore target.
func (c *Client) GetRestoreTarget(ctx context.Context, clusterID string, t *Task) (*RestoreTarget, error) {
	resp, err := c.operations.GetClusterClusterIDTasksRestoreTarget(&operations.GetClusterClusterIDTasksRestoreTargetParams{
		Context:    ctx,
		ClusterID:  clusterID,
		TaskFields: makeTaskUpdate(t),
	})
	if err != nil {
		return nil, err
	}

	return &RestoreTarget{RestoreTarget: *resp.Payload}, nil
}

// CreateTask creates a new task.
func (c *Client) CreateTask(ctx context.Context, clusterID string, t *Task) (uuid.UUID, error) {
	params := &operations.PostClusterClusterIDTasksParams{
		Context:    ctx,
		ClusterID:  clusterID,
		TaskFields: makeTaskUpdate(t),
	}
	resp, err := c.operations.PostClusterClusterIDTasks(params)
	if err != nil {
		return uuid.Nil, err
	}

	taskID, err := uuidFromLocation(resp.Location)
	if err != nil {
		return uuid.Nil, errors.Wrap(err, "cannot parse response")
	}

	return taskID, nil
}

// GetTask returns a task of a given type and ID.
func (c *Client) GetTask(ctx context.Context, clusterID, taskType string, taskID uuid.UUID) (*Task, error) {
	resp, err := c.operations.GetClusterClusterIDTaskTaskTypeTaskID(&operations.GetClusterClusterIDTaskTaskTypeTaskIDParams{
		Context:   ctx,
		ClusterID: clusterID,
		TaskType:  taskType,
		TaskID:    taskID.String(),
	})
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

// GetTaskHistory returns a run history of task of a given type and task ID.
func (c *Client) GetTaskHistory(ctx context.Context, clusterID, taskType string, taskID uuid.UUID, limit int64) (TaskRunSlice, error) {
	params := &operations.GetClusterClusterIDTaskTaskTypeTaskIDHistoryParams{
		Context:   ctx,
		ClusterID: clusterID,
		TaskType:  taskType,
		TaskID:    taskID.String(),
	}

	params.Limit = &limit

	resp, err := c.operations.GetClusterClusterIDTaskTaskTypeTaskIDHistory(params)
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

// StartTask starts executing a task.
func (c *Client) StartTask(ctx context.Context, clusterID, taskType string, taskID uuid.UUID, cont bool) error {
	_, err := c.operations.PutClusterClusterIDTaskTaskTypeTaskIDStart(&operations.PutClusterClusterIDTaskTaskTypeTaskIDStartParams{ // nolint: errcheck
		Context:   ctx,
		ClusterID: clusterID,
		TaskType:  taskType,
		TaskID:    taskID.String(),
		Continue:  cont,
	})

	return err
}

// StopTask stops executing a task.
func (c *Client) StopTask(ctx context.Context, clusterID, taskType string, taskID uuid.UUID, disable bool) error {
	_, err := c.operations.PutClusterClusterIDTaskTaskTypeTaskIDStop(&operations.PutClusterClusterIDTaskTaskTypeTaskIDStopParams{ // nolint: errcheck
		Context:   ctx,
		ClusterID: clusterID,
		TaskType:  taskType,
		TaskID:    taskID.String(),
		Disable:   &disable,
	})

	return err
}

// DeleteTask stops executing a task.
func (c *Client) DeleteTask(ctx context.Context, clusterID, taskType string, taskID uuid.UUID) error {
	_, err := c.operations.DeleteClusterClusterIDTaskTaskTypeTaskID(&operations.DeleteClusterClusterIDTaskTaskTypeTaskIDParams{ // nolint: errcheck
		Context:   ctx,
		ClusterID: clusterID,
		TaskType:  taskType,
		TaskID:    taskID.String(),
	})

	return err
}

// UpdateTask updates an existing task unit.
func (c *Client) UpdateTask(ctx context.Context, clusterID string, t *Task) error {
	_, err := c.operations.PutClusterClusterIDTaskTaskTypeTaskID(&operations.PutClusterClusterIDTaskTaskTypeTaskIDParams{ // nolint: errcheck
		Context:   ctx,
		ClusterID: clusterID,
		TaskType:  t.Type,
		TaskID:    t.ID,
		TaskFields: &models.TaskUpdate{
			Enabled:    t.Enabled,
			Name:       t.Name,
			Labels:     t.Labels,
			Schedule:   t.Schedule,
			Tags:       t.Tags,
			Properties: t.Properties,
		},
	})
	return err
}

// ListTasks returns tasks within a clusterID, optionally filtered by task type tp.
func (c *Client) ListTasks(ctx context.Context, clusterID, taskType string, all bool, status, taskID string) (TaskListItems, error) {
	resp, err := c.operations.GetClusterClusterIDTasks(&operations.GetClusterClusterIDTasksParams{
		Context:   ctx,
		ClusterID: clusterID,
		Type:      &taskType,
		All:       &all,
		Status:    &status,
		TaskID:    &taskID,
	})
	if err != nil {
		return TaskListItems{}, err
	}

	et := TaskListItems{
		All: all,
	}
	et.TaskListItemSlice = resp.Payload
	return et, nil
}

// TaskSplit is an extended version of the package level TaskSplit function.
// It adds support for providing task type only.
// If there is a single task of a given type the ID is returned.
// Otherwise an error is returned.
// If there is more tasks the error lists the available options.
func (c *Client) TaskSplit(ctx context.Context, cluster, s string) (taskType string, taskID uuid.UUID, err error) {
	var taskName string
	taskType, taskID, taskName, err = TaskSplit(s)
	if err != nil {
		return
	}

	if taskID != uuid.Nil {
		return taskType, taskID, nil
	}

	if taskName == "" {
		taskID, err = c.uniqueTaskID(ctx, cluster, taskType)
	} else {
		taskID, err = c.taskByName(ctx, cluster, taskType, taskName)
	}

	return
}

func (c *Client) uniqueTaskID(ctx context.Context, clusterID, taskType string) (uuid.UUID, error) {
	resp, err := c.operations.GetClusterClusterIDTasks(&operations.GetClusterClusterIDTasksParams{
		Context:   ctx,
		ClusterID: clusterID,
		Type:      &taskType,
		Short:     pointer.BoolPtr(true),
	})
	if err != nil {
		return uuid.Nil, err
	}

	tasks := resp.Payload
	switch len(tasks) {
	case 0:
		return uuid.Nil, errors.Errorf("no tasks of type %s", taskType)
	case 1:
		return uuid.Parse(tasks[0].ID)
	default:
		return uuid.Nil, errors.Errorf("task ambiguity, use one of:\n%s", formatTaskList(tasks))
	}
}

func (c *Client) taskByName(ctx context.Context, clusterID, taskType, taskName string) (uuid.UUID, error) {
	resp, err := c.operations.GetClusterClusterIDTasks(&operations.GetClusterClusterIDTasksParams{
		Context:   ctx,
		ClusterID: clusterID,
		Type:      &taskType,
		Short:     pointer.BoolPtr(true),
		All:       pointer.BoolPtr(true),
	})
	if err != nil {
		return uuid.Nil, err
	}

	tasks := resp.Payload
	if len(tasks) == 0 {
		return uuid.Nil, errors.Errorf("no tasks of type %s", taskType)
	}

	for _, t := range tasks {
		if t.Name == taskName {
			return uuid.Parse(t.ID)
		}
	}

	var names []string
	for _, t := range tasks {
		if t.Name != "" {
			names = append(names, t.Name)
		}
	}
	if len(names) > 0 {
		res, _ := edlib.FuzzySearch(taskName, names, edlib.Levenshtein) // nolint: errcheck
		if res != "" {
			return uuid.Nil, errors.Errorf("not found, did you mean %s", taskJoin(taskType, res))
		}
	}

	return uuid.Nil, errors.Errorf("not found, use one of:\n%s", formatTaskList(tasks))
}

func formatTaskList(tasks []*models.TaskListItem) string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		if t.Name != "" {
			ids[i] = "- " + taskJoin(t.Type, t.Name)
		} else {
			ids[i] = "- " + taskJoin(t.Type, t.ID)
		}
	}
	return strings.Join(ids, "\n")
}

// RepairProgress returns repair progress.
func (c *Client) RepairProgress(ctx context.Context, clusterID, taskID, runID string) (RepairProgress, error) {
	resp, err := c.operations.GetClusterClusterIDTaskRepairTaskIDRunID(&operations.GetClusterClusterIDTaskRepairTaskIDRunIDParams{
		Context:   ctx,
		ClusterID: clusterID,
		TaskID:    taskID,
		RunID:     runID,
	})
	if err != nil {
		return RepairProgress{}, err
	}

	return RepairProgress{
		TaskRunRepairProgress: resp.Payload,
	}, nil
}

// TabletRepairProgress returns tablet repair progress.
func (c *Client) TabletRepairProgress(ctx context.Context, clusterID, taskID, runID string) (TabletRepairProgress, error) {
	resp, err := c.operations.GetClusterClusterIDTaskTabletRepairTaskIDRunID(&operations.GetClusterClusterIDTaskTabletRepairTaskIDRunIDParams{
		Context:   ctx,
		ClusterID: clusterID,
		TaskID:    taskID,
		RunID:     runID,
	})
	if err != nil {
		return TabletRepairProgress{}, err
	}
	return TabletRepairProgress{
		TaskRunTabletRepairProgress: resp.GetPayload(),
	}, nil
}

// BackupProgress returns backup progress.
func (c *Client) BackupProgress(ctx context.Context, clusterID, taskID, runID string) (BackupProgress, error) {
	tr := &models.TaskRunBackupProgress{
		Progress: &models.BackupProgress{
			Stage: "INIT",
		},
		Run: &models.TaskRun{
			Status: "NEW",
		},
	}

	resp, err := c.operations.GetClusterClusterIDTaskBackupTaskIDRunID(&operations.GetClusterClusterIDTaskBackupTaskIDRunIDParams{
		Context:   ctx,
		ClusterID: clusterID,
		TaskID:    taskID,
		RunID:     runID,
	})
	if err != nil {
		return BackupProgress{
			TaskRunBackupProgress: tr,
		}, err
	}

	if resp.Payload.Progress == nil {
		resp.Payload.Progress = tr.Progress
	}
	if resp.Payload.Run == nil {
		resp.Payload.Run = tr.Run
	}

	return BackupProgress{
		TaskRunBackupProgress: resp.Payload,
	}, nil
}

// RestoreProgress returns restore progress.
func (c *Client) RestoreProgress(ctx context.Context, clusterID, taskID, runID string) (RestoreProgress, error) {
	resp, err := c.operations.GetClusterClusterIDTaskRestoreTaskIDRunID(&operations.GetClusterClusterIDTaskRestoreTaskIDRunIDParams{
		Context:   ctx,
		ClusterID: clusterID,
		TaskID:    taskID,
		RunID:     runID,
	})
	if err != nil {
		return RestoreProgress{}, err
	}

	return RestoreProgress{
		TaskRunRestoreProgress: resp.Payload,
	}, nil
}

// ValidateBackupProgress returns validate backup progress.
func (c *Client) ValidateBackupProgress(ctx context.Context, clusterID, taskID, runID string) (ValidateBackupProgress, error) {
	resp, err := c.operations.GetClusterClusterIDTaskValidateBackupTaskIDRunID(&operations.GetClusterClusterIDTaskValidateBackupTaskIDRunIDParams{
		Context:   ctx,
		ClusterID: clusterID,
		TaskID:    taskID,
		RunID:     runID,
	})
	if err != nil {
		return ValidateBackupProgress{}, err
	}

	return ValidateBackupProgress{
		TaskRunValidateBackupProgress: resp.Payload,
	}, nil
}

// BackupDescribeSchema returns backed up schema from DESCRIBE SCHEMA WITH INTERNALS query.
func (c *Client) BackupDescribeSchema(ctx context.Context, clusterID, snapshotTag, location, queryClusterID, queryTaskID string) (BackupDescribeSchema, error) {
	params := &operations.GetClusterClusterIDBackupsSchemaParams{
		Context:     ctx,
		ClusterID:   clusterID,
		SnapshotTag: snapshotTag,
		Location:    location,
	}
	if queryClusterID != "" {
		params.SetQueryClusterID(&queryClusterID)
	}
	if queryTaskID != "" {
		params.SetQueryTaskID(&queryTaskID)
	}

	resp, err := c.operations.GetClusterClusterIDBackupsSchema(params)
	if err != nil {
		return BackupDescribeSchema{}, err
	}
	return BackupDescribeSchema{
		BackupDescribeSchema: resp.GetPayload(),
	}, nil
}

// One2OneRestoreProgress returns 1-1-restore progress.
func (c *Client) One2OneRestoreProgress(ctx context.Context, clusterID, taskID, runID string) (One2OneRestoreProgress, error) {
	resp, err := c.operations.GetClusterClusterIDTask11RestoreTaskIDRunID(&operations.GetClusterClusterIDTask11RestoreTaskIDRunIDParams{
		Context:   ctx,
		ClusterID: clusterID,
		TaskID:    taskID,
		RunID:     runID,
	})
	if err != nil {
		return One2OneRestoreProgress{}, err
	}

	return One2OneRestoreProgress{
		TaskRunOne2OneRestoreProgress: resp.Payload,
	}, nil
}

// ListBackups returns listing of available backups.
func (c *Client) ListBackups(ctx context.Context, clusterID string,
	locations []string, allClusters bool, keyspace []string, minDate, maxDate time.Time,
) (BackupListItems, error) {
	p := &operations.GetClusterClusterIDBackupsParams{
		Context:   ctx,
		ClusterID: clusterID,
		Locations: locations,
		Keyspace:  keyspace,
	}
	if !allClusters {
		p.QueryClusterID = &clusterID
	}
	if !minDate.IsZero() {
		p.MinDate = (*strfmt.DateTime)(pointer.TimePtr(minDate))
	}
	if !maxDate.IsZero() {
		p.MaxDate = (*strfmt.DateTime)(pointer.TimePtr(maxDate))
	}

	resp, err := c.operations.GetClusterClusterIDBackups(p)
	if err != nil {
		return BackupListItems{}, err
	}

	return BackupListItems{items: resp.Payload}, nil
}

// ListBackupFiles returns a listing of available backup files.
func (c *Client) ListBackupFiles(ctx context.Context, clusterID string,
	locations []string, allClusters bool, keyspace []string, snapshotTag string,
) ([]*models.BackupFilesInfo, error) {
	p := &operations.GetClusterClusterIDBackupsFilesParams{
		Context:     ctx,
		ClusterID:   clusterID,
		Locations:   locations,
		Keyspace:    keyspace,
		SnapshotTag: snapshotTag,
	}
	if !allClusters {
		p.QueryClusterID = &clusterID
	}

	resp, err := c.operations.GetClusterClusterIDBackupsFiles(p)
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

// DeleteSnapshot deletes backup snapshot with all data associated with it.
func (c *Client) DeleteSnapshot(ctx context.Context, clusterID string,
	locations []string, snapshotTags []string,
) error {
	p := &operations.DeleteClusterClusterIDBackupsParams{
		Context:      ctx,
		ClusterID:    clusterID,
		Locations:    locations,
		SnapshotTags: snapshotTags,
	}

	_, err := c.operations.DeleteClusterClusterIDBackups(p) // nolint: errcheck
	return err
}

// DeleteLocalSnapshots deletes SM snapshots from nodes' disks.
func (c *Client) DeleteLocalSnapshots(ctx context.Context, clusterID string) error {
	p := &operations.DeleteClusterClusterIDBackupsLocalSnapshotsParams{
		Context:   ctx,
		ClusterID: clusterID,
	}
	_, err := c.operations.DeleteClusterClusterIDBackupsLocalSnapshots(p)
	return err
}

// Version returns server version.
func (c *Client) Version(ctx context.Context) (*models.Version, error) {
	resp, err := c.operations.GetVersion(&operations.GetVersionParams{
		Context: ctx,
	})
	if err != nil {
		return &models.Version{}, err
	}

	return resp.Payload, nil
}

// SetRepairIntensity updates ongoing repair intensity.
func (c *Client) SetRepairIntensity(ctx context.Context, clusterID string, intensity float64) error {
	p := &operations.PutClusterClusterIDRepairsIntensityParams{
		Context:   ctx,
		ClusterID: clusterID,
		Intensity: intensity,
	}

	_, err := c.operations.PutClusterClusterIDRepairsIntensity(p) // nolint: errcheck
	return err
}

// SetRepairParallel updates ongoing repair parallel disjoint host groups.
func (c *Client) SetRepairParallel(ctx context.Context, clusterID string, parallel int64) error {
	p := &operations.PutClusterClusterIDRepairsParallelParams{
		Context:   ctx,
		ClusterID: clusterID,
		Parallel:  parallel,
	}

	_, err := c.operations.PutClusterClusterIDRepairsParallel(p) // nolint: errcheck
	return err
}

// IsSuspended returns true iff the current cluster is suspended.
func (c *Client) IsSuspended(ctx context.Context, clusterID string) (bool, error) {
	p := &operations.GetClusterClusterIDSuspendedParams{
		Context:   ctx,
		ClusterID: clusterID,
	}

	s, err := c.operations.GetClusterClusterIDSuspended(p)
	if err != nil {
		return false, err
	}

	return bool(s.Payload), nil
}

// Suspend updates cluster suspended property.
func (c *Client) Suspend(ctx context.Context, clusterID string) error {
	p := &operations.PutClusterClusterIDSuspendedParams{
		Context:   ctx,
		ClusterID: clusterID,
		Suspended: true,
	}

	_, err := c.operations.PutClusterClusterIDSuspended(p) // nolint: errcheck
	return err
}

// SuspendParams describes additional params for suspending the cluster.
type SuspendParams struct {
	AllowedTaskType string
	SuspendPolicy   SuspendPolicy
	NoContinue      bool
}

// SuspendPolicy describes behavior towards running tasks (other than AllowedTaskType) when suspend is requested.
type SuspendPolicy = string

const (
	// SuspendPolicyStopRunningTasks results in stopping running tasks.
	SuspendPolicyStopRunningTasks SuspendPolicy = "stop_running_tasks"
	// SuspendPolicyFailIfRunningTasks results in failing to suspend cluster and returning ErrRunningTasks.
	SuspendPolicyFailIfRunningTasks SuspendPolicy = "fail_if_running_tasks"
)

// SuspendWithParams suspend the cluster.
func (c *Client) SuspendWithParams(ctx context.Context, clusterID string, sp SuspendParams) error {
	p := &operations.PutClusterClusterIDSuspendedParams{
		Context:    ctx,
		ClusterID:  clusterID,
		Suspended:  true,
		NoContinue: &sp.NoContinue,
	}
	if sp.AllowedTaskType != "" {
		p.SetAllowTaskType(&sp.AllowedTaskType)
	}
	if sp.SuspendPolicy != "" {
		p.SetSuspendPolicy(&sp.SuspendPolicy)
	}
	_, err := c.operations.PutClusterClusterIDSuspended(p)
	if err != nil {
		var conflictErr *operations.PutClusterClusterIDSuspendedConflict
		if errors.As(err, &conflictErr) {
			if conflictErr.GetPayload() != nil {
				return errors.Wrap(ErrRunningTasks, conflictErr.GetPayload().Message)
			}
			return ErrRunningTasks
		}

		return err
	}
	return nil
}

// Resume updates cluster suspended property.
func (c *Client) Resume(ctx context.Context, clusterID string, startTasks bool) error {
	p := &operations.PutClusterClusterIDSuspendedParams{
		Context:    ctx,
		ClusterID:  clusterID,
		StartTasks: startTasks,
		Suspended:  false,
	}

	_, err := c.operations.PutClusterClusterIDSuspended(p) // nolint: errcheck
	return err
}

// ResumeParams describes additional params for resuming the cluster.
type ResumeParams struct {
	StartTasks                 bool
	StartTasksMissedActivation bool
	NoContinue                 bool
}

// ResumeWithParams resumes the cluster.
func (c *Client) ResumeWithParams(ctx context.Context, clusterID string, rp ResumeParams) error {
	p := &operations.PutClusterClusterIDSuspendedParams{
		Context:                    ctx,
		ClusterID:                  clusterID,
		Suspended:                  false,
		StartTasks:                 rp.StartTasks,
		StartTasksMissedActivation: &rp.StartTasksMissedActivation,
		NoContinue:                 &rp.NoContinue,
	}
	_, err := c.operations.PutClusterClusterIDSuspended(p)
	return err
}

// SuspendDetails returns details about the cluster suspend state.
func (c *Client) SuspendDetails(ctx context.Context, clusterID string) (ClusterSuspendDetails, error) {
	p := &operations.GetClusterClusterIDSuspendedDetailsParams{
		Context:   ctx,
		ClusterID: clusterID,
	}

	resp, err := c.operations.GetClusterClusterIDSuspendedDetails(p)
	if err != nil {
		return ClusterSuspendDetails{}, err
	}

	return ClusterSuspendDetails{
		SuspendDetails: resp.Payload,
		ClusterID:      clusterID,
	}, nil
}
