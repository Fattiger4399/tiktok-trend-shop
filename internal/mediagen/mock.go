package mediagen

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
)

// mockPNGBase64 is the built-in 1x1 PNG returned by the mock provider.
const mockPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

// MockProvider is a deterministic Provider for development and tests: Submit
// returns a fake prompt id, the first Fetch per prompt reports pending and
// every later Fetch delivers one built-in 1x1 PNG.
type MockProvider struct {
	mu      sync.Mutex
	submits int
	calls   map[string]int
}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Available(context.Context) error { return nil }

func (m *MockProvider) Submit(_ context.Context, _ JobInput) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.submits++
	return fmt.Sprintf("mock_prompt_%d", m.submits), nil
}

func (m *MockProvider) Fetch(_ context.Context, promptID string) (bool, [][]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.calls == nil {
		m.calls = map[string]int{}
	}
	m.calls[promptID]++
	if m.calls[promptID] == 1 {
		return false, nil, nil
	}
	return true, [][]byte{MockPNG()}, nil
}

// MockPNG returns the built-in 1x1 PNG bytes used by MockProvider.
func MockPNG() []byte {
	data, err := base64.StdEncoding.DecodeString(mockPNGBase64)
	if err != nil {
		panic(err)
	}
	return data
}
