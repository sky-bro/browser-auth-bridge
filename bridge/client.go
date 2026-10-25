package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

type Session struct {
	conn    net.Conn
	decoder *json.Decoder
	encoder *json.Encoder
	mu      sync.Mutex
	session string
}

func Connect(ctx context.Context, namespace string, attach Message, dialTimeout time.Duration) (*Session, error) {
	if attach.Type != "attach" || !ValidSession(attach.Session) {
		return nil, fmt.Errorf("invalid bridge attach")
	}
	path, err := BrokerSocketPath(namespace)
	if err != nil {
		return nil, err
	}
	if dialTimeout <= 0 {
		dialTimeout = 300 * time.Millisecond
	}
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	client := &Session{
		conn: conn, decoder: json.NewDecoder(conn), encoder: json.NewEncoder(conn), session: attach.Session,
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if err := client.Send(attach); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return client, nil
}

func (session *Session) Read() (Message, error) {
	var message Message
	err := session.decoder.Decode(&message)
	return message, err
}

func (session *Session) Send(message Message) error {
	session.mu.Lock()
	defer session.mu.Unlock()
	message.Session = session.session
	return session.encoder.Encode(message)
}

func (session *Session) Close() error {
	if session == nil || session.conn == nil {
		return nil
	}
	// EOF makes the broker close the remote session. Do not send synchronously:
	// a stalled peer or concurrent blocked Send must never prevent cancellation.
	return session.conn.Close()
}
