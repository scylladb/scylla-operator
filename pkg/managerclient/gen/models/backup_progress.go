// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"strconv"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// BackupProgress backup progress
//
// swagger:model BackupProgress
type BackupProgress struct {

	// completed at
	// Format: date-time
	CompletedAt *strfmt.DateTime `json:"completed_at,omitempty"`

	// dcs
	Dcs []string `json:"dcs"`

	// failed
	Failed int64 `json:"failed,omitempty"`

	// hosts
	Hosts []*HostProgress `json:"hosts"`

	// size
	Size int64 `json:"size,omitempty"`

	// skipped
	Skipped int64 `json:"skipped,omitempty"`

	// snapshot tag
	SnapshotTag string `json:"snapshot_tag,omitempty"`

	// stage
	Stage string `json:"stage,omitempty"`

	// started at
	// Format: date-time
	StartedAt *strfmt.DateTime `json:"started_at,omitempty"`

	// uploaded
	Uploaded int64 `json:"uploaded,omitempty"`
}

// Validate validates this backup progress
func (m *BackupProgress) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateCompletedAt(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateHosts(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateStartedAt(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *BackupProgress) validateCompletedAt(formats strfmt.Registry) error {
	if swag.IsZero(m.CompletedAt) { // not required
		return nil
	}

	if err := validate.FormatOf("completed_at", "body", "date-time", m.CompletedAt.String(), formats); err != nil {
		return err
	}

	return nil
}

func (m *BackupProgress) validateHosts(formats strfmt.Registry) error {
	if swag.IsZero(m.Hosts) { // not required
		return nil
	}

	for i := 0; i < len(m.Hosts); i++ {
		if swag.IsZero(m.Hosts[i]) { // not required
			continue
		}

		if m.Hosts[i] != nil {
			if err := m.Hosts[i].Validate(formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("hosts" + "." + strconv.Itoa(i))
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("hosts" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

func (m *BackupProgress) validateStartedAt(formats strfmt.Registry) error {
	if swag.IsZero(m.StartedAt) { // not required
		return nil
	}

	if err := validate.FormatOf("started_at", "body", "date-time", m.StartedAt.String(), formats); err != nil {
		return err
	}

	return nil
}

// ContextValidate validate this backup progress based on the context it is used
func (m *BackupProgress) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateHosts(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *BackupProgress) contextValidateHosts(ctx context.Context, formats strfmt.Registry) error {

	for i := 0; i < len(m.Hosts); i++ {

		if m.Hosts[i] != nil {
			if err := m.Hosts[i].ContextValidate(ctx, formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("hosts" + "." + strconv.Itoa(i))
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("hosts" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

// MarshalBinary interface implementation
func (m *BackupProgress) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *BackupProgress) UnmarshalBinary(b []byte) error {
	var res BackupProgress
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
