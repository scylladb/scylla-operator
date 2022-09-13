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

// GetClusterClusterIDTasksRepairTargetReader is a Reader for the GetClusterClusterIDTasksRepairTarget structure.
type GetClusterClusterIDTasksRepairTargetReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *GetClusterClusterIDTasksRepairTargetReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewGetClusterClusterIDTasksRepairTargetOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewGetClusterClusterIDTasksRepairTargetDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewGetClusterClusterIDTasksRepairTargetOK creates a GetClusterClusterIDTasksRepairTargetOK with default headers values
func NewGetClusterClusterIDTasksRepairTargetOK() *GetClusterClusterIDTasksRepairTargetOK {
	return &GetClusterClusterIDTasksRepairTargetOK{}
}

/*
GetClusterClusterIDTasksRepairTargetOK describes a response with status code 200, with default header values.

Repair target
*/
type GetClusterClusterIDTasksRepairTargetOK struct {
	Payload *models.RepairTarget
}

// IsSuccess returns true when this get cluster cluster Id tasks repair target o k response has a 2xx status code
func (o *GetClusterClusterIDTasksRepairTargetOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this get cluster cluster Id tasks repair target o k response has a 3xx status code
func (o *GetClusterClusterIDTasksRepairTargetOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this get cluster cluster Id tasks repair target o k response has a 4xx status code
func (o *GetClusterClusterIDTasksRepairTargetOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this get cluster cluster Id tasks repair target o k response has a 5xx status code
func (o *GetClusterClusterIDTasksRepairTargetOK) IsServerError() bool {
	return false
}

// IsCode returns true when this get cluster cluster Id tasks repair target o k response a status code equal to that given
func (o *GetClusterClusterIDTasksRepairTargetOK) IsCode(code int) bool {
	return code == 200
}

func (o *GetClusterClusterIDTasksRepairTargetOK) Error() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/tasks/repair/target][%d] getClusterClusterIdTasksRepairTargetOK  %+v", 200, o.Payload)
}

func (o *GetClusterClusterIDTasksRepairTargetOK) String() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/tasks/repair/target][%d] getClusterClusterIdTasksRepairTargetOK  %+v", 200, o.Payload)
}

func (o *GetClusterClusterIDTasksRepairTargetOK) GetPayload() *models.RepairTarget {
	return o.Payload
}

func (o *GetClusterClusterIDTasksRepairTargetOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.RepairTarget)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewGetClusterClusterIDTasksRepairTargetDefault creates a GetClusterClusterIDTasksRepairTargetDefault with default headers values
func NewGetClusterClusterIDTasksRepairTargetDefault(code int) *GetClusterClusterIDTasksRepairTargetDefault {
	return &GetClusterClusterIDTasksRepairTargetDefault{
		_statusCode: code,
	}
}

/*
GetClusterClusterIDTasksRepairTargetDefault describes a response with status code -1, with default header values.

Error
*/
type GetClusterClusterIDTasksRepairTargetDefault struct {
	_statusCode int

	Payload *models.ErrorResponse
}

// Code gets the status code for the get cluster cluster ID tasks repair target default response
func (o *GetClusterClusterIDTasksRepairTargetDefault) Code() int {
	return o._statusCode
}

// IsSuccess returns true when this get cluster cluster ID tasks repair target default response has a 2xx status code
func (o *GetClusterClusterIDTasksRepairTargetDefault) IsSuccess() bool {
	return o._statusCode/100 == 2
}

// IsRedirect returns true when this get cluster cluster ID tasks repair target default response has a 3xx status code
func (o *GetClusterClusterIDTasksRepairTargetDefault) IsRedirect() bool {
	return o._statusCode/100 == 3
}

// IsClientError returns true when this get cluster cluster ID tasks repair target default response has a 4xx status code
func (o *GetClusterClusterIDTasksRepairTargetDefault) IsClientError() bool {
	return o._statusCode/100 == 4
}

// IsServerError returns true when this get cluster cluster ID tasks repair target default response has a 5xx status code
func (o *GetClusterClusterIDTasksRepairTargetDefault) IsServerError() bool {
	return o._statusCode/100 == 5
}

// IsCode returns true when this get cluster cluster ID tasks repair target default response a status code equal to that given
func (o *GetClusterClusterIDTasksRepairTargetDefault) IsCode(code int) bool {
	return o._statusCode == code
}

func (o *GetClusterClusterIDTasksRepairTargetDefault) Error() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/tasks/repair/target][%d] GetClusterClusterIDTasksRepairTarget default  %+v", o._statusCode, o.Payload)
}

func (o *GetClusterClusterIDTasksRepairTargetDefault) String() string {
	return fmt.Sprintf("[GET /cluster/{cluster_id}/tasks/repair/target][%d] GetClusterClusterIDTasksRepairTarget default  %+v", o._statusCode, o.Payload)
}

func (o *GetClusterClusterIDTasksRepairTargetDefault) GetPayload() *models.ErrorResponse {
	return o.Payload
}

func (o *GetClusterClusterIDTasksRepairTargetDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorResponse)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}