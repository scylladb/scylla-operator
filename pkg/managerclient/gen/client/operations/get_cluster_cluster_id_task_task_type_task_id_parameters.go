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
)

// NewGetClusterClusterIDTaskTaskTypeTaskIDParams creates a new GetClusterClusterIDTaskTaskTypeTaskIDParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewGetClusterClusterIDTaskTaskTypeTaskIDParams() *GetClusterClusterIDTaskTaskTypeTaskIDParams {
	return &GetClusterClusterIDTaskTaskTypeTaskIDParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewGetClusterClusterIDTaskTaskTypeTaskIDParamsWithTimeout creates a new GetClusterClusterIDTaskTaskTypeTaskIDParams object
// with the ability to set a timeout on a request.
func NewGetClusterClusterIDTaskTaskTypeTaskIDParamsWithTimeout(timeout time.Duration) *GetClusterClusterIDTaskTaskTypeTaskIDParams {
	return &GetClusterClusterIDTaskTaskTypeTaskIDParams{
		timeout: timeout,
	}
}

// NewGetClusterClusterIDTaskTaskTypeTaskIDParamsWithContext creates a new GetClusterClusterIDTaskTaskTypeTaskIDParams object
// with the ability to set a context for a request.
func NewGetClusterClusterIDTaskTaskTypeTaskIDParamsWithContext(ctx context.Context) *GetClusterClusterIDTaskTaskTypeTaskIDParams {
	return &GetClusterClusterIDTaskTaskTypeTaskIDParams{
		Context: ctx,
	}
}

// NewGetClusterClusterIDTaskTaskTypeTaskIDParamsWithHTTPClient creates a new GetClusterClusterIDTaskTaskTypeTaskIDParams object
// with the ability to set a custom HTTPClient for a request.
func NewGetClusterClusterIDTaskTaskTypeTaskIDParamsWithHTTPClient(client *http.Client) *GetClusterClusterIDTaskTaskTypeTaskIDParams {
	return &GetClusterClusterIDTaskTaskTypeTaskIDParams{
		HTTPClient: client,
	}
}

/*
GetClusterClusterIDTaskTaskTypeTaskIDParams contains all the parameters to send to the API endpoint

	for the get cluster cluster ID task task type task ID operation.

	Typically these are written to a http.Request.
*/
type GetClusterClusterIDTaskTaskTypeTaskIDParams struct {

	// ClusterID.
	ClusterID string

	// TaskID.
	TaskID string

	// TaskType.
	TaskType string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the get cluster cluster ID task task type task ID params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) WithDefaults() *GetClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the get cluster cluster ID task task type task ID params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) WithTimeout(timeout time.Duration) *GetClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) WithContext(ctx context.Context) *GetClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) WithHTTPClient(client *http.Client) *GetClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithClusterID adds the clusterID to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) WithClusterID(clusterID string) *GetClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetClusterID(clusterID)
	return o
}

// SetClusterID adds the clusterId to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) SetClusterID(clusterID string) {
	o.ClusterID = clusterID
}

// WithTaskID adds the taskID to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) WithTaskID(taskID string) *GetClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetTaskID(taskID)
	return o
}

// SetTaskID adds the taskId to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) SetTaskID(taskID string) {
	o.TaskID = taskID
}

// WithTaskType adds the taskType to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) WithTaskType(taskType string) *GetClusterClusterIDTaskTaskTypeTaskIDParams {
	o.SetTaskType(taskType)
	return o
}

// SetTaskType adds the taskType to the get cluster cluster ID task task type task ID params
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) SetTaskType(taskType string) {
	o.TaskType = taskType
}

// WriteToRequest writes these params to a swagger request
func (o *GetClusterClusterIDTaskTaskTypeTaskIDParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param cluster_id
	if err := r.SetPathParam("cluster_id", o.ClusterID); err != nil {
		return err
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
