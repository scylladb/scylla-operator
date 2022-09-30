// Copyright (C) 2017 ScyllaDB

package managerclient

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	api "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/managerclient/gen/client/operations"
	"github.com/scylladb/scylla-operator/pkg/managerclient/gen/models"
	"github.com/scylladb/scylla-operator/pkg/util/uuid"
)

var disableOpenAPIDebugOnce sync.Once

//go:generate ./internalgen.sh

// Client provides means to interact with Manager.
type Client struct {
	operations operations.ClientService
}

// DefaultTLSConfig specifies default TLS configuration used when creating a new
// client.
var DefaultTLSConfig = func() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
	}
}

func NewClient(rawURL string, transport http.RoundTripper) (Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Client{}, err
	}

	disableOpenAPIDebugOnce.Do(func() {
		middleware.Debug = false
	})

	if transport == nil {
		transport = &http.Transport{
			TLSClientConfig: DefaultTLSConfig(),
		}
	}

	httpClient := &http.Client{
		Transport: transport,
	}

	r := api.NewWithClient(u.Host, u.Path, []string{u.Scheme}, httpClient)
	// debug can be turned on by SWAGGER_DEBUG or DEBUG env variable
	// we change that to SCTOOL_DUMP_HTTP
	r.Debug, _ = strconv.ParseBool(os.Getenv("SCTOOL_DUMP_HTTP"))

	return Client{operations: operations.New(r, strfmt.Default)}, nil
}

// CreateCluster creates a new cluster.
func (c Client) CreateCluster(ctx context.Context, cluster *Cluster) (string, error) {
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
func (c Client) GetCluster(ctx context.Context, clusterID string) (*Cluster, error) {
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
func (c Client) UpdateCluster(ctx context.Context, cluster *Cluster) error {
	_, err := c.operations.PutClusterClusterID(&operations.PutClusterClusterIDParams{ // nolint: errcheck
		Context:   ctx,
		ClusterID: cluster.ID,
		Cluster:   cluster,
	})
	return err
}

// DeleteCluster removes cluster.
func (c Client) DeleteCluster(ctx context.Context, clusterID string) error {
	_, err := c.operations.DeleteClusterClusterID(&operations.DeleteClusterClusterIDParams{ // nolint: errcheck
		Context:   ctx,
		ClusterID: clusterID,
	})
	return err
}

// DeleteClusterSecrets removes cluster secrets.
func (c Client) DeleteClusterSecrets(ctx context.Context, clusterID string, cqlCreds, sslUserCert bool) error {
	ok := false
	p := &operations.DeleteClusterClusterIDParams{
		Context:   ctx,
		ClusterID: clusterID,
	}
	if cqlCreds {
		p.CqlCreds = &cqlCreds
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
func (c Client) ListClusters(ctx context.Context) (ClusterSlice, error) {
	resp, err := c.operations.GetClusters(&operations.GetClustersParams{
		Context: ctx,
	})
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

// ClusterStatus returns health check progress.
func (c Client) ClusterStatus(ctx context.Context, clusterID string) (ClusterStatus, error) {
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
			Schedule:   t.Schedule,
			Tags:       t.Tags,
			Properties: t.Properties,
		},
	})
	return err
}

// ListTasks returns uled tasks within a clusterID, optionaly filtered by task type tp.
func (c *Client) ListTasks(ctx context.Context, clusterID, taskType string, all bool, status string) (ExtendedTasks, error) {
	resp, err := c.operations.GetClusterClusterIDTasks(&operations.GetClusterClusterIDTasksParams{
		Context:   ctx,
		ClusterID: clusterID,
		Type:      &taskType,
		All:       &all,
		Status:    &status,
	})
	if err != nil {
		return ExtendedTasks{}, err
	}

	et := ExtendedTasks{
		All: all,
	}
	et.ExtendedTaskSlice = resp.Payload
	return et, nil
}

// RepairProgress returns repair progress.
func (c Client) RepairProgress(ctx context.Context, clusterID, taskID, runID string) (RepairProgress, error) {
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

// BackupProgress returns backup progress.
func (c Client) BackupProgress(ctx context.Context, clusterID, taskID, runID string) (BackupProgress, error) {
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
		return tr, err
	}

	if resp.Payload.Progress == nil {
		resp.Payload.Progress = tr.Progress
	}
	if resp.Payload.Run == nil {
		resp.Payload.Run = tr.Run
	}

	return resp.Payload, nil
}

// ListBackups returns listing of available backups.
func (c Client) ListBackups(ctx context.Context, clusterID string,
	locations []string, allClusters bool, keyspace []string, minDate, maxDate strfmt.DateTime) (BackupListItems, error) {
	p := &operations.GetClusterClusterIDBackupsParams{
		Context:   ctx,
		ClusterID: clusterID,
		Locations: locations,
		Keyspace:  keyspace,
	}
	if !allClusters {
		p.QueryClusterID = &clusterID
	}
	if !time.Time(minDate).IsZero() {
		p.MinDate = &minDate
	}
	if !time.Time(maxDate).IsZero() {
		p.MaxDate = &maxDate
	}

	resp, err := c.operations.GetClusterClusterIDBackups(p)
	if err != nil {
		return BackupListItems{}, err
	}

	return resp.Payload, nil
}

// ListBackupFiles returns a listing of available backup files.
func (c Client) ListBackupFiles(ctx context.Context, clusterID string,
	locations []string, allClusters bool, keyspace []string, snapshotTag string) ([]*models.BackupFilesInfo, error) {
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
func (c Client) DeleteSnapshot(ctx context.Context, clusterID string,
	locations []string, snapshotTag string) error {
	p := &operations.DeleteClusterClusterIDBackupsParams{
		Context:      ctx,
		ClusterID:    clusterID,
		Locations:    locations,
		SnapshotTags: []string{snapshotTag},
	}

	_, err := c.operations.DeleteClusterClusterIDBackups(p) // nolint: errcheck
	return err
}

// Version returns server version.
func (c Client) Version(ctx context.Context) (*models.Version, error) {
	resp, err := c.operations.GetVersion(&operations.GetVersionParams{
		Context: ctx,
	})
	if err != nil {
		return &models.Version{}, err
	}

	return resp.Payload, nil
}

// SetRepairIntensity updates ongoing repair intensity.
func (c Client) SetRepairIntensity(ctx context.Context, clusterID string, intensity float64) error {
	p := &operations.PutClusterClusterIDRepairsIntensityParams{
		Context:   ctx,
		ClusterID: clusterID,
		Intensity: intensity,
	}

	_, err := c.operations.PutClusterClusterIDRepairsIntensity(p) // nolint: errcheck
	return err
}
