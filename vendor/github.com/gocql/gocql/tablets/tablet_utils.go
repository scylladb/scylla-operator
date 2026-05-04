package tablets

import (
	"encoding/binary"
	"math"
	"math/rand"
	"sync/atomic"

	"github.com/gocql/gocql/internal/tests"
)

const randSeed = 100

// ReplicaSetGenerator generates all possible k-combinations (replica sets) of a given list of hosts,
// where each combination contains `rf` elements. The generator cycles through all possible combinations
// infinitely in a thread-safe manner using an atomic counter.
type ReplicaSetGenerator struct {
	hosts   []HostUUID // List of available hosts
	rf      int        // Replication factor (number of hosts per combination)
	len     int        // Total number of hosts
	counter uint64     // Current position in the sequence of combinations
	total   uint64     // Total number of possible combinations (n choose rf)
}

// NewReplicaSetGenerator creates and returns a new ReplicaSetGenerator for the given set of hosts
// and replication factor `rf`. It panics if `rf` is non-positive or greater than the number of hosts.
// The generator produces all k-combinations of the input set and loops over them indefinitely.
func NewReplicaSetGenerator(hosts []HostUUID, rf int) *ReplicaSetGenerator {
	n := len(hosts)
	if rf <= 0 {
		panic("replication factor must be positive")
	}
	if rf > len(hosts) {
		panic("replication factor cannot exceed number of hosts")
	}
	return &ReplicaSetGenerator{
		hosts: hosts,
		rf:    rf,
		len:   n,
		total: uint64(binomial(n, rf)),
	}
}

// Next returns the next replica set as a slice of ReplicaInfo. The combinations are returned in a
// deterministic order and wrap around after exhausting all possible combinations.
// This method is safe for concurrent use.
func (it *ReplicaSetGenerator) Next() []ReplicaInfo {
	// Advance and wrap around
	counter := atomic.AddUint64(&it.counter, 1) % it.total
	// Map current counter to combination
	return unrankCombination(it.len, it.rf, int(counter), it.hosts)
}

// binomial calculates the number of unique combinations (n choose k)
// for selecting `rf` elements from a set of `hosts` elements.
//
// It returns the binomial coefficient C(hosts, rf), which represents
// the number of ways to choose `rf` items from a total of `hosts` without
// regard to order.
//
// If rf < 0 or rf > hosts, the function returns 0.
// If rf == 0 or rf == hosts, the function returns 1.
func binomial(hosts, rf int) int {
	if rf < 0 || rf > hosts {
		return 0
	}
	if rf == 0 || rf == hosts {
		return 1
	}
	num := 1
	den := 1
	for i := 1; i <= rf; i++ {
		num *= hosts - (i - 1)
		den *= i
	}
	return num / den
}

// unrankCombination returns the k-combination of elements from the input slice
// corresponding to the given rank (counter) in lexicographic order.
func unrankCombination(n, k, counter int, input []HostUUID) []ReplicaInfo {
	comb := make([]ReplicaInfo, 0, k)
	x := 0
	for i := 0; i < k; i++ {
		for {
			b := binomial(n-x-1, k-i-1)
			if counter < b {
				comb = append(comb, ReplicaInfo{
					hostId:  input[x],
					shardId: 0,
				})
				x++
				break
			} else {
				counter -= b
				x++
			}
		}
	}
	return comb
}

func getThreadSafeRnd() *tests.ThreadSafeRand {
	return tests.NewThreadSafeRand(randSeed)
}

func getRnd() *rand.Rand {
	return rand.New(rand.NewSource(randSeed))
}

// GenerateHostUUIDs generates a slice of deterministic HostUUIDs for testing.
// Byte 0 is set to 0xFE so that even index 0 is never the zero UUID.
func GenerateHostUUIDs(count int) []HostUUID {
	hosts := make([]HostUUID, count)
	for i := range hosts {
		hosts[i][0] = 0xFE
		binary.BigEndian.PutUint64(hosts[i][8:], uint64(i))
	}
	return hosts
}

// createTablets generates a list of TabletInfo entries for a given keyspace and table.
// Each tablet is assigned a token range and a set of replica hosts.
func createTablets(ks, table string, hosts []HostUUID, rf, count int, tokenRangeCount int64) TabletInfoList {
	out := make(TabletInfoList, count)
	step := math.MaxUint64 / uint64(tokenRangeCount)
	repGen := NewReplicaSetGenerator(hosts, rf)
	firstToken := int64(math.MinInt64)
	for i := 0; i < count; i++ {
		out[i] = TabletInfo{
			keyspaceName: ks,
			tableName:    table,
			firstToken:   firstToken,
			lastToken:    firstToken + int64(step),
			replicas:     repGen.Next(),
		}
		firstToken = firstToken + int64(step)
	}
	return out
}
