/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package events provides public event types for Cassandra/ScyllaDB server events.
// These events are sent by the server to notify clients of topology changes, status changes,
// and schema changes.
package events

import (
	"fmt"
	"net"
)

// EventType represents the type of event
type EventType int

const (
	// ClusterEventTypeTopologyChange represents a topology change event (NEW_NODE, REMOVED_NODE, MOVED_NODE)
	ClusterEventTypeTopologyChange EventType = iota
	// ClusterEventTypeStatusChange represents a status change event (UP, DOWN)
	ClusterEventTypeStatusChange
	// ClusterEventTypeSchemaChangeKeyspace represents a keyspace schema change
	ClusterEventTypeSchemaChangeKeyspace
	// ClusterEventTypeSchemaChangeTable represents a table schema change
	ClusterEventTypeSchemaChangeTable
	// ClusterEventTypeSchemaChangeType represents a UDT schema change
	ClusterEventTypeSchemaChangeType
	// ClusterEventTypeSchemaChangeFunction represents a function schema change
	ClusterEventTypeSchemaChangeFunction
	// ClusterEventTypeSchemaChangeAggregate represents an aggregate schema change
	ClusterEventTypeSchemaChangeAggregate
	// ClusterEventTypeClientRoutesChanged represents an event of update of `system.client_routes` table
	ClusterEventTypeClientRoutesChanged
	// SessionEventTypeControlConnectionRecreated is fired when the session loses it's control connection to the cluster and has just been re-established it.
	SessionEventTypeControlConnectionRecreated
)

func (t EventType) IsClusterEvent() bool {
	switch t {
	case ClusterEventTypeTopologyChange:
		return true
	case ClusterEventTypeStatusChange:
		return true
	case ClusterEventTypeSchemaChangeKeyspace:
		return true
	case ClusterEventTypeSchemaChangeTable:
		return true
	case ClusterEventTypeSchemaChangeType:
		return true
	case ClusterEventTypeSchemaChangeFunction:
		return true
	case ClusterEventTypeSchemaChangeAggregate:
		return true
	case ClusterEventTypeClientRoutesChanged:
		return true
	default:
		return false
	}
}

func (t EventType) String() string {
	switch t {
	case ClusterEventTypeTopologyChange:
		return "CLUSTER<TOPOLOGY_CHANGE>"
	case ClusterEventTypeStatusChange:
		return "CLUSTER<STATUS_CHANGE>"
	case ClusterEventTypeSchemaChangeKeyspace:
		return "CLUSTER<SCHEMA_CHANGE_KEYSPACE>"
	case ClusterEventTypeSchemaChangeTable:
		return "CLUSTER<SCHEMA_CHANGE_TABLE>"
	case ClusterEventTypeSchemaChangeType:
		return "CLUSTER<SCHEMA_CHANGE_TYPE>"
	case ClusterEventTypeSchemaChangeFunction:
		return "CLUSTER<SCHEMA_CHANGE_FUNCTION>"
	case ClusterEventTypeSchemaChangeAggregate:
		return "CLUSTER<SCHEMA_CHANGE_AGGREGATE>"
	case ClusterEventTypeClientRoutesChanged:
		return "CLUSTER<CLIENT_ROUTES_CHANGE>"
	case SessionEventTypeControlConnectionRecreated:
		return "SESSION<CONTROL_CONNECTION_RECREATED>"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", t)
	}
}

// Event is the common interface for all event types
type Event interface {
	// Type returns the type of event
	Type() EventType
	// String returns a string representation of the event
	String() string
}

// TopologyChangeEvent represents a topology change in the cluster
type TopologyChangeEvent struct {
	// Change is the type of topology change (NEW_NODE, REMOVED_NODE, MOVED_NODE)
	Change string
	// Host is the IP address of the node
	Host net.IP
	// Port is the port number
	Port int
}

// Type returns ClusterEventTypeTopologyChange
func (e *TopologyChangeEvent) Type() EventType {
	return ClusterEventTypeTopologyChange
}

// String returns a string representation of the event
func (e *TopologyChangeEvent) String() string {
	return fmt.Sprintf("TopologyChange{change=%s, host=%s, port=%d}", e.Change, e.Host, e.Port)
}

// StatusChangeEvent represents a status change of a node
type StatusChangeEvent struct {
	// Change is the type of status change (UP, DOWN)
	Change string
	// Host is the IP address of the node
	Host net.IP
	// Port is the port number
	Port int
}

// Type returns ClusterEventTypeStatusChange
func (e *StatusChangeEvent) Type() EventType {
	return ClusterEventTypeStatusChange
}

// String returns a string representation of the event
func (e *StatusChangeEvent) String() string {
	return fmt.Sprintf("StatusChange{change=%s, host=%s, port=%d}", e.Change, e.Host, e.Port)
}

// SchemaChangeKeyspaceEvent represents a keyspace schema change
type SchemaChangeKeyspaceEvent struct {
	// Change is the type of change (CREATED, UPDATED, DROPPED)
	Change string
	// Keyspace is the name of the keyspace
	Keyspace string
}

// Type returns ClusterEventTypeSchemaChangeKeyspace
func (e *SchemaChangeKeyspaceEvent) Type() EventType {
	return ClusterEventTypeSchemaChangeKeyspace
}

// String returns a string representation of the event
func (e *SchemaChangeKeyspaceEvent) String() string {
	return fmt.Sprintf("SchemaChangeKeyspace{change=%s, keyspace=%s}", e.Change, e.Keyspace)
}

// SchemaChangeTableEvent represents a table schema change
type SchemaChangeTableEvent struct {
	// Change is the type of change (CREATED, UPDATED, DROPPED)
	Change string
	// Keyspace is the name of the keyspace
	Keyspace string
	// Table is the name of the table
	Table string
}

// Type returns ClusterEventTypeSchemaChangeTable
func (e *SchemaChangeTableEvent) Type() EventType {
	return ClusterEventTypeSchemaChangeTable
}

// String returns a string representation of the event
func (e *SchemaChangeTableEvent) String() string {
	return fmt.Sprintf("SchemaChangeTable{change=%s, keyspace=%s, table=%s}", e.Change, e.Keyspace, e.Table)
}

// SchemaChangeTypeEvent represents a UDT (User Defined Type) schema change
type SchemaChangeTypeEvent struct {
	// Change is the type of change (CREATED, UPDATED, DROPPED)
	Change string
	// Keyspace is the name of the keyspace
	Keyspace string
	// TypeName is the name of the UDT
	TypeName string
}

// Type returns ClusterEventTypeSchemaChangeType
func (e *SchemaChangeTypeEvent) Type() EventType {
	return ClusterEventTypeSchemaChangeType
}

// String returns a string representation of the event
func (e *SchemaChangeTypeEvent) String() string {
	return fmt.Sprintf("SchemaChangeType{change=%s, keyspace=%s, type=%s}", e.Change, e.Keyspace, e.TypeName)
}

// SchemaChangeFunctionEvent represents a function schema change
type SchemaChangeFunctionEvent struct {
	// Change is the type of change (CREATED, UPDATED, DROPPED)
	Change string
	// Keyspace is the name of the keyspace
	Keyspace string
	// Function is the name of the function
	Function string
	// Arguments is the list of argument types
	Arguments []string
}

// Type returns ClusterEventTypeSchemaChangeFunction
func (e *SchemaChangeFunctionEvent) Type() EventType {
	return ClusterEventTypeSchemaChangeFunction
}

// String returns a string representation of the event
func (e *SchemaChangeFunctionEvent) String() string {
	return fmt.Sprintf("SchemaChangeFunction{change=%s, keyspace=%s, function=%s, args=%v}",
		e.Change, e.Keyspace, e.Function, e.Arguments)
}

// SchemaChangeAggregateEvent represents an aggregate schema change
type SchemaChangeAggregateEvent struct {
	// Change is the type of change (CREATED, UPDATED, DROPPED)
	Change string
	// Keyspace is the name of the keyspace
	Keyspace string
	// Aggregate is the name of the aggregate
	Aggregate string
	// Arguments is the list of argument types
	Arguments []string
}

// Type returns ClusterEventTypeSchemaChangeAggregate
func (e *SchemaChangeAggregateEvent) Type() EventType {
	return ClusterEventTypeSchemaChangeAggregate
}

// String returns a string representation of the event
func (e *SchemaChangeAggregateEvent) String() string {
	return fmt.Sprintf("SchemaChangeAggregate{change=%s, keyspace=%s, aggregate=%s, args=%v}",
		e.Change, e.Keyspace, e.Aggregate, e.Arguments)
}

// ClientRoutesChangedEvent represents an aggregate schema change
type ClientRoutesChangedEvent struct {
	// Change is the type of change (UPDATED)
	ChangeType string
	// List of connection ids involved into update
	ConnectionIDs []string
	// List of host ids involved into update
	HostIDs []string
}

// Type returns ClusterEventTypeClientRoutesChanged
func (e *ClientRoutesChangedEvent) Type() EventType {
	return ClusterEventTypeClientRoutesChanged
}

// String returns a string representation of the event
func (e *ClientRoutesChangedEvent) String() string {
	return fmt.Sprintf("ConnectionMetadataChanged{changeType=%s, ConnectionIDs=%s, HostIDs=%s}",
		e.ChangeType, e.ConnectionIDs, e.HostIDs)
}

type HostInfo struct {
	HostID string
	Host   net.IP
	Port   int
}

// String returns a string representation of the event
func (h *HostInfo) String() string {
	return fmt.Sprintf("HostInfo{Host=%s, Port=%d, HostID=%s}", h.Host, h.Port, h.HostID)
}

// ControlConnectionRecreatedEvent represents a control connection reconnection event.
type ControlConnectionRecreatedEvent struct {
	OldHost HostInfo
	NewHost HostInfo
}

// Type returns SessionEventTypeControlConnectionRecreated
func (e *ControlConnectionRecreatedEvent) Type() EventType {
	return SessionEventTypeControlConnectionRecreated
}

// String returns a string representation of the event
func (e *ControlConnectionRecreatedEvent) String() string {
	return fmt.Sprintf("ControlConnectionRecreatedEvent{OldHost=%s, NewHost=%s}", e.OldHost.String(), e.NewHost.String())
}
