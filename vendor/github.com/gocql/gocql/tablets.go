package gocql

import (
	"sync"
)

type ReplicaInfo struct {
	hostId  UUID
	shardId int
}

type TabletInfo struct {
	keyspaceName string
	tableName    string
	firstToken   int64
	lastToken    int64
	replicas     []ReplicaInfo
}

func (t *TabletInfo) KeyspaceName() string {
	return t.keyspaceName
}

func (t *TabletInfo) FirstToken() int64 {
	return t.firstToken
}

func (t *TabletInfo) LastToken() int64 {
	return t.lastToken
}

func (t *TabletInfo) TableName() string {
	return t.tableName
}

func (t *TabletInfo) Replicas() []ReplicaInfo {
	return t.replicas
}

type TabletInfoList []*TabletInfo

// Search for place in tablets table with specific Keyspace and Table name
func (t TabletInfoList) findTablets(keyspace string, table string) (int, int) {
	l := -1
	r := -1
	for i, tablet := range t {
		if tablet.KeyspaceName() == keyspace && tablet.TableName() == table {
			if l == -1 {
				l = i
			}
			r = i
		} else if l != -1 {
			break
		}
	}

	return l, r
}

func (t TabletInfoList) addTabletToTabletsList(tablet *TabletInfo) TabletInfoList {
	l, r := t.findTablets(tablet.keyspaceName, tablet.tableName)
	if l == -1 && r == -1 {
		l = 0
		r = 0
	} else {
		r = r + 1
	}

	l1, r1 := l, r
	l2, r2 := l1, r1

	// find first overlaping range
	for l1 < r1 {
		mid := (l1 + r1) / 2
		if t[mid].FirstToken() < tablet.FirstToken() {
			l1 = mid + 1
		} else {
			r1 = mid
		}
	}
	start := l1

	if start > l && t[start-1].LastToken() > tablet.FirstToken() {
		start = start - 1
	}

	// find last overlaping range
	for l2 < r2 {
		mid := (l2 + r2) / 2
		if t[mid].LastToken() < tablet.LastToken() {
			l2 = mid + 1
		} else {
			r2 = mid
		}
	}
	end := l2
	if end < r && t[end].FirstToken() >= tablet.LastToken() {
		end = end - 1
	}
	if end == len(t) {
		end = end - 1
	}

	updated_tablets := t
	if start <= end {
		// Delete elements from index start to end
		updated_tablets = append(t[:start], t[end+1:]...)
	}
	// Insert tablet element at index start
	t = append(updated_tablets[:start], append([]*TabletInfo{tablet}, updated_tablets[start:]...)...)
	return t
}

// Remove all tablets that have given host as a replica
func (t TabletInfoList) removeTabletsWithHostFromTabletsList(host *HostInfo) TabletInfoList {
	filteredTablets := make([]*TabletInfo, 0, len(t)) // Preallocate for efficiency

	for _, tablet := range t {
		// Check if any replica matches the given host ID
		shouldExclude := false
		for _, replica := range tablet.replicas {
			if replica.hostId.String() == host.HostID() {
				shouldExclude = true
				break
			}
		}
		if !shouldExclude {
			filteredTablets = append(filteredTablets, tablet)
		}
	}

	t = filteredTablets
	return t
}

func (t TabletInfoList) removeTabletsWithKeyspaceFromTabletsList(keyspace string) TabletInfoList {
	filteredTablets := make([]*TabletInfo, 0, len(t))

	for _, tablet := range t {
		if tablet.keyspaceName != keyspace {
			filteredTablets = append(filteredTablets, tablet)
		}
	}

	t = filteredTablets
	return t
}

func (t TabletInfoList) removeTabletsWithTableFromTabletsList(keyspace string, table string) TabletInfoList {
	filteredTablets := make([]*TabletInfo, 0, len(t))

	for _, tablet := range t {
		if !(tablet.keyspaceName == keyspace && tablet.tableName == table) {
			filteredTablets = append(filteredTablets, tablet)
		}
	}

	t = filteredTablets
	return t
}

// Search for place in tablets table for token starting from index l to index r
func (t TabletInfoList) findTabletForToken(token Token, l int, r int) *TabletInfo {
	for l < r {
		var m int
		if r*l > 0 {
			m = l + (r-l)/2
		} else {
			m = (r + l) / 2
		}
		if int64Token(t[m].LastToken()).Less(token) {
			l = m + 1
		} else {
			r = m
		}
	}

	return t[l]
}

// cowTabletList implements a copy on write tablet list, its equivalent type is TabletInfoList
type cowTabletList struct {
	list TabletInfoList
	mu   sync.RWMutex
}

func (c *cowTabletList) get() TabletInfoList {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.list
}

func (c *cowTabletList) set(tablets TabletInfoList) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.list = tablets
}
