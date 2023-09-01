package gapi

import (
	"encoding/json"
	"fmt"
)

// ServiceAccountPermission represents a service account permission for a user or a team.
type ServiceAccountPermission struct {
	ID         int64  `json:"id"`
	TeamID     int64  `json:"teamId,omitempty"`
	UserID     int64  `json:"userId,omitempty"`
	IsManaged  bool   `json:"isManaged"`
	Permission string `json:"permission"`
}

// ServiceAccountPermissionItems represents Grafana service account permission items used for permission updates.
type ServiceAccountPermissionItems struct {
	Permissions []*ServiceAccountPermissionItem `json:"permissions"`
}

// ServiceAccountPermissionItem represents a Grafana service account permission item.
type ServiceAccountPermissionItem struct {
	TeamID     int64  `json:"teamId,omitempty"`
	UserID     int64  `json:"userId,omitempty"`
	Permission string `json:"permission"`
}

// GetServiceAccountPermissions fetches and returns the permissions for the service account whose ID it's passed in.
func (c *Client) GetServiceAccountPermissions(id int64) ([]*ServiceAccountPermission, error) {
	permissions := make([]*ServiceAccountPermission, 0)
	err := c.request("GET", fmt.Sprintf("/api/access-control/serviceaccounts/%d", id), nil, nil, &permissions)
	if err != nil {
		return permissions, err
	}

	return permissions, nil
}

// UpdateServiceAccountPermissions updates service account permissions for teams and users included in the request.
func (c *Client) UpdateServiceAccountPermissions(id int64, items *ServiceAccountPermissionItems) error {
	path := fmt.Sprintf("/api/access-control/serviceaccounts/%d", id)
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}

	return c.request("POST", path, nil, data, nil)
}
