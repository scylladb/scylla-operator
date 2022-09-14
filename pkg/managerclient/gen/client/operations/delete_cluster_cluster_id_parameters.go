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
	"github.com/go-openapi/swag"
)

// NewDeleteClusterClusterIDParams creates a new DeleteClusterClusterIDParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewDeleteClusterClusterIDParams() *DeleteClusterClusterIDParams {
	return &DeleteClusterClusterIDParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewDeleteClusterClusterIDParamsWithTimeout creates a new DeleteClusterClusterIDParams object
// with the ability to set a timeout on a request.
func NewDeleteClusterClusterIDParamsWithTimeout(timeout time.Duration) *DeleteClusterClusterIDParams {
	return &DeleteClusterClusterIDParams{
		timeout: timeout,
	}
}

// NewDeleteClusterClusterIDParamsWithContext creates a new DeleteClusterClusterIDParams object
// with the ability to set a context for a request.
func NewDeleteClusterClusterIDParamsWithContext(ctx context.Context) *DeleteClusterClusterIDParams {
	return &DeleteClusterClusterIDParams{
		Context: ctx,
	}
}

// NewDeleteClusterClusterIDParamsWithHTTPClient creates a new DeleteClusterClusterIDParams object
// with the ability to set a custom HTTPClient for a request.
func NewDeleteClusterClusterIDParamsWithHTTPClient(client *http.Client) *DeleteClusterClusterIDParams {
	return &DeleteClusterClusterIDParams{
		HTTPClient: client,
	}
}

/*
DeleteClusterClusterIDParams contains all the parameters to send to the API endpoint

	for the delete cluster cluster ID operation.

	Typically these are written to a http.Request.
*/
type DeleteClusterClusterIDParams struct {

	// ClusterID.
	ClusterID string

	// CqlCreds.
	CqlCreds *bool

	// SslUserCert.
	SslUserCert *bool

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the delete cluster cluster ID params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DeleteClusterClusterIDParams) WithDefaults() *DeleteClusterClusterIDParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the delete cluster cluster ID params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DeleteClusterClusterIDParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) WithTimeout(timeout time.Duration) *DeleteClusterClusterIDParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) WithContext(ctx context.Context) *DeleteClusterClusterIDParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) WithHTTPClient(client *http.Client) *DeleteClusterClusterIDParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithClusterID adds the clusterID to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) WithClusterID(clusterID string) *DeleteClusterClusterIDParams {
	o.SetClusterID(clusterID)
	return o
}

// SetClusterID adds the clusterId to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) SetClusterID(clusterID string) {
	o.ClusterID = clusterID
}

// WithCqlCreds adds the cqlCreds to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) WithCqlCreds(cqlCreds *bool) *DeleteClusterClusterIDParams {
	o.SetCqlCreds(cqlCreds)
	return o
}

// SetCqlCreds adds the cqlCreds to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) SetCqlCreds(cqlCreds *bool) {
	o.CqlCreds = cqlCreds
}

// WithSslUserCert adds the sslUserCert to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) WithSslUserCert(sslUserCert *bool) *DeleteClusterClusterIDParams {
	o.SetSslUserCert(sslUserCert)
	return o
}

// SetSslUserCert adds the sslUserCert to the delete cluster cluster ID params
func (o *DeleteClusterClusterIDParams) SetSslUserCert(sslUserCert *bool) {
	o.SslUserCert = sslUserCert
}

// WriteToRequest writes these params to a swagger request
func (o *DeleteClusterClusterIDParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param cluster_id
	if err := r.SetPathParam("cluster_id", o.ClusterID); err != nil {
		return err
	}

	if o.CqlCreds != nil {

		// query param cql_creds
		var qrCqlCreds bool

		if o.CqlCreds != nil {
			qrCqlCreds = *o.CqlCreds
		}
		qCqlCreds := swag.FormatBool(qrCqlCreds)
		if qCqlCreds != "" {

			if err := r.SetQueryParam("cql_creds", qCqlCreds); err != nil {
				return err
			}
		}
	}

	if o.SslUserCert != nil {

		// query param ssl_user_cert
		var qrSslUserCert bool

		if o.SslUserCert != nil {
			qrSslUserCert = *o.SslUserCert
		}
		qSslUserCert := swag.FormatBool(qrSslUserCert)
		if qSslUserCert != "" {

			if err := r.SetQueryParam("ssl_user_cert", qSslUserCert); err != nil {
				return err
			}
		}
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
