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

// GetClusterClusterIDTasksBackupTargetReader is a Reader for the GetClusterClusterIDTasksBackupTarget structure.
type GetClusterClusterIDTasksBackupTargetReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *GetClusterClusterIDTasksBackupTargetReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewGetClusterClusterIDTasksBackupTargetOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewGetClusterClusterIDTasksBackupTargetDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewGetClusterClusterIDTasksBackupTargetOK creates a GetClusterClusterIDTasksBackupTargetOK with default headers values
func NewGetClusterClusterIDTasksBackupTargetOK() *GetClusterClusterIDTasksBackupTargetOK {
	return &GetClusterClusterIDTasksBackupTargetOK{}
}

/*
GetClusterClusterIDTasksBackupTargetOK describes a response with status code 200, with default header values.

Backup target
*/
type GetClusterClusterIDTasksBackupTargetOK struct {
	Payload *models.BackupTarget
}

// IsSuccess returns true when this get cluster cluster Id tasks backup target o k response has a 2xx status code
func (o *GetClusterClusterIDTasksBackupTargetOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this get cluster cluster Id tasks backup target o k response has a 3xx status code
func (o *GetClusterClusterIDTasksBackupTargetOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this get cluster cluster Id tasks backup target o k response has a 4xx status code
func (o *GetClusterClusterIDTasksBackupTargetOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this get cluster cluster Id tasks backup target o k response has a 5xx status code
func (o *GetClusterClusterIDTasksBackupTargetOK) IsServerError() bool {
	return false
}

// IsCode returns true when this get cluster cluster Id tasks backup target o k response a status code equal to that given
func (o *GetClusterClusterIDTasksBackupTargetOK) IsCode(code int) bool {
	return code == 200
}

func (o *GetClusterClusterIDTasksBackupTargetOK) Error() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/tasks/backup/target][%d] getClusterClusterIdTasksBackupTargetOK  %+v", 200, o.Payload)
}

func (o *GetClusterClusterIDTasksBackupTargetOK) String() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/tasks/backup/target][%d] getClusterClusterIdTasksBackupTargetOK  %+v", 200, o.Payload)
}

func (o *GetClusterClusterIDTasksBackupTargetOK) GetPayload() *models.BackupTarget {
	return o.Payload
}

func (o *GetClusterClusterIDTasksBackupTargetOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.BackupTarget)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewGetClusterClusterIDTasksBackupTargetDefault creates a GetClusterClusterIDTasksBackupTargetDefault with default headers values
func NewGetClusterClusterIDTasksBackupTargetDefault(code int) *GetClusterClusterIDTasksBackupTargetDefault {
	return &GetClusterClusterIDTasksBackupTargetDefault{
		_statusCode: code,
	}
}

/*
GetClusterClusterIDTasksBackupTargetDefault describes a response with status code -1, with default header values.

Error
*/
type GetClusterClusterIDTasksBackupTargetDefault struct {
	_statusCode int

	Payload *models.ErrorResponse
}

// Code gets the status code for the get cluster cluster ID tasks backup target default response
func (o *GetClusterClusterIDTasksBackupTargetDefault) Code() int {
	return o._statusCode
}

// IsSuccess returns true when this get cluster cluster ID tasks backup target default response has a 2xx status code
func (o *GetClusterClusterIDTasksBackupTargetDefault) IsSuccess() bool {
	return o._statusCode/100 == 2
}

// IsRedirect returns true when this get cluster cluster ID tasks backup target default response has a 3xx status code
func (o *GetClusterClusterIDTasksBackupTargetDefault) IsRedirect() bool {
	return o._statusCode/100 == 3
}

// IsClientError returns true when this get cluster cluster ID tasks backup target default response has a 4xx status code
func (o *GetClusterClusterIDTasksBackupTargetDefault) IsClientError() bool {
	return o._statusCode/100 == 4
}

// IsServerError returns true when this get cluster cluster ID tasks backup target default response has a 5xx status code
func (o *GetClusterClusterIDTasksBackupTargetDefault) IsServerError() bool {
	return o._statusCode/100 == 5
}

// IsCode returns true when this get cluster cluster ID tasks backup target default response a status code equal to that given
func (o *GetClusterClusterIDTasksBackupTargetDefault) IsCode(code int) bool {
	return o._statusCode == code
}

func (o *GetClusterClusterIDTasksBackupTargetDefault) Error() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/tasks/backup/target][%d] GetClusterClusterIDTasksBackupTarget default  %+v", o._statusCode, o.Payload)
}

func (o *GetClusterClusterIDTasksBackupTargetDefault) String() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/tasks/backup/target][%d] GetClusterClusterIDTasksBackupTarget default  %+v", o._statusCode, o.Payload)
}

func (o *GetClusterClusterIDTasksBackupTargetDefault) GetPayload() *models.ErrorResponse {
	return o.Payload
}

func (o *GetClusterClusterIDTasksBackupTargetDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorResponse)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
