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

package events

import (
	frm "github.com/gocql/gocql/internal/frame"
)

// FrameToEvent converts an internal frame to a public Event interface.
// This function has access to internal frame types and can perform
// type-safe conversions.
// Returns nil if the frame is not an event frame.
func FrameToEvent(f interface{}) Event {
	if f == nil {
		return nil
	}

	switch frame := f.(type) {
	case *frm.TopologyChangeEventFrame:
		return &TopologyChangeEvent{
			Change: frame.Change,
			Host:   frame.Host,
			Port:   frame.Port,
		}

	case *frm.StatusChangeEventFrame:
		return &StatusChangeEvent{
			Change: frame.Change,
			Host:   frame.Host,
			Port:   frame.Port,
		}

	case *frm.SchemaChangeKeyspace:
		return &SchemaChangeKeyspaceEvent{
			Change:   frame.Change,
			Keyspace: frame.Keyspace,
		}

	case *frm.SchemaChangeTable:
		return &SchemaChangeTableEvent{
			Change:   frame.Change,
			Keyspace: frame.Keyspace,
			Table:    frame.Object,
		}

	case *frm.SchemaChangeType:
		return &SchemaChangeTypeEvent{
			Change:   frame.Change,
			Keyspace: frame.Keyspace,
			TypeName: frame.Object,
		}

	case *frm.SchemaChangeFunction:
		return &SchemaChangeFunctionEvent{
			Change:    frame.Change,
			Keyspace:  frame.Keyspace,
			Function:  frame.Name,
			Arguments: frame.Args,
		}

	case *frm.SchemaChangeAggregate:
		return &SchemaChangeAggregateEvent{
			Change:    frame.Change,
			Keyspace:  frame.Keyspace,
			Aggregate: frame.Name,
			Arguments: frame.Args,
		}
	case *frm.ClientRoutesChanged:
		return &ClientRoutesChangedEvent{
			ChangeType:    frame.ChangeType,
			ConnectionIDs: frame.ConnectionIDs,
			HostIDs:       frame.HostIDs,
		}
	default:
		return nil
	}
}
