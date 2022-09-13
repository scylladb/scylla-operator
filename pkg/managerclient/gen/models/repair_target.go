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
)

// RepairTarget repair target
//
// swagger:model RepairTarget
type RepairTarget struct {

	// cluster id
	ClusterID string `json:"cluster_id,omitempty"`

	// dc
	Dc []string `json:"dc"`

	// host
	Host string `json:"host,omitempty"`

	// ignore hosts
	IgnoreHosts []string `json:"ignore_hosts"`

	// token ranges
	TokenRanges string `json:"token_ranges,omitempty"`

	// units
	Units []*RepairUnit `json:"units"`

	// with hosts
	WithHosts []string `json:"with_hosts"`
}

// Validate validates this repair target
func (m *RepairTarget) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateUnits(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *RepairTarget) validateUnits(formats strfmt.Registry) error {
	if swag.IsZero(m.Units) { // not required
		return nil
	}

	for i := 0; i < len(m.Units); i++ {
		if swag.IsZero(m.Units[i]) { // not required
			continue
		}

		if m.Units[i] != nil {
			if err := m.Units[i].Validate(formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("units" + "." + strconv.Itoa(i))
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("units" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

// ContextValidate validate this repair target based on the context it is used
func (m *RepairTarget) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateUnits(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *RepairTarget) contextValidateUnits(ctx context.Context, formats strfmt.Registry) error {

	for i := 0; i < len(m.Units); i++ {

		if m.Units[i] != nil {
			if err := m.Units[i].ContextValidate(ctx, formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("units" + "." + strconv.Itoa(i))
				} else if ce, ok := err.(*errors.CompositeError); ok {
					return ce.ValidateName("units" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

// MarshalBinary interface implementation
func (m *RepairTarget) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *RepairTarget) UnmarshalBinary(b []byte) error {
	var res RepairTarget
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}