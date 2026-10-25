package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/sky-bro/browser-auth-bridge/bridge"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdapterPinsRecipe(t *testing.T) {
	p := bridge.PTYOpen{Version: 1, Provider: "demo", Target: "echo", Recipe: recipe(), Cols: 80, Rows: 24}
	raw, _ := json.Marshal(p)
	m := bridge.Message{Service: "demo", Operation: "pty", Payload: raw}
	if err := validate(m); err != nil {
		t.Fatal(err)
	}
	p.Recipe.Authorization.URL = origin + "/another-endpoint"
	m.Payload, _ = json.Marshal(p)
	if validate(m) == nil {
		t.Fatal("host accepted unpinned recipe")
	}
}
func TestLocalFixtureRoundTrip(t *testing.T) {
	server := httptest.NewServer(handler())
	defer server.Close()
	conn, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	io.WriteString(conn, "GET /stream HTTP/1.1\r\nHost: fixture\r\nOrigin: "+origin+"\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: MDEyMzQ1Njc4OWFiY2RlZg==\r\n\r\n")
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	if err != nil || !strings.Contains(line, "101") {
		t.Fatalf("handshake: %q %v", line, err)
	}
	for {
		line, err = r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	input := []byte(`{"action":"input","payload":"hello"}`)
	packet := append([]byte{0x81, 0x80 | byte(len(input)), 1, 2, 3, 4}, input...)
	for i := range input {
		packet[6+i] ^= packet[2+i%4]
	}
	conn.Write(packet)
	h := make([]byte, 2)
	if _, err = io.ReadFull(r, h); err != nil {
		t.Fatal(err)
	}
	if h[0] != 0x81 || h[1]&0x80 != 0 {
		t.Fatal("invalid server frame")
	}
	data := make([]byte, int(h[1]))
	if _, err = io.ReadFull(r, data); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"payload":"hello"`)) {
		t.Fatalf("echo: %s", data)
	}
}
func TestRejectUnmaskedAndOversizedFrames(t *testing.T) {
	for _, data := range [][]byte{{0x81, 1, 'x'}, {0x81, 0xff, 0, 0, 0, 0, 0, 0x20, 0, 0}} {
		if _, err := readFrame(bufio.NewReader(bytes.NewReader(data))); err == nil {
			t.Fatal("unsafe frame accepted")
		}
	}
}
