package mocks

import (
	"fmt"
)

// Suppose your natsClient is something like:
// type NatsClient interface {
//     Publish(subject string, data []byte) error
// }

// We'll define a mock:

type MockNatsClient struct {
	PublishFn func(subject string, data []byte) error
}

func (m *MockNatsClient) Publish(subject string, data []byte) error {
	if m.PublishFn != nil {
		return m.PublishFn(subject, data)
	}
	// default - do nothing
	fmt.Printf("MockNATS Publish to subject=%s data=%s\n", subject, string(data))
	return nil
}

func NewMockNatsClient() *MockNatsClient {
	return &MockNatsClient{}
}
