package client

// Fake client for testing nodetool.
// Will respond with given response and error.
type FakeClient struct {
	Response []byte
	Error    error
}

var _ Interface = &FakeClient{}

func (c *FakeClient) Do(request interface{}) ([]byte, error) {
	return c.Response, c.Error
}
