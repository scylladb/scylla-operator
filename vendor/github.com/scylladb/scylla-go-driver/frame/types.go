package frame

import (
	"errors"
	"net"
)

// Generic types from CQL binary protocol.
// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L214-L266
type (
	Int            = int32
	Long           = int64
	Short          = uint16
	Byte           = byte
	UUID           = [16]byte
	StringList     = []string
	Bytes          = []byte
	ShortBytes     = []byte
	StringMap      = map[string]string
	StringMultiMap = map[string][]string
	BytesMap       = map[string]Bytes
)

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L229-L233
type Value struct {
	N     Int
	Bytes Bytes
}

func (v Value) Clone() Value {
	c := Value{
		N: v.N,
	}
	if len(v.Bytes) != 0 {
		c.Bytes = make(Bytes, len(v.Bytes))
		copy(c.Bytes, v.Bytes)
	}
	return c
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L241-L245
type Inet struct {
	IP   Bytes
	Port Int
}

// String only takes care of IP part of the address.
func (i Inet) String() string {
	return net.IP(i.IP).String()
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L183-L201
type OpCode = Byte

const (
	OpError         OpCode = 0x00
	OpStartup       OpCode = 0x01
	OpReady         OpCode = 0x02
	OpAuthenticate  OpCode = 0x03
	OpOptions       OpCode = 0x05
	OpSupported     OpCode = 0x06
	OpQuery         OpCode = 0x07
	OpResult        OpCode = 0x08
	OpPrepare       OpCode = 0x09
	OpExecute       OpCode = 0x0A
	OpRegister      OpCode = 0x0B
	OpEvent         OpCode = 0x0C
	OpBatch         OpCode = 0x0D
	OpAuthChallenge OpCode = 0x0E
	OpAuthResponse  OpCode = 0x0F
	OpAuthSuccess   OpCode = 0x10
)

var allOpCodes = map[OpCode]struct{}{
	OpError:         {},
	OpStartup:       {},
	OpReady:         {},
	OpAuthenticate:  {},
	OpOptions:       {},
	OpSupported:     {},
	OpQuery:         {},
	OpResult:        {},
	OpPrepare:       {},
	OpExecute:       {},
	OpRegister:      {},
	OpEvent:         {},
	OpBatch:         {},
	OpAuthChallenge: {},
	OpAuthResponse:  {},
	OpAuthSuccess:   {},
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L246-L259
type Consistency = Short

const (
	ANY         Consistency = 0x0000
	ONE         Consistency = 0x0001
	TWO         Consistency = 0x0002
	THREE       Consistency = 0x0003
	QUORUM      Consistency = 0x0004
	ALL         Consistency = 0x0005
	LOCALQUORUM Consistency = 0x0006
	EACHQUORUM  Consistency = 0x0007
	SERIAL      Consistency = 0x0008
	LOCALSERIAL Consistency = 0x0009
	LOCALONE    Consistency = 0x000A
)

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L502
type ErrorCode = Int

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L125-L158
type HeaderFlags = Byte

const (
	Compress      HeaderFlags = 0x01
	Tracing       HeaderFlags = 0x02
	CustomPayload HeaderFlags = 0x04
	Warning       HeaderFlags = 0x08
)

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L346-L385
type QueryFlags = Byte

const (
	Values                QueryFlags = 0x01
	SkipMetadata          QueryFlags = 0x02
	PageSize              QueryFlags = 0x04
	WithPagingState       QueryFlags = 0x08
	WithSerialConsistency QueryFlags = 0x10
	WithDefaultTimestamp  QueryFlags = 0x20
	WithNamesForValues    QueryFlags = 0x40
)

type (
	// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L576-L594
	ResultFlags = Int

	// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L684-L690
	PreparedFlags = Int
)

const (
	GlobalTablesSpec ResultFlags = 0x0001
	HasMorePages     ResultFlags = 0x0002
	NoMetadata       ResultFlags = 0x0004
)

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L421-L426
type BatchTypeFlag = byte

const (
	LoggedBatchFlag   BatchTypeFlag = 0
	UnloggedBatchFlag BatchTypeFlag = 1
	CounterBatchFlag  BatchTypeFlag = 2
)

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L456
type BatchQueryKind = byte

// CQLv4 is the only protocol version currently supported.
const CQLv4 Byte = 0x4

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1086-L1107
type WriteType string

const (
	Simple        WriteType = "SIMPLE"
	Batch         WriteType = "BATCH"
	UnloggedBatch WriteType = "UNLOGGED_BATCH"
	Counter       WriteType = "COUNTER"
	BatchLog      WriteType = "BATCH_LOG"
	CAS           WriteType = "CAS"
	View          WriteType = "VIEW"
	CDC           WriteType = "CDC"
)

var allWriteTypes = map[WriteType]struct{}{
	Simple:        {},
	Batch:         {},
	UnloggedBatch: {},
	Counter:       {},
	BatchLog:      {},
	CAS:           {},
	View:          {},
	CDC:           {},
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L757-L791
type EventType = string

const (
	TopologyChange EventType = "TOPOLOGY_CHANGE"
	StatusChange   EventType = "STATUS_CHANGE"
	SchemaChange   EventType = "SCHEMA_CHANGE"
)

var allEventTypes = map[EventType]struct{}{
	TopologyChange: {},
	StatusChange:   {},
	SchemaChange:   {},
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L760-L765
type TopologyChangeType string

const (
	NewNode     TopologyChangeType = "NEW_NODE"
	RemovedNode TopologyChangeType = "REMOVED_NODE"
)

var allTopologyChangeTypes = map[TopologyChangeType]struct{}{
	NewNode:     {},
	RemovedNode: {},
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L766-L770
type StatusChangeType string

const (
	Up   StatusChangeType = "UP"
	Down StatusChangeType = "DOWN"
)

var allStatusChangeTypes = map[StatusChangeType]struct{}{
	Up:   {},
	Down: {},
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L771-L791
type SchemaChangeType string

const (
	Created SchemaChangeType = "CREATED"
	Updated SchemaChangeType = "UPDATED"
	Dropped SchemaChangeType = "DROPPED"
)

var allSchemaChangeTypes = map[SchemaChangeType]struct{}{
	Created: {},
	Updated: {},
	Dropped: {},
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L775-L779
type SchemaChangeTarget string

const (
	Keyspace  SchemaChangeTarget = "KEYSPACE"
	Table     SchemaChangeTarget = "TABLE"
	UserType  SchemaChangeTarget = "TYPE"
	Function  SchemaChangeTarget = "FUNCTION"
	Aggregate SchemaChangeTarget = "AGGREGATE"
)

var allSchemaChangeTargets = map[SchemaChangeTarget]struct{}{
	Keyspace:  {},
	Table:     {},
	UserType:  {},
	Function:  {},
	Aggregate: {},
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L296-L308
type StartupOptions StringMap

type Compression string

const (
	Lz4    Compression = "lz4"
	Snappy Compression = "snappy"
)

// Mandatory values and keys that can be given in Startup body
// value in the map means option name and key means its possible values.
var mandatoryOptions = StringMultiMap{
	"CQL_VERSION": {
		"3.0.0",
		"4.0.0",
	},
}

var possibleOptions = StringMultiMap{
	"COMPRESSION": {
		"lz4",
		"snappy",
	},
	"NO_COMPACT":        {},
	"THROW_ON_OVERLOAD": {},
}

// QueryOptions represent optional Values defined by flags.
// Consists of Values required for all flags.
// Values for unset flags are uninitialized.
// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L350-L385
type QueryOptions struct {
	Flags             QueryFlags
	Values            []Value
	Names             StringList
	PageSize          Int
	PagingState       Bytes
	SerialConsistency Consistency
	Timestamp         Long
}

func (q *QueryOptions) SetFlags() {
	if q.Values != nil {
		q.Flags |= Values
	}
	if q.PageSize != 0 {
		q.Flags |= PageSize
	}
	if q.PagingState != nil {
		q.Flags |= WithPagingState
	}
	if q.SerialConsistency != 0 {
		q.Flags |= WithSerialConsistency
	}
	if q.Timestamp != 0 {
		q.Flags |= WithDefaultTimestamp
	}
	if q.Names != nil {
		q.Flags |= WithNamesForValues
	}
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L236-L239
type OptionID Short

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L615-L658
// https://github.com/apache/cassandra/blob/881b08f7015a4342833079e648e478526cc3b31a/doc/native_protocol_v5.spec#L1050-L1210
const (
	CustomID    OptionID = 0x0000
	ASCIIID     OptionID = 0x0001
	BigIntID    OptionID = 0x0002
	BlobID      OptionID = 0x0003
	BooleanID   OptionID = 0x0004
	CounterID   OptionID = 0x0005
	DecimalID   OptionID = 0x0006
	DoubleID    OptionID = 0x0007
	FloatID     OptionID = 0x0008
	IntID       OptionID = 0x0009
	TimestampID OptionID = 0x000B
	UUIDID      OptionID = 0x000C
	VarcharID   OptionID = 0x000D
	VarintID    OptionID = 0x000E
	TimeUUIDID  OptionID = 0x000F
	InetID      OptionID = 0x0010
	DateID      OptionID = 0x0011
	TimeID      OptionID = 0x0012
	SmallIntID  OptionID = 0x0013
	TinyIntID   OptionID = 0x0014
	DurationID  OptionID = 0x0015
	ListID      OptionID = 0x0020
	MapID       OptionID = 0x0021
	SetID       OptionID = 0x0022
	UDTID       OptionID = 0x0030
	TupleID     OptionID = 0x0031
)

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L612-L617
type CustomOption struct {
	Name string
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L637-L638
type ListOption struct {
	Element Option
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L639-L640
type MapOption struct {
	Key   Option
	Value Option
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L641-L642
type SetOption struct {
	Element Option
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L643-L654
type UDTOption struct {
	Keyspace   string
	Name       string
	fieldNames []string
	fieldTypes []Option
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L655-L658
type TupleOption struct {
	ValueTypes []Option
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L236-L239
type Option struct {
	ID     OptionID
	Custom *CustomOption
	List   *ListOption
	Map    *MapOption
	Set    *SetOption
	UDT    *UDTOption
	Tuple  *TupleOption
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L240
type OptionList []Option

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L573-L658
type ResultMetadata struct {
	Flags      ResultFlags
	ColumnsCnt Int

	// nil if flagPagingState is not set.
	PagingState    Bytes
	GlobalKeyspace string
	GlobalTable    string

	Columns []ColumnSpec
}

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L601-L658
type ColumnSpec struct {
	Keyspace string
	Table    string
	Name     string
	Type     Option
}

type Row []CqlValue

// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L681-L724
type PreparedMetadata struct {
	Flags          PreparedFlags
	ColumnsCnt     Int
	PkCnt          Int
	PkIndexes      []Short
	GlobalKeyspace string
	GlobalTable    string
	Columns        []ColumnSpec
}

type Duration struct {
	Months      int32
	Days        int32
	Nanoseconds int64
}

var errInvalidDuration = errors.New("duration fields must be all positive or all negative")

// validate checks that the Duration complies with the protocol specification.
//
// If there is an issue, validate returns an error.
func (d Duration) validate() error {
	// All the fields must have the same sign (or be zero).
	// https://github.com/apache/cassandra/blob/afa7dfb5a48ecb56abc2d8bbb1de0fc8f1ca77b9/doc/native_protocol_v5.spec#L1107-L1116
	// We compare all three pairs to account for zero fields.
	months := int64(d.Months)
	days := int64(d.Days)
	valid := sameSign(months, days) && sameSign(days, d.Nanoseconds) && sameSign(months, d.Nanoseconds)
	if !valid {
		return errInvalidDuration
	}
	return nil
}

func sameSign(a, b int64) bool {
	return a == 0 || b == 0 || (a < 0) == (b < 0)
}
