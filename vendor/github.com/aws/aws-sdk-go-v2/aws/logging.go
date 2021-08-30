// Code generated by aws/logging_generate.go DO NOT EDIT.

package aws

// ClientLogMode represents the logging mode of SDK clients. The client logging mode is a bit-field where
// each bit is a flag that describes the logging behavior for one or more client components.
// The entire 64-bit group is reserved for later expansion by the SDK.
//
// Example: Setting ClientLogMode to enable logging of retries and requests
//  clientLogMode := aws.LogRetries | aws.LogRequest
//
// Example: Adding an additional log mode to an existing ClientLogMode value
//  clientLogMode |= aws.LogResponse
type ClientLogMode uint64

// Supported ClientLogMode bits that can be configured to toggle logging of specific SDK events.
const (
	LogSigning ClientLogMode = 1 << (64 - 1 - iota)
	LogRetries
	LogRequest
	LogRequestWithBody
	LogResponse
	LogResponseWithBody
)

// IsSigning returns whether the Signing logging mode bit is set
func (m ClientLogMode) IsSigning() bool {
	return m&LogSigning != 0
}

// IsRetries returns whether the Retries logging mode bit is set
func (m ClientLogMode) IsRetries() bool {
	return m&LogRetries != 0
}

// IsRequest returns whether the Request logging mode bit is set
func (m ClientLogMode) IsRequest() bool {
	return m&LogRequest != 0
}

// IsRequestWithBody returns whether the RequestWithBody logging mode bit is set
func (m ClientLogMode) IsRequestWithBody() bool {
	return m&LogRequestWithBody != 0
}

// IsResponse returns whether the Response logging mode bit is set
func (m ClientLogMode) IsResponse() bool {
	return m&LogResponse != 0
}

// IsResponseWithBody returns whether the ResponseWithBody logging mode bit is set
func (m ClientLogMode) IsResponseWithBody() bool {
	return m&LogResponseWithBody != 0
}

// ClearSigning clears the Signing logging mode bit
func (m *ClientLogMode) ClearSigning() {
	*m &^= LogSigning
}

// ClearRetries clears the Retries logging mode bit
func (m *ClientLogMode) ClearRetries() {
	*m &^= LogRetries
}

// ClearRequest clears the Request logging mode bit
func (m *ClientLogMode) ClearRequest() {
	*m &^= LogRequest
}

// ClearRequestWithBody clears the RequestWithBody logging mode bit
func (m *ClientLogMode) ClearRequestWithBody() {
	*m &^= LogRequestWithBody
}

// ClearResponse clears the Response logging mode bit
func (m *ClientLogMode) ClearResponse() {
	*m &^= LogResponse
}

// ClearResponseWithBody clears the ResponseWithBody logging mode bit
func (m *ClientLogMode) ClearResponseWithBody() {
	*m &^= LogResponseWithBody
}
