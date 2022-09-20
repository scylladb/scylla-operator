package frame

// Package frame implements generic functions for reading and writing types from CQL binary protocol.
// Reading from and writing to is done in Big Endian order.
// Frame currently supports v4 protocol:
// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec
