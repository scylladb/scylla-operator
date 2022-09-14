// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/scylladb/scylla-operator/pkg/managerclient/gen/models"
)

// NewPutClusterClusterIDTaskTaskTypeTaskIDParams creates a new PutClusterClusterIDTaskTaskTypeTaskIDParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewPutClusterClusterIDTaskTaskTypeTaskIDParams() *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	return &PutClusterClusterIDTaskTaskTypeTaskIDParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewPutClusterClusterIDTaskTaskTypeTaskIDParamsWithTimeout creates a new PutClusterClusterIDTaskTaskTypeTaskIDParams object
// with the ability to set a timeout on a request.
func NewPutClusterClusterIDTaskTaskTypeTaskIDParamsWithTimeout(timeout time.Duration) *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	return &PutClusterClusterIDTaskTaskTypeTaskIDParams{
		timeout: timeout,
	}
}

// NewPutClusterClusterIDTaskTaskTypeTaskIDParamsWithContext creates a new PutClusterClusterIDTaskTaskTypeTaskIDParams object
// with the ability to set a context for a request.
func NewPutClusterClusterIDTaskTaskTypeTaskIDParamsWithContext(ctx context.Context) *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	return &PutClusterClusterIDTaskTaskTypeTaskIDParams{
		Context: ctx,
	}
}

// NewPutClusterClusterIDTaskTaskTypeTaskIDParamsWithHTTPClient creates a new PutClusterClusterIDTaskTaskTypeTaskIDParams object
// with the ability to set a custom HTTPClient for a request.
func NewPutClusterClusterIDTaskTaskTypeTaskIDParamsWithHTTPClient(client *http.Client) *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	return &PutClusterClusterIDTaskTaskTypeTaskIDParams{
		HTTPClient: client,
	}
}

/*
PutClusterClusterIDTaskTaskTypeTaskIDParams contains all the parameters to send to the API endpoint

	for the put cluster cluster ID task task type task ID operation.

	Typically these are written to a http.Request.
*/
type PutClusterClusterIDTaskTaskTypeTaskIDParams struct {

	// ClusterID.
	ClusterID string

	// TaskFields.
	TaskFields *models.TaskUpdate

	// TaskID.
	TaskID string

	// TaskType.
	TaskType string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the put cluster cluster ID task task type task ID params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) WithDefaults() *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the put cluster cluster ID task task type task ID params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) WithTimeout(timeout time.Duration) *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) WithContext(ctx context.Context) *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) WithHTTPClient(client *http.Client) *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithClusterID adds the clusterID to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) WithClusterID(clusterID string) *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetClusterID(clusterID)
	return o
}

// SetClusterID adds the clusterId to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) SetClusterID(clusterID string) {
	o.ClusterID = clusterID
}

// WithTaskFields adds the taskFields to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) WithTaskFields(taskFields *models.TaskUpdate) *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetTaskFields(taskFields)
	return o
}

// SetTaskFields adds the taskFields to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) SetTaskFields(taskFields *models.TaskUpdate) {
	o.TaskFields = taskFields
}

// WithTaskID adds the taskID to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) WithTaskID(taskID string) *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetTaskID(taskID)
	return o
}

// SetTaskID adds the taskId to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) SetTaskID(taskID string) {
	o.TaskID = taskID
}

// WithTaskType adds the taskType to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) WithTaskType(taskType string) *PutClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetTaskType(taskType)
	return o
}

// SetTaskType adds the taskType to the put cluster cluster ID task task type task ID params
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) SetTaskType(taskType string) {
	o.TaskType = taskType
}

// WriteToRequest writes these params to a swagger request
func (o *PutClusterClusterIDTaskTaskTypeTaskIDParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param cluster_id
	if err := r.SetPathParam("cluster_id", o.ClusterID); err != nil {
		return err
	}
	if o.TaskFields != nil {
		if err := r.SetBodyParam(o.TaskFields); err != nil {
			return err
		}
	}

	// path param task_id
	if err := r.SetPathParam("task_id", o.TaskID); err != nil {
		return err
	}

	// path param task_type
	if err := r.SetPathParam("task_type", o.TaskType); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
