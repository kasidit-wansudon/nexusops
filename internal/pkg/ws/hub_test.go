package ws

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	h := NewHub()
	if h == nil {
		t.Fatal("NewHub returned nil")
	}
	if h.clients == nil {
		t.Fatal("clients map is nil")
	}
	if h.rooms == nil {
		t.Fatal("rooms map is nil")
	}
	if h.register == nil {
		t.Fatal("register channel is nil")
	}
	if h.unregister == nil {
		t.Fatal("unregister channel is nil")
	}
	if h.broadcast == nil {
		t.Fatal("broadcast channel is nil")
	}
	if h.ClientCount() != 0 {
		t.Errorf("ClientCount = %d, want 0", h.ClientCount())
	}
}

func TestHubJoinAndLeaveRoom(t *testing.T) {
	h := NewHub()
	go h.Run()
	time.Sleep(10 * time.Millisecond)

	// Create a client without a real websocket connection (for room tracking only).
	client := &Client{
		hub:    h,
		send:   make(chan []byte, 256),
		rooms:  make(map[string]bool),
		userID: "test-user",
	}

	// Register client via the hub's channel.
	h.register <- client
	time.Sleep(10 * time.Millisecond)

	if h.ClientCount() != 1 {
		t.Errorf("ClientCount after register = %d, want 1", h.ClientCount())
	}

	// Join a room.
	h.JoinRoom(client, "room-1")
	if h.RoomSize("room-1") != 1 {
		t.Errorf("RoomSize(room-1) = %d, want 1", h.RoomSize("room-1"))
	}

	rooms := client.Rooms()
	found := false
	for _, r := range rooms {
		if r == "room-1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("client.Rooms() = %v, want it to contain 'room-1'", rooms)
	}

	// Leave the room.
	h.LeaveRoom(client, "room-1")
	if h.RoomSize("room-1") != 0 {
		t.Errorf("RoomSize(room-1) after leave = %d, want 0", h.RoomSize("room-1"))
	}

	rooms = client.Rooms()
	for _, r := range rooms {
		if r == "room-1" {
			t.Error("client still has room-1 after LeaveRoom")
		}
	}
}

func TestHubMultipleClientsInRoom(t *testing.T) {
	h := NewHub()
	go h.Run()
	time.Sleep(10 * time.Millisecond)

	client1 := &Client{
		hub:    h,
		send:   make(chan []byte, 256),
		rooms:  make(map[string]bool),
		userID: "user-1",
	}
	client2 := &Client{
		hub:    h,
		send:   make(chan []byte, 256),
		rooms:  make(map[string]bool),
		userID: "user-2",
	}

	h.register <- client1
	h.register <- client2
	time.Sleep(10 * time.Millisecond)

	if h.ClientCount() != 2 {
		t.Errorf("ClientCount = %d, want 2", h.ClientCount())
	}

	// Both join the same room.
	h.JoinRoom(client1, "shared-room")
	h.JoinRoom(client2, "shared-room")
	if h.RoomSize("shared-room") != 2 {
		t.Errorf("RoomSize(shared-room) = %d, want 2", h.RoomSize("shared-room"))
	}

	// Remove one client from the room.
	h.LeaveRoom(client1, "shared-room")
	if h.RoomSize("shared-room") != 1 {
		t.Errorf("RoomSize(shared-room) after one leave = %d, want 1", h.RoomSize("shared-room"))
	}

	// Remove the other.
	h.LeaveRoom(client2, "shared-room")
	if h.RoomSize("shared-room") != 0 {
		t.Errorf("RoomSize(shared-room) after both leave = %d, want 0", h.RoomSize("shared-room"))
	}
}

func TestBroadcastToRoom(t *testing.T) {
	h := NewHub()
	go h.Run()
	time.Sleep(10 * time.Millisecond)

	client := &Client{
		hub:    h,
		send:   make(chan []byte, 256),
		rooms:  make(map[string]bool),
		userID: "broadcast-user",
	}

	h.register <- client
	time.Sleep(10 * time.Millisecond)

	h.JoinRoom(client, "updates")

	msg := []byte(`{"event":"deploy"}`)
	h.BroadcastToRoom("updates", msg)

	select {
	case received := <-client.send:
		if string(received) != string(msg) {
			t.Errorf("received = %q, want %q", string(received), string(msg))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast message")
	}
}

func TestBroadcastToEmptyRoom(t *testing.T) {
	h := NewHub()
	go h.Run()
	time.Sleep(10 * time.Millisecond)

	// Broadcasting to a room with no members should not panic.
	h.BroadcastToRoom("empty-room", []byte(`{"event":"test"}`))
}

func TestMessageSerialization(t *testing.T) {
	tests := []struct {
		name    string
		msgType MessageType
		room    string
	}{
		{"build log", MsgBuildLog, "build-123"},
		{"deploy status", MsgDeployStatus, "deploy-456"},
		{"metric update", MsgMetricUpdate, "metrics"},
		{"pipeline event", MsgPipelineEvent, "pipeline-789"},
		{"system alert", MsgSystemAlert, "alerts"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]string{"key": "value"})
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			msg := &Message{
				Type:      tc.msgType,
				Room:      tc.room,
				Payload:   json.RawMessage(payload),
				Timestamp: time.Now().UTC(),
			}

			data, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("Marshal message failed: %v", err)
			}

			var decoded Message
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal message failed: %v", err)
			}

			if decoded.Type != tc.msgType {
				t.Errorf("Type = %q, want %q", decoded.Type, tc.msgType)
			}
			if decoded.Room != tc.room {
				t.Errorf("Room = %q, want %q", decoded.Room, tc.room)
			}
			if len(decoded.Payload) == 0 {
				t.Error("Payload is empty after round-trip")
			}
		})
	}
}

func TestMessageTypes(t *testing.T) {
	types := []MessageType{MsgBuildLog, MsgDeployStatus, MsgMetricUpdate, MsgPipelineEvent, MsgSystemAlert}
	expected := []string{"build_log", "deploy_status", "metric_update", "pipeline_event", "system_alert"}

	if len(types) != len(expected) {
		t.Fatalf("types count = %d, expected count = %d", len(types), len(expected))
	}

	seen := make(map[MessageType]bool)
	for i, mt := range types {
		if string(mt) != expected[i] {
			t.Errorf("types[%d] = %q, want %q", i, string(mt), expected[i])
		}
		if string(mt) == "" {
			t.Errorf("message type at index %d is empty", i)
		}
		if seen[mt] {
			t.Errorf("duplicate message type: %q", mt)
		}
		seen[mt] = true
	}
}
