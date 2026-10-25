package bridge

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestCloseDoesNotWaitForBlockedSend(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	session := &Session{conn: client, encoder: json.NewEncoder(client)}
	sending := make(chan struct{})
	sent := make(chan struct{})
	go func() { close(sending); session.Send(Message{Type: "stdin", Data: "blocked"}); close(sent) }()
	<-sending
	closed := make(chan struct{})
	go func() { session.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close waited for peer or send lock")
	}
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("blocked send did not wake")
	}
}
