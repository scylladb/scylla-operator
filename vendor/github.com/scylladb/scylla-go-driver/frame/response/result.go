package response

import (
	"log"

	"github.com/scylladb/scylla-go-driver/frame"
)

// Result spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L546
// Below are types of Result with different bodies.

// VoidResult spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L562
type VoidResult struct{}

func ParseVoidResult(_ *frame.Buffer) *VoidResult {
	return &VoidResult{}
}

// RowsResult spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L568
type RowsResult struct {
	Metadata    frame.ResultMetadata
	RowsCnt     frame.Int
	RowsContent []frame.Row
}

const (
	// 5333334 is equal to 128MB divided by slice size.
	maxRowSliceSize = 5_333_334
	maxRowSize      = 5_333_334
	maxCellsCnt     = 128_000_000 // TODO: Theoretically should be equal to 2_000_000_000 and limits above should not exist.
)

func ParseRowsResult(b *frame.Buffer) *RowsResult {
	r := RowsResult{
		Metadata: b.ReadResultMetadata(),
		RowsCnt:  b.ReadInt(),
	}

	if b.Error() != nil ||
		r.RowsCnt < 0 || maxRowSliceSize < r.RowsCnt ||
		r.Metadata.ColumnsCnt < 0 || maxRowSize < r.Metadata.ColumnsCnt ||
		r.RowsCnt*r.Metadata.ColumnsCnt < 0 || maxCellsCnt < r.RowsCnt*r.Metadata.ColumnsCnt {
		return nil
	}
	r.RowsContent = make([]frame.Row, r.RowsCnt)
	// holder is used to avoid many small allocations.
	holder := make(frame.Row, r.RowsCnt*r.Metadata.ColumnsCnt)
	for i := range r.RowsContent {
		if b.Error() != nil {
			return nil
		}
		r.RowsContent[i] = holder[i*int(r.Metadata.ColumnsCnt) : (i+1)*int(r.Metadata.ColumnsCnt)]
		for j := 0; j < int(r.Metadata.ColumnsCnt); j++ {
			r.RowsContent[i][j] = frame.CqlValue{
				Value: b.ReadBytes(),
			}

			if r.Metadata.Columns != nil {
				r.RowsContent[i][j].Type = &r.Metadata.Columns[j].Type
			}
		}
	}

	return &r
}

// SetKeyspaceResult spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L669
type SetKeyspaceResult struct {
	Name string
}

func ParseSetKeyspaceResult(b *frame.Buffer) *SetKeyspaceResult {
	return &SetKeyspaceResult{
		Name: b.ReadString(),
	}
}

// PreparedResult spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L675
type PreparedResult struct {
	ID             frame.ShortBytes
	Metadata       frame.PreparedMetadata
	ResultMetadata frame.ResultMetadata
}

func ParsePreparedResult(b *frame.Buffer) *PreparedResult {
	return &PreparedResult{
		ID:             b.ReadShortBytes(),
		Metadata:       b.ReadPreparedMetadata(),
		ResultMetadata: b.ReadResultMetadata(),
	}
}

// SchemaChangeResult spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L742
type SchemaChangeResult struct {
	SchemaChange SchemaChange
}

func ParseSchemaChangeResult(b *frame.Buffer) *SchemaChangeResult {
	return &SchemaChangeResult{
		SchemaChange: *ParseSchemaChange(b),
	}
}

type ResultKind = frame.Int

const (
	VoidKind         ResultKind = 1
	RowsKind         ResultKind = 2
	SetKeySpaceKind  ResultKind = 3
	PreparedKind     ResultKind = 4
	SchemaChangeKind ResultKind = 5
)

func ParseResult(b *frame.Buffer) frame.Response {
	resultKind := b.ReadInt()
	switch resultKind {
	case VoidKind:
		return ParseVoidResult(b)
	case RowsKind:
		return ParseRowsResult(b)
	case SetKeySpaceKind:
		return ParseSetKeyspaceResult(b)
	case PreparedKind:
		return ParsePreparedResult(b)
	case SchemaChangeKind:
		return ParseSchemaChangeResult(b)
	default:
		log.Printf("result kind not supported: %d", resultKind)
		return nil
	}
}
