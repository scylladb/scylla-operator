package response

import (
	"log"
	"strconv"

	"github.com/scylladb/scylla-go-driver/frame"
)

// Supported spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L537
type Supported struct {
	Options frame.StringMultiMap
}

func ParseSupported(b *frame.Buffer) *Supported {
	return &Supported{
		Options: b.ReadStringMultiMap(),
	}
}

// ScyllaSupported represents Scylla connection options as sent in SUPPORTED
// https://github.com/scylladb/scylla/blob/4bfcead2ba60072c720241cce6f42f620930c380/docs/dev/protocol-extensions.md#intranode-sharding
type ScyllaSupported struct {
	Shard             uint16
	NrShards          uint16
	MsbIgnore         uint8
	Partitioner       string
	ShardingAlgorithm string
	ShardAwarePort    uint16
	ShardAwarePortSSL uint16
	LwtFlagMask       int
}

const (
	ScyllaShard             = "SCYLLA_SHARD"
	ScyllaNrShards          = "SCYLLA_NR_SHARDS"
	ScyllaPartitioner       = "SCYLLA_PARTITIONER"
	ScyllaShardingAlgorithm = "SCYLLA_SHARDING_ALGORITHM"
	ScyllaShardingIgnoreMSB = "SCYLLA_SHARDING_IGNORE_MSB"
	ScyllaShardAwarePort    = "SCYLLA_SHARD_AWARE_PORT"
	ScyllaShardAwarePortSSL = "SCYLLA_SHARD_AWARE_PORT_SSL"
)

func (s *Supported) ScyllaSupported() *ScyllaSupported {
	// This variable is filled during function
	var si ScyllaSupported

	if s, ok := s.Options[ScyllaShard]; ok {
		if shard, err := strconv.ParseUint(s[0], 10, 16); err != nil {
			if frame.Debug {
				log.Printf("scylla: failed to parse %s value %v: %s", ScyllaShard, s, err)
			}
		} else {
			si.Shard = uint16(shard)
		}
	}
	if s, ok := s.Options[ScyllaNrShards]; ok {
		if nrShards, err := strconv.ParseUint(s[0], 10, 16); err != nil {
			if frame.Debug {
				log.Printf("scylla: failed to parse %s value %v: %s", ScyllaNrShards, s, err)
			}
		} else {
			si.NrShards = uint16(nrShards)
		}
	}
	if s, ok := s.Options[ScyllaShardingIgnoreMSB]; ok {
		if msbIgnore, err := strconv.ParseUint(s[0], 10, 8); err != nil {
			if frame.Debug {
				log.Printf("scylla: failed to parse %s value %v: %s", ScyllaShardingIgnoreMSB, s, err)
			}
		} else {
			si.MsbIgnore = uint8(msbIgnore)
		}
	}
	if s, ok := s.Options[ScyllaShardAwarePort]; ok {
		if shardAwarePort, err := strconv.ParseUint(s[0], 10, 16); err != nil {
			if frame.Debug {
				log.Printf("scylla: failed to parse %s value %v: %s", ScyllaShardAwarePort, s, err)
			}
		} else {
			si.ShardAwarePort = uint16(shardAwarePort)
		}
	}
	if s, ok := s.Options[ScyllaShardAwarePortSSL]; ok {
		if shardAwarePortSSL, err := strconv.ParseUint(s[0], 10, 16); err != nil {
			if frame.Debug {
				log.Printf("scylla: failed to parse %s value %v: %s", ScyllaShardAwarePortSSL, s, err)
			}
		} else {
			si.ShardAwarePortSSL = uint16(shardAwarePortSSL)
		}
	}

	if s, ok := s.Options[ScyllaPartitioner]; ok {
		si.Partitioner = s[0]
	}
	if s, ok := s.Options[ScyllaShardingAlgorithm]; ok {
		si.ShardingAlgorithm = s[0]
	}

	// Currently, only one sharding algorithm is defined, and it is 'biased-token-round-robin'.
	// For now we only support 'Murmur3Partitioner', it is due to change in the future.
	if si.Partitioner != "org.apache.cassandra.dht.Murmur3Partitioner" ||
		si.ShardingAlgorithm != "biased-token-round-robin" || si.NrShards == 0 || si.MsbIgnore == 0 {
		if frame.Debug {
			log.Printf(`scylla: unsupported sharding configuration, partitioner=%s, algorithm=%s, 
						no_shards=%d, msb_ignore=%d`, si.Partitioner, si.ShardingAlgorithm, si.NrShards, si.MsbIgnore)
		}
		return &ScyllaSupported{}
	}

	return &si
}
