package commandstream

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestEveryMarkerAndUTF8Split(t *testing.T) {
	for _, f := range []struct {
		start, end string
		term       byte
		normalize  bool
	}{{"\nBEGIN_x\n", "\nEND_x:", '\n', true}, {"\x1ex\x1f", "\x1ex:", '\x1f', false}} {
		body := "hello\n世界\x1b[31mred\x1b[0m\n"
		input := "unrelated prompt" + f.start + body + f.end + "7" + string(f.term) + "ignored prompt"
		if f.normalize {
			input = strings.ReplaceAll(input, "\n", "\r\n")
		}
		for split := 0; split <= len(input); split++ {
			var out bytes.Buffer
			d := New(&out, f.start, f.end, f.term, f.normalize)
			if _, _, err := d.Feed(input[:split]); err != nil {
				t.Fatal(err)
			}
			done, code, err := d.Feed(input[split:])
			if err != nil || !done || code != 7 || out.String() != body {
				t.Fatalf("split=%d done=%v code=%d err=%v output=%q", split, done, code, err, out.String())
			}
		}
		var out bytes.Buffer
		d := New(&out, f.start, f.end, f.term, f.normalize)
		for _, b := range []byte(input) {
			if _, _, err := d.Feed(string([]byte{b})); err != nil {
				t.Fatal(err)
			}
		}
		if !d.done || d.code != 7 || out.String() != body {
			t.Fatalf("bytewise output=%q", out.String())
		}
	}
}
func TestOutputArrivesBeforeCompletion(t *testing.T) {
	var out bytes.Buffer
	d := New(&out, "START", "\nEND:", '\n', true)
	done, _, err := d.Feed("STARTready")
	if err != nil || done || out.String() != "ready" {
		t.Fatalf("done=%v err=%v output=%q", done, err, out.String())
	}
	d.Feed("\nEND:12")
	if out.String() != "ready" {
		t.Fatal("partial marker leaked")
	}
	done, code, err := d.Feed("\r\n")
	if err != nil || !done || code != 12 || out.String() != "ready" {
		t.Fatalf("done=%v code=%d err=%v output=%q", done, code, err, out.String())
	}
}
func TestOrdinaryNewlineIsDeliveredImmediately(t *testing.T) {
	var out bytes.Buffer
	d := New(&out, "\x1eSTART\x1f", "\x1eEND:", '\x1f', true)
	done, _, err := d.Feed("\x1eSTART\x1fready\r\n")
	if err != nil || done || out.String() != "ready\n" {
		t.Fatalf("done=%v err=%v output=%q", done, err, out.String())
	}
}
func TestNearMatchesAreOutputAndStateIsBounded(t *testing.T) {
	var out bytes.Buffer
	d := New(&out, "START", "\nEND:", '\n', false)
	body := "a\nEND:9999\nb\nEND:256\nc\nEND:no\nd\n"
	for _, b := range []byte("START" + body + "\nEND:0\n") {
		if _, _, err := d.Feed(string([]byte{b})); err != nil {
			t.Fatal(err)
		}
		if len(d.pending) > len(d.end)+4 {
			t.Fatal("unbounded partial status")
		}
	}
	if !d.done || out.String() != body {
		t.Fatalf("output=%q", out.String())
	}
	missing := New(io.Discard, "START", "END:", '\n', false)
	chunk := strings.Repeat("x", 32<<10)
	for i := 0; i < 1024; i++ {
		missing.Feed(chunk)
		if len(missing.pending) >= len(missing.start) {
			t.Fatal("unbounded preamble")
		}
	}
}

type countWriter struct {
	n   int64
	max int
}

func (w *countWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	w.max = max(w.max, len(p))
	return len(p), nil
}
func TestOutputBeyondOldLimitUsesBoundedState(t *testing.T) {
	sink := &countWriter{}
	d := New(sink, "START", "END:", '\n', false)
	d.Feed("START")
	chunk := strings.Repeat("x", 32<<10)
	for i := 0; i < 1024; i++ {
		if _, _, err := d.Feed(chunk); err != nil {
			t.Fatal(err)
		}
		if len(d.pending) > len(d.end)+4 {
			t.Fatal("unbounded state")
		}
	}
	done, code, err := d.Feed("END:0\n")
	if err != nil || !done || code != 0 || sink.n != 32<<20 || sink.max > 32<<10 {
		t.Fatalf("done=%v code=%d err=%v sink=%+v", done, code, err, sink)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestSinkErrorsPropagate(t *testing.T) {
	d := New(failWriter{}, "START", "END:", '\n', false)
	if _, _, err := d.Feed("STARToutput"); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}
