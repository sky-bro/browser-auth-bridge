package commandstream

import (
	"context"
	"io"
)

// Writer delivers at most one bounded chunk at a time. There is no output queue.
// Cancellation stops waiting for a blocked sink without closing caller-owned
// stdout. If a sink cannot be interrupted, its one in-flight Write can remain
// blocked until the sink is released (or the CLI process exits).
type Writer struct {
	ctx      context.Context
	cancel   context.CancelFunc
	requests chan writeRequest
}
type writeRequest struct {
	data   []byte
	result chan writeResult
}
type writeResult struct {
	n   int
	err error
}

func NewWriter(ctx context.Context, sink io.Writer) *Writer {
	ctx, cancel := context.WithCancel(ctx)
	w := &Writer{ctx: ctx, cancel: cancel, requests: make(chan writeRequest)}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case req := <-w.requests:
				n, err := sink.Write(req.data)
				if err == nil && n != len(req.data) {
					err = io.ErrShortWrite
				}
				req.result <- writeResult{n, err}
			}
		}
	}()
	return w
}
func (w *Writer) Close() { w.cancel() }
func (w *Writer) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		if err := w.ctx.Err(); err != nil {
			return total, err
		}
		n := min(len(p), 32<<10)
		req := writeRequest{data: append([]byte(nil), p[:n]...), result: make(chan writeResult, 1)}
		select {
		case <-w.ctx.Done():
			return total, w.ctx.Err()
		case w.requests <- req:
		}
		select {
		case <-w.ctx.Done():
			return total, w.ctx.Err()
		case result := <-req.result:
			total += result.n
			if result.err != nil {
				return total, result.err
			}
		}
		p = p[n:]
	}
	return total, nil
}
