package agent

import (
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

func TestSessionTracker_AddAndRetrieve(t *testing.T) {
	st := NewSessionTracker()
	defer st.Close()

	u1 := acp.UpdateAgentMessageText("hello")
	u2 := acp.UpdateAgentMessageText("world")

	st.AddUpdate(u1)
	st.AddUpdate(u2)

	updates := st.Updates()
	if len(updates) != 2 {
		t.Fatalf("len(updates) = %d, want 2", len(updates))
	}
}

func TestSessionTracker_UpdatesReturnsACopy(t *testing.T) {
	st := NewSessionTracker()
	defer st.Close()

	st.AddUpdate(acp.UpdateAgentMessageText("first"))

	updates1 := st.Updates()
	st.AddUpdate(acp.UpdateAgentMessageText("second"))
	updates2 := st.Updates()

	if len(updates1) != 1 {
		t.Errorf("first snapshot should have 1 update, got %d", len(updates1))
	}
	if len(updates2) != 2 {
		t.Errorf("second snapshot should have 2 updates, got %d", len(updates2))
	}
}

func TestSessionTracker_UpdateChan(t *testing.T) {
	st := NewSessionTracker()
	defer st.Close()

	go func() {
		st.AddUpdate(acp.UpdateAgentMessageText("streamed"))
	}()

	select {
	case u := <-st.UpdateChan():
		if u.AgentMessageChunk == nil {
			t.Error("expected AgentMessageChunk update")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update on channel")
	}
}

func TestSessionTracker_NonBlockingWhenChannelFull(t *testing.T) {
	st := NewSessionTracker()
	defer st.Close()

	// Fill the channel buffer (256)
	for i := 0; i < 300; i++ {
		st.AddUpdate(acp.UpdateAgentMessageText("spam"))
	}

	// Should not have panicked or blocked
	updates := st.Updates()
	if len(updates) != 300 {
		t.Errorf("stored updates = %d, want 300", len(updates))
	}
}

func TestSessionTracker_EmptyUpdates(t *testing.T) {
	st := NewSessionTracker()
	defer st.Close()

	updates := st.Updates()
	if len(updates) != 0 {
		t.Errorf("updates should be empty, got %d", len(updates))
	}
}

func TestSessionTracker_VariousUpdateTypes(t *testing.T) {
	st := NewSessionTracker()
	defer st.Close()

	st.AddUpdate(acp.UpdateAgentMessageText("message"))
	st.AddUpdate(acp.UpdateAgentThoughtText("thought"))
	st.AddUpdate(acp.StartToolCall("tc-1", "Read file"))

	updates := st.Updates()
	if len(updates) != 3 {
		t.Fatalf("len(updates) = %d, want 3", len(updates))
	}

	if updates[0].AgentMessageChunk == nil {
		t.Error("update 0 should be AgentMessageChunk")
	}
	if updates[1].AgentThoughtChunk == nil {
		t.Error("update 1 should be AgentThoughtChunk")
	}
	if updates[2].ToolCall == nil {
		t.Error("update 2 should be ToolCall")
	}
}
