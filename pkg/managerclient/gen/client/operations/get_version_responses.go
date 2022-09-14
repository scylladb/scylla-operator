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

// GetVersionReader is a Reader for the GetVersion structure.
type GetVersionReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *GetVersionReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewGetVersionOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewGetVersionDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewGetVersionOK creates a GetVersionOK with default headers values
func NewGetVersionOK() *GetVersionOK {
	return &GetVersionOK{}
}

/*
GetVersionOK describes a response with status code 200, with default header values.

Server version
*/
type GetVersionOK struct {
	Payload *models.Version
}

// IsSuccess returns true when this get version o k response has a 2xx status code
func (o *GetVersionOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this get version o k response has a 3xx status code
func (o *GetVersionOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this get version o k response has a 4xx status code
func (o *GetVersionOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this get version o k response has a 5xx status code
func (o *GetVersionOK) IsServerError() bool {
	return false
}

// IsCode returns true when this get version o k response a status code equal to that given
func (o *GetVersionOK) IsCode(code int) bool {
	return code == 200
}

func (o *GetVersionOK) Error() string {
	return fmt.Sprintf("[GET /version][%d] getVersionOK  %+v", 200, o.Payload)
}

func (o *GetVersionOK) String() string {
	return fmt.Sprintf("[GET /version][%d] getVersionOK  %+v", 200, o.Payload)
}

func (o *GetVersionOK) GetPayload() *models.Version {
	return o.Payload
}

func (o *GetVersionOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Version)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewGetVersionDefault creates a GetVersionDefault with default headers values
func NewGetVersionDefault(code int) *GetVersionDefault {
	return &GetVersionDefault{
		_statusCode: code,
	}
}

/*
GetVersionDefault describes a response with status code -1, with default header values.

Error
*/
type GetVersionDefault struct {
	_statusCode int

	Payload *models.ErrorResponse
}

// Code gets the status code for the get version default response
func (o *GetVersionDefault) Code() int {
	return o._statusCode
}

// IsSuccess returns true when this get version default response has a 2xx status code
func (o *GetVersionDefault) IsSuccess() bool {
	return o._statusCode/100 == 2
}

// IsRedirect returns true when this get version default response has a 3xx status code
func (o *GetVersionDefault) IsRedirect() bool {
	return o._statusCode/100 == 3
}

// IsClientError returns true when this get version default response has a 4xx status code
func (o *GetVersionDefault) IsClientError() bool {
	return o._statusCode/100 == 4
}

// IsServerError returns true when this get version default response has a 5xx status code
func (o *GetVersionDefault) IsServerError() bool {
	return o._statusCode/100 == 5
}

// IsCode returns true when this get version default response a status code equal to that given
func (o *GetVersionDefault) IsCode(code int) bool {
	return o._statusCode == code
}

func (o *GetVersionDefault) Error() string {
	return fmt.Sprintf("[GET /version][%d] GetVersion default  %+v", o._statusCode, o.Payload)
}

func (o *GetVersionDefault) String() string {
	return fmt.Sprintf("[GET /version][%d] GetVersion default  %+v", o._statusCode, o.Payload)
}

func (o *GetVersionDefault) GetPayload() *models.ErrorResponse {
	return o.Payload
}

func (o *GetVersionDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorResponse)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
