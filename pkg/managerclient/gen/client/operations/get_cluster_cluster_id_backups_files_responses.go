// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/scylladb/scylla-operator/pkg/managerclient/gen/models"
)

// GetClusterClusterIDBackupsFilesReader is a Reader for the GetClusterClusterIDBackupsFiles structure.
type GetClusterClusterIDBackupsFilesReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *GetClusterClusterIDBackupsFilesReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewGetClusterClusterIDBackupsFilesOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewGetClusterClusterIDBackupsFilesDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewGetClusterClusterIDBackupsFilesOK creates a GetClusterClusterIDBackupsFilesOK with default headers values
func NewGetClusterClusterIDBackupsFilesOK() *GetClusterClusterIDBackupsFilesOK {
	return &GetClusterClusterIDBackupsFilesOK{}
}

/*
GetClusterClusterIDBackupsFilesOK describes a response with status code 200, with default header values.

Backup list
*/
type GetClusterClusterIDBackupsFilesOK struct {
	Payload []*models.BackupFilesInfo
}

// IsSuccess returns true when this get cluster cluster Id backups files o k response has a 2xx status code
func (o *GetClusterClusterIDBackupsFilesOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this get cluster cluster Id backups files o k response has a 3xx status code
func (o *GetClusterClusterIDBackupsFilesOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this get cluster cluster Id backups files o k response has a 4xx status code
func (o *GetClusterClusterIDBackupsFilesOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this get cluster cluster Id backups files o k response has a 5xx status code
func (o *GetClusterClusterIDBackupsFilesOK) IsServerError() bool {
	return false
}

// IsCode returns true when this get cluster cluster Id backups files o k response a status code equal to that given
func (o *GetClusterClusterIDBackupsFilesOK) IsCode(code int) bool {
	return code == 200
}

func (o *GetClusterClusterIDBackupsFilesOK) Error() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/backups/files][%d] getClusterClusterIdBackupsFilesOK  %+v", 200, o.Payload)
}

func (o *GetClusterClusterIDBackupsFilesOK) String() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/backups/files][%d] getClusterClusterIdBackupsFilesOK  %+v", 200, o.Payload)
}

func (o *GetClusterClusterIDBackupsFilesOK) GetPayload() []*models.BackupFilesInfo {
	return o.Payload
}

func (o *GetClusterClusterIDBackupsFilesOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewGetClusterClusterIDBackupsFilesDefault creates a GetClusterClusterIDBackupsFilesDefault with default headers values
func NewGetClusterClusterIDBackupsFilesDefault(code int) *GetClusterClusterIDBackupsFilesDefault {
	return &GetClusterClusterIDBackupsFilesDefault{
		_statusCode: code,
	}
}

/*
GetClusterClusterIDBackupsFilesDefault describes a response with status code -1, with default header values.

Error
*/
type GetClusterClusterIDBackupsFilesDefault struct {
	_statusCode int

	Payload *models.ErrorResponse
}

// Code gets the status code for the get cluster cluster ID backups files default response
func (o *GetClusterClusterIDBackupsFilesDefault) Code() int {
	return o._statusCode
}

// IsSuccess returns true when this get cluster cluster ID backups files default response has a 2xx status code
func (o *GetClusterClusterIDBackupsFilesDefault) IsSuccess() bool {
	return o._statusCode/100 == 2
}

// IsRedirect returns true when this get cluster cluster ID backups files default response has a 3xx status code
func (o *GetClusterClusterIDBackupsFilesDefault) IsRedirect() bool {
	return o._statusCode/100 == 3
}

// IsClientError returns true when this get cluster cluster ID backups files default response has a 4xx status code
func (o *GetClusterClusterIDBackupsFilesDefault) IsClientError() bool {
	return o._statusCode/100 == 4
}

// IsServerError returns true when this get cluster cluster ID backups files default response has a 5xx status code
func (o *GetClusterClusterIDBackupsFilesDefault) IsServerError() bool {
	return o._statusCode/100 == 5
}

// IsCode returns true when this get cluster cluster ID backups files default response a status code equal to that given
func (o *GetClusterClusterIDBackupsFilesDefault) IsCode(code int) bool {
	return o._statusCode == code
}

func (o *GetClusterClusterIDBackupsFilesDefault) Error() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/backups/files][%d] GetClusterClusterIDBackupsFiles default  %+v", o._statusCode, o.Payload)
}

func (o *GetClusterClusterIDBackupsFilesDefault) String() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/backups/files][%d] GetClusterClusterIDBackupsFiles default  %+v", o._statusCode, o.Payload)
}

func (o *GetClusterClusterIDBackupsFilesDefault) GetPayload() *models.ErrorResponse {
	return o.Payload
}

func (o *GetClusterClusterIDBackupsFilesDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorResponse)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}