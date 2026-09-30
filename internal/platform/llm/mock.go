package llm

import (
	"context"
	"fmt"
	"sync"
)

// MockProvider is a scripted Provider for unit tests and e2e: each Chat pops
// the next queued Response (or calls Handler, when set), and every Request
// is recorded for assertions.
type MockProvider struct {
	mu        sync.Mutex
	queue     []mockStep
	Requests  []Request
	callCount int
	// Handler, when set, overrides the queue and can inspect the request —
	// e.g. answer a tool call by echoing args.
	Handler func(req Request) (Response, error)
}

type mockStep struct {
	resp Response
	err  error
}

// NewMockProvider returns an empty scripted provider.
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// Name identifies the provider implementation.
func (m *MockProvider) Name() string { return "mock" }

// Chat returns the next scripted response. An empty queue (and no Handler)
// is a test bug surfaced as an error rather than a panic.
func (m *MockProvider) Chat(_ context.Context, req Request) (Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Requests = append(m.Requests, req)
	m.callCount++
	if m.Handler != nil {
		return m.Handler(req)
	}
	if len(m.queue) == 0 {
		return Response{}, fmt.Errorf("llm mock: no scripted response for call %d", m.callCount)
	}
	step := m.queue[0]
	m.queue = m.queue[1:]
	return step.resp, step.err
}

// CallCount reports how many Chat calls were made.
func (m *MockProvider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// EnqueueText scripts an assistant content reply.
func (m *MockProvider) EnqueueText(content string) {
	m.queue = append(m.queue, mockStep{resp: Response{Message: Message{Role: RoleAssistant, Content: content}}})
}

// EnqueueToolCalls scripts an assistant reply requesting tool calls.
func (m *MockProvider) EnqueueToolCalls(calls ...ToolCall) {
	m.queue = append(m.queue, mockStep{resp: Response{Message: Message{Role: RoleAssistant, ToolCalls: calls}}})
}

// EnqueueError scripts a failed call.
func (m *MockProvider) EnqueueError(err error) {
	m.queue = append(m.queue, mockStep{err: err})
}

// ToolResult builds the tool-result message to append for a ToolCall —
// convenience for tests driving a hand-rolled agent loop.
func ToolResult(call ToolCall, content string) Message {
	return Message{Role: RoleTool, ToolCallID: call.ID, Name: call.Name, Content: content}
}
