package services

import (
	"testing"
	"time"
)

func TestHubRegisterUnregister(t *testing.T) {
	hub := &WSHub{
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		clients:    make(map[string]*WSClient),
	}
	go hub.Run()

	conn := &WSConnection{Conn: nil} // nil conn is fine; hub doesn't touch it
	userA := &WSClient{
		Hub:      hub,
		Conn:     conn,
		Send:     make(chan []byte, 8),
		UserInfo: &UserInfo{UserID: "user-a", Username: "A"},
	}

	hub.Register(userA)
	time.Sleep(10 * time.Millisecond) // allow goroutine to process

	if !hub.IsUserOnline("user-a") {
		t.Fatal("user-a should be online after register")
	}
	if hub.GetOnlineCount() != 1 {
		t.Fatalf("expected 1 online, got %d", hub.GetOnlineCount())
	}

	hub.Unregister(userA)
	time.Sleep(10 * time.Millisecond)

	if hub.IsUserOnline("user-a") {
		t.Fatal("user-a should NOT be online after unregister")
	}
	if hub.GetOnlineCount() != 0 {
		t.Fatalf("expected 0 online, got %d", hub.GetOnlineCount())
	}
}

func TestHubBroadcastToUser(t *testing.T) {
	hub := &WSHub{
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		clients:    make(map[string]*WSClient),
	}
	go hub.Run()

	conn := &WSConnection{Conn: nil}
	userB := &WSClient{
		Hub:      hub,
		Conn:     conn,
		Send:     make(chan []byte, 8),
		UserInfo: &UserInfo{UserID: "user-b", Username: "B"},
	}
	hub.Register(userB)
	time.Sleep(10 * time.Millisecond)

	ok := hub.BroadcastToUser("user-b", []byte("hello"))
	if !ok {
		t.Fatal("broadcast should succeed for online user")
	}

	select {
	case msg := <-userB.Send:
		if string(msg) != "hello" {
			t.Fatalf("expected 'hello', got '%s'", string(msg))
		}
	default:
		t.Fatal("expected message on send channel")
	}

	// broadcast to offline user
	ok = hub.BroadcastToUser("user-c", []byte("world"))
	if ok {
		t.Fatal("broadcast should fail for offline user")
	}
}

func TestHubKickUser(t *testing.T) {
	hub := &WSHub{
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		clients:    make(map[string]*WSClient),
	}
	go hub.Run()

	conn := &WSConnection{Conn: nil}
	userC := &WSClient{
		Hub:      hub,
		Conn:     conn,
		Send:     make(chan []byte, 8),
		UserInfo: &UserInfo{UserID: "user-c", Username: "C"},
	}
	hub.Register(userC)
	time.Sleep(10 * time.Millisecond)

	ok := hub.KickUser("user-c")
	if !ok {
		t.Fatal("kick should succeed for online user")
	}

	// the kick closes the Send channel
	select {
	case _, open := <-userC.Send:
		if open {
			t.Fatal("Send channel should be closed after kick")
		}
	default:
	}
}
