// Package commandstream supplies optional consumer-side helpers for framed
// terminal command output. It is independent of bridge transport and site policy.
package commandstream

import (
	"bytes"
	"io"
	"strconv"
)

// Decoder removes a start delimiter and an end-prefix + decimal exit code +
// terminator from terminal text. It retains only a possible delimiter suffix,
// never the complete output. Delimiters must be nonempty trusted configuration.
type Decoder struct {
	out                          io.Writer
	start, end                   []byte
	term                         byte
	pending                      []byte
	started, done, normalize, cr bool
	code                         int
}

func New(out io.Writer, start, end string, terminator byte, normalizeCRLF bool) *Decoder {
	if out == nil || start == "" || end == "" {
		panic("commandstream: writer and delimiters are required")
	}
	return &Decoder{out: out, start: []byte(start), end: []byte(end), term: terminator, normalize: normalizeCRLF}
}

// Feed accepts one bounded transport chunk. A completed decoder ignores trailing
// shell prompts. Writer failures are returned; an incomplete frame is not success.
func (d *Decoder) Feed(data string) (bool, int, error) {
	if d.done {
		return true, d.code, nil
	}
	if d.normalize {
		if d.cr {
			data = "\r" + data
			d.cr = false
		}
		if len(data) > 0 && data[len(data)-1] == '\r' {
			data = data[:len(data)-1]
			d.cr = true
		}
		data = string(bytes.ReplaceAll([]byte(data), []byte("\r\n"), []byte("\n")))
	}
	data = string(d.pending) + data
	d.pending = d.pending[:0]
	b := []byte(data)
	if !d.started {
		i := bytes.Index(b, d.start)
		if i < 0 {
			d.retainSuffix(b, d.start)
			return false, 0, nil
		}
		d.started = true
		b = b[i+len(d.start):]
	}
	for len(b) > 0 {
		i := bytes.Index(b, d.end)
		if i < 0 {
			n := suffixLength(b, d.end)
			if err := writeAll(d.out, b[:len(b)-n]); err != nil {
				return false, 0, err
			}
			d.pending = append(d.pending, b[len(b)-n:]...)
			return false, 0, nil
		}
		if err := writeAll(d.out, b[:i]); err != nil {
			return false, 0, err
		}
		b = b[i:]
		rest := b[len(d.end):]
		digits := 0
		for digits < len(rest) && digits < 4 && rest[digits] >= '0' && rest[digits] <= '9' {
			digits++
		}
		if digits <= 3 && digits == len(rest) {
			d.pending = append(d.pending, b...)
			return false, 0, nil
		}
		if digits > 0 && digits <= 3 && rest[digits] == d.term {
			code, err := strconv.Atoi(string(rest[:digits]))
			if err == nil && code <= 255 {
				d.done = true
				d.code = code
				return true, code, nil
			}
		}
		// A near-match is ordinary output. Move at least one byte to bound both
		// retained state and work even for malformed or adversarial marker-like text.
		if err := writeAll(d.out, b[:1]); err != nil {
			return false, 0, err
		}
		b = b[1:]
	}
	return false, 0, nil
}

func (d *Decoder) retainSuffix(b, prefix []byte) {
	n := suffixLength(b, prefix)
	d.pending = append(d.pending, b[len(b)-n:]...)
}
func suffixLength(b, prefix []byte) int {
	n := min(len(b), len(prefix)-1)
	for ; n > 0; n-- {
		if bytes.Equal(b[len(b)-n:], prefix[:n]) {
			return n
		}
	}
	return 0
}
func writeAll(out io.Writer, b []byte) error {
	for len(b) > 0 {
		chunk := b[:min(len(b), 32<<10)]
		n, err := out.Write(chunk)
		if err != nil {
			return err
		}
		if n != len(chunk) {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
