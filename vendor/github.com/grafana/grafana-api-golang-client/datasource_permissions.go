package gapi

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type DatasourcePermissionType int

const (
	_ DatasourcePermissionType = iota // 0 is not a valid permission
	DatasourcePermissionQuery
	DatasourcePermissionEdit
)

// DatasourcePermission has information such as a datasource, user, team, role and permission.
type DatasourcePermission struct {
	ID           int64  `json:"id"`
	DatasourceID int64  `json:"datasourceId"`
	UserID       int64  `json:"userId"`
	UserEmail    string `json:"userEmail"`
	TeamID       int64  `json:"teamId"`
	BuiltInRole  string `json:"builtInRole"`

	// Permission levels are
	// 1 = Query
	// 2 = Edit
	Permission     DatasourcePermissionType `json:"permission"`
	PermissionName string                   `json:"permissionName"`
}

type DatasourcePermissionsResponse struct {
	DatasourceID int64                   `json:"datasourceId"`
	Enabled      bool                    `json:"enabled"`
	Permissions  []*DatasourcePermission `json:"permissions"`
}

type DatasourcePermissionAddPayload struct {
	UserID      int64                    `json:"userId"`
	TeamID      int64                    `json:"teamId"`
	BuiltInRole string                   `json:"builtinRole"`
	Permission  DatasourcePermissionType `json:"permission"`
}

// EnableDatasourcePermissions enables the datasource permissions (this is a datasource setting)
func (c *Client) EnableDatasourcePermissions(id int64) error {
	path := fmt.Sprintf("/api/datasources/%d/enable-permissions", id)
	if err := c.request("POST", path, nil, nil, nil); err != nil {
		return fmt.Errorf("error enabling permissions at %s: %w", path, err)
	}
	return nil
}

// DisableDatasourcePermissions disables the datasource permissions (this is a datasource setting)
func (c *Client) DisableDatasourcePermissions(id int64) error {
	path := fmt.Sprintf("/api/datasources/%d/disable-permissions", id)
	if err := c.request("POST", path, nil, nil, nil); err != nil {
		return fmt.Errorf("error disabling permissions at %s: %w", path, err)
	}
	return nil
}

// DatasourcePermissions fetches and returns the permissions for the datasource whose ID it's passed.
func (c *Client) DatasourcePermissions(id int64) (*DatasourcePermissionsResponse, error) {
	path := fmt.Sprintf("/api/datasources/%d/permissions", id)
	var out *DatasourcePermissionsResponse
	err := c.request("GET", path, nil, nil, &out)
	if err != nil {
		return out, fmt.Errorf("error getting permissions at %s: %w", path, err)
	}

	return out, nil
}

// AddDatasourcePermission adds the given permission item
func (c *Client) AddDatasourcePermission(id int64, item *DatasourcePermissionAddPayload) error {
	path := fmt.Sprintf("/api/datasources/%d/permissions", id)
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal err: %w", err)
	}

	if err = c.request("POST", path, nil, bytes.NewBuffer(data), nil); err != nil {
		return fmt.Errorf("error adding permissions at %s: %w", path, err)
	}

	return nil
}

// RemoveDatasourcePermission removes the permission with the given id
func (c *Client) RemoveDatasourcePermission(id, permissionID int64) error {
	path := fmt.Sprintf("/api/datasources/%d/permissions/%d", id, permissionID)
	if err := c.request("DELETE", path, nil, nil, nil); err != nil {
		return fmt.Errorf("error deleting permissions at %s: %w", path, err)
	}

	return nil
}
