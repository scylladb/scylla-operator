package murmur

// Package murmur implements third version of https://en.wikipedia.org/wiki/MurmurHash.
// It's very similar to https://github.com/scylladb/scylla/blob/master/utils/murmur_hash.hh.
// It's a function for computing which data is stored on which node in the cluster.
// The partitioner takes a partition key as an input, and returns a ring token as an output.
// Murmurhash3 is a default hash function for Scylla.
