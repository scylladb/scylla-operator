package tablets

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type ReplicaInfo struct {
	// hostId for sake of better performance, it has to be same type as HostInfo.hostId
	hostId  string
	shardId int
}

func (r ReplicaInfo) HostID() string {
	return r.hostId
}

func (r ReplicaInfo) ShardID() int {
	return r.shardId
}

func (r ReplicaInfo) String() string {
	return fmt.Sprintf("ReplicaInfo{hostId:%s, shardId:%d}", r.hostId, r.shardId)
}

type TabletInfoBuilder struct {
	KeyspaceName string
	TableName    string
	Replicas     [][]interface{}
	FirstToken   int64
	LastToken    int64
}

func NewTabletInfoBuilder() TabletInfoBuilder {
	return TabletInfoBuilder{}
}

type toString interface {
	String() string
}

func (b TabletInfoBuilder) Build() (*TabletInfo, error) {
	tabletReplicas := make([]ReplicaInfo, 0, len(b.Replicas))
	for _, replica := range b.Replicas {
		if len(replica) != 2 {
			return nil, fmt.Errorf("replica info should have exactly two elements, but it has %d: %v", len(replica), replica)
		}
		if hostId, ok := replica[0].(toString); ok {
			if shardId, ok := replica[1].(int); ok {
				repInfo := ReplicaInfo{hostId.String(), shardId}
				tabletReplicas = append(tabletReplicas, repInfo)
			} else {
				return nil, fmt.Errorf("second element (shard) of replica is not int: %v", replica)
			}
		} else {
			return nil, fmt.Errorf("first element (hostID) of replica is not UUID: %v", replica)
		}
	}

	return &TabletInfo{
		keyspaceName: b.KeyspaceName,
		tableName:    b.TableName,
		firstToken:   b.FirstToken,
		lastToken:    b.LastToken,
		replicas:     tabletReplicas,
	}, nil
}

type TabletInfo struct {
	keyspaceName string
	tableName    string
	replicas     []ReplicaInfo
	firstToken   int64
	lastToken    int64
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

// FindTablets returns the range [l, r] of indices within the TabletInfoList
// that correspond to consecutive tablets matching the given keyspace and table.
//
// If no matching tablets are found, both l and r are set to -1.
// The search stops at the first non-matching tablet after finding the first match.
//
// Parameters:
//
//	keyspace - the name of the keyspace to match.
//	table    - the name of the table to match.
//
// Returns:
//
//	l - the index of the first matching tablet.
//	r - the index of the last matching tablet in the contiguous block.
func (t TabletInfoList) FindTablets(keyspace string, table string) (int, int) {
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

// AddTabletToTabletsList inserts a new tablet into the TabletInfoList while preserving sorted order
// and removing any existing overlapping tablets for the same keyspace and table.
//
// It first locates the range of tablets corresponding to the same keyspace and table,
// then determines the overlapping region (if any) based on token ranges.
// Any overlapping tablets in that range are removed, and the new tablet is inserted
// at the appropriate position.
//
// Parameters:
//
//	tablet - pointer to the TabletInfo to be added.
//
// Returns:
//
//	A new TabletInfoList with the given tablet inserted and any overlapping tablets removed.
func (t TabletInfoList) AddTabletToTabletsList(tablet *TabletInfo) TabletInfoList {
	l, r := t.FindTablets(tablet.keyspaceName, tablet.tableName)
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

// BulkAddTabletsToTabletsList inserts a sorted list of tablets into the TabletInfoList,
// replacing any overlapping tablets for the same keyspace and table.
//
// The method assumes the input tablets are sorted by token range. It locates the existing
// tablet range matching the keyspace and table, finds and removes any tablets whose token
// ranges overlap with the new ones, and inserts the new tablets at the appropriate position.
//
// Parameters:
//
//	tablets - a slice of *TabletInfo to insert.
//
// Returns:
//
//	A new TabletInfoList with the given tablets inserted and any overlapping tablets removed.
func (t TabletInfoList) BulkAddTabletsToTabletsList(tablets []*TabletInfo) TabletInfoList {
	firstToken := tablets[0].FirstToken()
	lastToken := tablets[len(tablets)-1].LastToken()
	l, r := t.FindTablets(tablets[0].keyspaceName, tablets[0].tableName)
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
		if t[mid].FirstToken() < firstToken {
			l1 = mid + 1
		} else {
			r1 = mid
		}
	}
	start := l1

	if start > l && t[start-1].LastToken() > firstToken {
		start = start - 1
	}

	// find last overlaping range
	for l2 < r2 {
		mid := (l2 + r2) / 2
		if t[mid].LastToken() < lastToken {
			l2 = mid + 1
		} else {
			r2 = mid
		}
	}
	end := l2
	if end < r && t[end].FirstToken() >= lastToken {
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
	t = append(updated_tablets[:start], append(append([]*TabletInfo(nil), tablets...), updated_tablets[start:]...)...)
	return t
}

// RemoveTabletsWithHost returns a new TabletInfoList excluding any tablets
// that have a replica hosted on the specified host ID.
//
// It iterates through the list and filters out tablets where any replica's hostId
// matches the provided value.
//
// Parameters:
//
//	hostID - the ID of the host to filter out.
//
// Returns:
//
//	A new TabletInfoList excluding tablets with replicas on the specified host.
func (t TabletInfoList) RemoveTabletsWithHost(hostID string) TabletInfoList {
	filteredTablets := make([]*TabletInfo, 0, len(t)) // Preallocate for efficiency

	for _, tablet := range t {
		// Check if any replica matches the given host ID
		shouldExclude := false
		for _, replica := range tablet.replicas {
			if replica.hostId == hostID {
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

// RemoveTabletsWithKeyspace returns a new TabletInfoList excluding all tablets
// that belong to the specified keyspace.
//
// It filters out any tablet whose keyspace name matches the given keyspace.
//
// Parameters:
//
//	keyspace - the name of the keyspace to remove.
//
// Returns:
//
//	A new TabletInfoList without tablets from the specified keyspace.
func (t TabletInfoList) RemoveTabletsWithKeyspace(keyspace string) TabletInfoList {
	filteredTablets := make([]*TabletInfo, 0, len(t))

	for _, tablet := range t {
		if tablet.keyspaceName != keyspace {
			filteredTablets = append(filteredTablets, tablet)
		}
	}

	t = filteredTablets
	return t
}

// RemoveTabletsWithTableFromTabletsList returns a new TabletInfoList excluding all tablets
// that belong to the specified keyspace and table.
//
// It filters out any tablet whose keyspace and table name both match the provided values.
//
// Parameters:
//
//	keyspace - the name of the keyspace to remove.
//	table    - the name of the table to remove.
//
// Returns:
//
//	A new TabletInfoList without tablets from the specified keyspace and table.
func (t TabletInfoList) RemoveTabletsWithTableFromTabletsList(keyspace string, table string) TabletInfoList {
	filteredTablets := make([]*TabletInfo, 0, len(t))

	for _, tablet := range t {
		if !(tablet.keyspaceName == keyspace && tablet.tableName == table) {
			filteredTablets = append(filteredTablets, tablet)
		}
	}

	t = filteredTablets
	return t
}

// FindTabletForToken performs a binary search within the specified range [l, r)
// of the TabletInfoList to find the tablet that owns the given token.
//
// It assumes the tablets are sorted by token range and returns the first tablet
// whose LastToken is greater than or equal to the given token.
//
// Parameters:
//
//	token - the token to search for.
//	l     - the start index of the search range (inclusive).
//	r     - the end index of the search range (exclusive).
//
// Returns:
//
//	A pointer to the TabletInfo that owns the token.
func (t TabletInfoList) FindTabletForToken(token int64, l int, r int) *TabletInfo {
	for l < r {
		var m int
		if r*l > 0 {
			m = l + (r-l)/2
		} else {
			m = (r + l) / 2
		}
		if t[m].LastToken() < token {
			l = m + 1
		} else {
			r = m
		}
	}

	return t[l]
}

// CowTabletList is a copy-on-write wrapper around a TabletInfoList.
// It allows concurrent reads without locking by storing the list atomically,
// while ensuring writes are serialized via a mutex to avoid lost updates.
type CowTabletList struct {
	list      atomic.Value // Stores the current TabletInfoList
	writeLock sync.Mutex   // Ensures exclusive access during write operations
}

// NewCowTabletList creates a new CowTabletList instance initialized with an empty TabletInfoList.
func NewCowTabletList() CowTabletList {
	list := atomic.Value{}
	list.Store(make(TabletInfoList, 0))
	return CowTabletList{
		list: list,
	}
}

// Get returns the current snapshot of the tablet list.
// It is safe for concurrent use.
func (c *CowTabletList) Get() TabletInfoList {
	return c.list.Load().(TabletInfoList)
}

// set replaces the current tablet list with the provided one.
// It is not safe for concurrent use and should be called only from within a locked context.
func (c *CowTabletList) set(tablets TabletInfoList) {
	c.list.Store(tablets)
}

// AddTablet adds a single tablet to the list in a thread-safe manner.
func (c *CowTabletList) AddTablet(tablet *TabletInfo) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	c.set(c.Get().AddTabletToTabletsList(tablet))
}

// BulkAddTablets adds multiple tablets to the list in a single atomic update.
func (c *CowTabletList) BulkAddTablets(tablets []*TabletInfo) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	c.set(c.Get().BulkAddTabletsToTabletsList(tablets))
}

// RemoveTabletsWithHost removes all tablets associated with the specified host ID.
func (c *CowTabletList) RemoveTabletsWithHost(hostID string) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	c.set(c.Get().RemoveTabletsWithHost(hostID))
}

// RemoveTabletsWithKeyspace removes all tablets belonging to the given keyspace.
func (c *CowTabletList) RemoveTabletsWithKeyspace(keyspace string) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	c.set(c.Get().RemoveTabletsWithKeyspace(keyspace))
}

// RemoveTabletsWithTableFromTabletsList removes all tablets for the specified keyspace and table.
func (c *CowTabletList) RemoveTabletsWithTableFromTabletsList(keyspace string, table string) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	c.set(c.Get().RemoveTabletsWithTableFromTabletsList(keyspace, table))
}

// FindReplicasForToken returns the replica set responsible for the given token,
// within the specified keyspace and table.
func (c *CowTabletList) FindReplicasForToken(keyspace, table string, token int64) []ReplicaInfo {
	tl := c.FindTabletForToken(keyspace, table, token)
	if tl == nil {
		return nil
	}
	return tl.Replicas()
}

// FindTabletForToken locates the tablet that covers the given token
// for the specified keyspace and table. Returns nil if not found.
func (c *CowTabletList) FindTabletForToken(keyspace, table string, token int64) *TabletInfo {
	tablets := c.Get()
	l, r := tablets.FindTablets(keyspace, table)
	if l == -1 {
		return nil
	}
	return tablets.FindTabletForToken(token, l, r)
}
