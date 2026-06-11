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
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package tablets

import (
	"fmt"

	"github.com/gocql/gocql/internal/cqlproto"
)

// ParseHint is a zero-reflection binary parser for the
// tablets-routing-v1 custom payload. The wire format is a CQL tuple:
//
//	tuple<bigint, bigint, list<tuple<uuid, int>>>
//
// Each tuple/list element is length-prefixed with a 4-byte int32.
// This replaces the generic Unmarshal path which used reflection and
// allocated 34 objects per call; the direct parser does 1 allocation
// (the replicas slice) and is ~22x faster.
func ParseHint(data []byte, keyspace, table string) (TabletInfo, error) {
	r := cqlproto.NewReader(data)

	firstToken, _ := r.ReadBigInt()
	lastToken, _ := r.ReadBigInt()

	// list<tuple<uuid, int>>: read the list [bytes] envelope, then element count.
	listBody := r.ReadBytes()
	if r.Err() != nil {
		return TabletInfo{}, r.Err()
	}

	listR := cqlproto.NewReader(listBody)
	count := listR.ReadCollectionCount()
	if listR.Err() != nil {
		return TabletInfo{}, listR.Err()
	}
	if count < 0 || count > 1000 {
		return TabletInfo{}, fmt.Errorf("unreasonable replica count: %d", count)
	}

	replicas := make([]ReplicaInfo, count)
	for i := 0; i < count; i++ {
		// Each replica is a tuple<uuid, int> wrapped in a [bytes] envelope.
		tupleBody := listR.ReadBytes()
		if listR.Err() != nil {
			return TabletInfo{}, listR.Err()
		}
		tupleR := cqlproto.NewReader(tupleBody)

		hostUUID, _ := tupleR.ReadUUID()
		shardID, _ := tupleR.ReadInt()
		if tupleR.Err() != nil {
			return TabletInfo{}, fmt.Errorf("replica %d: %w", i, tupleR.Err())
		}

		replicas[i] = NewReplicaInfo(HostUUID(hostUUID), int(shardID))
	}

	return NewTabletInfo(keyspace, table, firstToken, lastToken, replicas)
}
