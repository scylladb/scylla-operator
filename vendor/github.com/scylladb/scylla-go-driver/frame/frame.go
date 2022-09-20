package frame

// Request specifies interface for all frame/request types.
type Request interface {
	WriteTo(b *Buffer)
	OpCode() OpCode
}

// Response specifies interface for all frame/response types.
type Response interface{}
