package commandstream

import (
	"context"
	"errors"
	"testing"
	"time"
)

type blockedWriter struct {
	entered           chan int
	release, finished chan struct{}
}

func (b blockedWriter) Write(p []byte) (int, error) {
	b.entered <- len(p)
	<-b.release
	close(b.finished)
	return len(p), nil
}
func TestSlowSinkHasOneBoundedWriteAndCancellationReturns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sink := blockedWriter{make(chan int, 1), make(chan struct{}), make(chan struct{})}
	w := NewWriter(ctx, sink)
	defer w.Close()
	result := make(chan error, 1)
	go func() { _, err := w.Write(make([]byte, 20<<20)); result <- err }()
	select {
	case n := <-sink.entered:
		if n > 32<<10 {
			t.Fatal(n)
		}
	case <-time.After(time.Second):
		t.Fatal("writer never started")
	}
	select {
	case err := <-result:
		t.Fatalf("slow sink did not apply backpressure: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation blocked on sink")
	}
	close(sink.release)
	<-sink.finished
}
