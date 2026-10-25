package main

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sky-bro/browser-auth-bridge/bridge"
)

type fixtureSession struct {
	script   chan string
	messages chan bridge.Message
	closed   chan struct{}
	once     sync.Once
}

func newFixtureSession() *fixtureSession {
	return &fixtureSession{make(chan string, 1), make(chan bridge.Message), make(chan struct{}), sync.Once{}}
}
func (s *fixtureSession) Send(m bridge.Message) error { s.script <- m.Data; return nil }
func (s *fixtureSession) Read() (bridge.Message, error) {
	select {
	case m := <-s.messages:
		return m, nil
	case <-s.closed:
		return bridge.Message{}, io.EOF
	}
}
func (s *fixtureSession) Close() error { s.once.Do(func() { close(s.closed) }); return nil }
func (s *fixtureSession) emit(data string) bool {
	select {
	case s.messages <- bridge.Message{Type: "stdout", Data: data}:
		return true
	case <-s.closed:
		return false
	}
}

type outputCounter struct {
	count int64
	first chan struct{}
}

func (w *outputCounter) Write(p []byte) (int, error) {
	if len(p) > 0 && w.count == 0 {
		close(w.first)
	}
	w.count += int64(len(p))
	return len(p), nil
}
func TestExecStreamsBeyond16MiBBeforeCompletionAndKeepsExitCode(t *testing.T) {
	s := newFixtureSession()
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sink := &outputCounter{first: make(chan struct{})}
	result := make(chan error, 1)
	go func() { result <- executeSession(ctx, s, []string{"printf", "large"}, sink) }()
	script := <-s.script
	match := regexp.MustCompile(`'([a-f0-9]{64})'`).FindStringSubmatch(script)
	if len(match) != 2 {
		t.Fatal("missing random completion token")
	}
	token := match[1]
	release := make(chan struct{})
	defer func() { s.Close() }()
	go func() {
		if !s.emit("\x1e" + token + "\x1fready") {
			return
		}
		select {
		case <-release:
		case <-s.closed:
			return
		}
		chunk := strings.Repeat("x", 32<<10)
		for i := 0; i < 640; i++ {
			if !s.emit(chunk) {
				return
			}
		}
		s.emit("\x1e" + token + ":7\x1f")
	}()
	select {
	case <-sink.first:
	case <-ctx.Done():
		t.Fatal("output was buffered until exit")
	}
	select {
	case err := <-result:
		t.Fatalf("completed before release: %v", err)
	default:
	}
	close(release)
	select {
	case err := <-result:
		var status exitError
		if !errors.As(err, &status) || status != 7 || sink.count != 5+(20<<20) {
			t.Fatalf("error=%v bytes=%d", err, sink.count)
		}
	case <-ctx.Done():
		t.Fatal("stream stalled")
	}
}
func TestExecCancellationClosesSession(t *testing.T) {
	s := newFixtureSession()
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- executeSession(ctx, s, []string{"tail", "-f", "log"}, io.Discard) }()
	<-s.script
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled command succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("read did not cancel")
	}
	select {
	case <-s.closed:
	case <-time.After(time.Second):
		t.Fatal("session not closed")
	}
}
func TestExecFlagsAreHandledBeforeInstallation(t *testing.T) {
	for _, args := range [][]string{{"exec", "--timeout", "-1s", "--", "true"}, {"exec", "--timeout", "bad", "--", "true"}} {
		if err := runCLI(args, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, code := range []int{124, 130} {
		var status interface{ ExitCode() int }
		if !errors.As(&commandError{code, context.Canceled}, &status) || status.ExitCode() != code {
			t.Fatal("lost local failure exit code")
		}
	}
}
