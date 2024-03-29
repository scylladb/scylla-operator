// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// RestoreViewProgress restore view progress
//
// swagger:model RestoreViewProgress
type RestoreViewProgress struct {

	// base table
	BaseTable string `json:"base_table,omitempty"`

	// create stmt
	CreateStmt string `json:"create_stmt,omitempty"`

	// keyspace
	Keyspace string `json:"keyspace,omitempty"`

	// status
	Status string `json:"status,omitempty"`

	// type
	Type string `json:"type,omitempty"`

	// view
	View string `json:"view,omitempty"`
}

// Validate validates this restore view progress
func (m *RestoreViewProgress) Validate(formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *RestoreViewProgress) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *RestoreViewProgress) UnmarshalBinary(b []byte) error {
	var res RestoreViewProgress
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
