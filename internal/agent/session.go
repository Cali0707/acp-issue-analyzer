package agent

import (
	"sync"

	acp "github.com/coder/acp-go-sdk"
)

// SessionTracker accumulates session updates and provides them to the TUI.
type SessionTracker struct {
	mu      sync.Mutex
	updates []acp.SessionUpdate
	updateC chan acp.SessionUpdate
}

func NewSessionTracker() *SessionTracker {
	return &SessionTracker{
		updateC: make(chan acp.SessionUpdate, 256),
	}
}

// AddUpdate stores an update and pushes it to the channel for TUI consumption.
func (st *SessionTracker) AddUpdate(update acp.SessionUpdate) {
	st.mu.Lock()
	st.updates = append(st.updates, update)
	st.mu.Unlock()

	// Non-blocking send — if TUI isn't consuming, we still store it.
	select {
	case st.updateC <- update:
	default:
	}
}

// Updates returns a copy of all accumulated updates.
func (st *SessionTracker) Updates() []acp.SessionUpdate {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]acp.SessionUpdate, len(st.updates))
	copy(out, st.updates)
	return out
}

// UpdateChan returns the channel that delivers updates for TUI streaming.
func (st *SessionTracker) UpdateChan() <-chan acp.SessionUpdate {
	return st.updateC
}

// Close closes the update channel.
func (st *SessionTracker) Close() {
	close(st.updateC)
}
