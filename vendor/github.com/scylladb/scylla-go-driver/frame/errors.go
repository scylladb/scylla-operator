package frame

// This file is identical to errors.go located in scylla pkg.
// Error codes are necessary in frame pkg for parsing responses,
// and we need to have them in scylla pkg for user convenience.

// See CQL Binary Protocol v4, section 9 for more details.
// https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1046-L1200
const (
	// ErrCodeServer indicates unexpected error on server-side.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1051-L1052
	ErrCodeServer ErrorCode = 0x0000

	// ErrCodeProtocol indicates a protocol violation by some client message.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1053-L1055
	ErrCodeProtocol ErrorCode = 0x000A

	// ErrCodeCredentials indicates missing required authentication.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1056-L1059
	ErrCodeCredentials ErrorCode = 0x0100

	// ErrCodeUnavailable indicates unavailable error.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1060-L1070
	ErrCodeUnavailable ErrorCode = 0x1000

	// ErrCodeOverloaded returned in case of request on overloaded node coordinator.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1071-L1072
	ErrCodeOverloaded ErrorCode = 0x1001

	// ErrCodeBootstrapping returned from the coordinator node in bootstrapping phase.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1073-L1074
	ErrCodeBootstrapping ErrorCode = 0x1002

	// ErrCodeTruncate indicates truncation exception.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1075
	ErrCodeTruncate ErrorCode = 0x1003

	// ErrCodeWriteTimeout returned in case of timeout during the request write.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1076-L1107
	ErrCodeWriteTimeout ErrorCode = 0x1100

	// ErrCodeReadTimeout returned in case of timeout during the request read.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1108-L1124
	ErrCodeReadTimeout ErrorCode = 0x1200

	// ErrCodeReadFailure indicates request read error which is not covered by ErrCodeReadTimeout.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1125-L1139
	ErrCodeReadFailure ErrorCode = 0x1300

	// ErrCodeFunctionFailure indicates an error in user-defined function.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1140-L1146
	ErrCodeFunctionFailure ErrorCode = 0x1400

	// ErrCodeWriteFailure indicates request write error which is not covered by ErrCodeWriteTimeout.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1147-L1180
	ErrCodeWriteFailure ErrorCode = 0x1500

	// ErrCodeSyntax indicates the syntax error in the query.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1182
	ErrCodeSyntax ErrorCode = 0x2000

	// ErrCodeUnauthorized indicates access rights violation by user on performed operation.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1183-L1184
	ErrCodeUnauthorized ErrorCode = 0x2100

	// ErrCodeInvalid indicates invalid query error which is not covered by ErrCodeSyntax.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1185
	ErrCodeInvalid ErrorCode = 0x2200

	// ErrCodeConfig indicates the configuration error.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1186
	ErrCodeConfig ErrorCode = 0x2300

	// ErrCodeAlreadyExists is returned for the requests creating the existing keyspace/table.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1187-L1196
	ErrCodeAlreadyExists ErrorCode = 0x2400

	// ErrCodeUnprepared returned from the host for prepared statement which is unknown.
	// See https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1197-L1200
	ErrCodeUnprepared ErrorCode = 0x2500
)
