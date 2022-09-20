package transport

import (
	"github.com/scylladb/scylla-go-driver/frame"
	. "github.com/scylladb/scylla-go-driver/frame/request"
	. "github.com/scylladb/scylla-go-driver/frame/response"
)

type Statement struct {
	ID                frame.Bytes
	Content           string
	Values            []frame.Value
	PkIndexes         []frame.Short
	PkCnt             frame.Int
	PageSize          frame.Int
	Consistency       frame.Consistency
	SerialConsistency frame.Consistency
	Tracing           bool
	Compression       bool
	Idempotent        bool
	Metadata          *frame.ResultMetadata
}

// Clone makes new Values to avoid data overwrite in binding.
// ID and PkIndexes stay the same, as they are immutable.
func (s Statement) Clone() Statement {
	c := s
	if len(s.Values) != 0 {
		c.Values = make([]frame.Value, len(s.Values))
		for i := range s.Values {
			c.Values[i] = s.Values[i].Clone()
		}
	}
	return c
}

func makeQuery(s Statement, pagingState frame.Bytes) Query {
	return Query{
		Query:       s.Content,
		Consistency: s.Consistency,
		Options: frame.QueryOptions{
			Values:            s.Values,
			SerialConsistency: s.SerialConsistency,
			PagingState:       pagingState,
			PageSize:          s.PageSize,
		},
	}
}

func makeExecute(s Statement, pagingState frame.Bytes) Execute {
	return Execute{
		ID:          s.ID,
		Consistency: s.Consistency,
		Options: frame.QueryOptions{
			Flags:             frame.SkipMetadata,
			Values:            s.Values,
			SerialConsistency: s.SerialConsistency,
			PagingState:       pagingState,
			PageSize:          s.PageSize,
		},
	}
}

func makeStatement(cql string) Statement {
	return Statement{
		Content:     cql,
		Consistency: frame.ONE,
	}
}

type QueryResult struct {
	Rows         []frame.Row
	Warnings     []string
	TracingID    frame.UUID
	HasMorePages bool
	PagingState  frame.Bytes
	ColSpec      []frame.ColumnSpec
	SchemaChange *SchemaChange
}

func MakeQueryResult(res frame.Response, meta *frame.ResultMetadata) (QueryResult, error) {
	switch v := res.(type) {
	case *RowsResult:
		ret := QueryResult{
			Rows:         v.RowsContent,
			PagingState:  v.Metadata.PagingState,
			HasMorePages: v.Metadata.Flags&frame.HasMorePages > 0,
			ColSpec:      v.Metadata.Columns,
		}
		if meta != nil && meta.Columns != nil {
			for i := range ret.Rows {
				for j := range meta.Columns {
					ret.Rows[i][j].Type = &meta.Columns[j].Type
				}
			}
		}
		return ret, nil
	case *VoidResult, *SetKeyspaceResult:
		return QueryResult{}, nil
	case *SchemaChangeResult:
		return QueryResult{SchemaChange: &v.SchemaChange}, nil
	default:
		return QueryResult{}, responseAsError(res)
	}
}
