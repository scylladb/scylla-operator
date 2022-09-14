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

// PutClusterClusterIDReader is a Reader for the PutClusterClusterID structure.
type PutClusterClusterIDReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *PutClusterClusterIDReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewPutClusterClusterIDOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewPutClusterClusterIDDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewPutClusterClusterIDOK creates a PutClusterClusterIDOK with default headers values
func NewPutClusterClusterIDOK() *PutClusterClusterIDOK {
	return &PutClusterClusterIDOK{}
}

/*
PutClusterClusterIDOK describes a response with status code 200, with default header values.

Updated cluster info
*/
type PutClusterClusterIDOK struct {
	Payload *models.Cluster
}

// IsSuccess returns true when this put cluster cluster Id o k response has a 2xx status code
func (o *PutClusterClusterIDOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this put cluster cluster Id o k response has a 3xx status code
func (o *PutClusterClusterIDOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this put cluster cluster Id o k response has a 4xx status code
func (o *PutClusterClusterIDOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this put cluster cluster Id o k response has a 5xx status code
func (o *PutClusterClusterIDOK) IsServerError() bool {
	return false
}

// IsCode returns true when this put cluster cluster Id o k response a status code equal to that given
func (o *PutClusterClusterIDOK) IsCode(code int) bool {
	return code == 200
}

func (o *PutClusterClusterIDOK) Error() string {
	return fmt.Sprintf("[PUT /cluster/{cluster_id}][%d] putClusterClusterIdOK  %+v", 200, o.Payload)
}

func (o *PutClusterClusterIDOK) String() string {
	return fmt.Sprintf("[PUT /cluster/{cluster_id}][%d] putClusterClusterIdOK  %+v", 200, o.Payload)
}

func (o *PutClusterClusterIDOK) GetPayload() *models.Cluster {
	return o.Payload
}

func (o *PutClusterClusterIDOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Cluster)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewPutClusterClusterIDDefault creates a PutClusterClusterIDDefault with default headers values
func NewPutClusterClusterIDDefault(code int) *PutClusterClusterIDDefault {
	return &PutClusterClusterIDDefault{
		_statusCode: code,
	}
}

/*
PutClusterClusterIDDefault describes a response with status code -1, with default header values.

Error
*/
type PutClusterClusterIDDefault struct {
	_statusCode int

	Payload *models.ErrorResponse
}

// Code gets the status code for the put cluster cluster ID default response
func (o *PutClusterClusterIDDefault) Code() int {
	return o._statusCode
}

// IsSuccess returns true when this put cluster cluster ID default response has a 2xx status code
func (o *PutClusterClusterIDDefault) IsSuccess() bool {
	return o._statusCode/100 == 2
}

// IsRedirect returns true when this put cluster cluster ID default response has a 3xx status code
func (o *PutClusterClusterIDDefault) IsRedirect() bool {
	return o._statusCode/100 == 3
}

// IsClientError returns true when this put cluster cluster ID default response has a 4xx status code
func (o *PutClusterClusterIDDefault) IsClientError() bool {
	return o._statusCode/100 == 4
}

// IsServerError returns true when this put cluster cluster ID default response has a 5xx status code
func (o *PutClusterClusterIDDefault) IsServerError() bool {
	return o._statusCode/100 == 5
}

// IsCode returns true when this put cluster cluster ID default response a status code equal to that given
func (o *PutClusterClusterIDDefault) IsCode(code int) bool {
	return o._statusCode == code
}

func (o *PutClusterClusterIDDefault) Error() string {
	return fmt.Sprintf("[PUT /cluster/{cluster_id}][%d] PutClusterClusterID default  %+v", o._statusCode, o.Payload)
}

func (o *PutClusterClusterIDDefault) String() string {
	return fmt.Sprintf("[PUT /cluster/{cluster_id}][%d] PutClusterClusterID default  %+v", o._statusCode, o.Payload)
}

func (o *PutClusterClusterIDDefault) GetPayload() *models.ErrorResponse {
	return o.Payload
}

func (o *PutClusterClusterIDDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorResponse)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
