// bridge-demo is a credential-free echo fixture, not a remote shell.
package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/sky-bro/browser-auth-bridge/bridge"
	"github.com/sky-bro/browser-auth-bridge/nativehost"
)

const origin = "http://127.0.0.1:8765"
const host = "org.browserauthbridge.demo"

func recipe() bridge.StreamRecipe {
	return bridge.StreamRecipe{Version: 1, Origin: origin, PageURL: origin + "/console", SocketURL: "ws://127.0.0.1:8765/stream",
		Authorization: bridge.AuthorizationRecipe{URL: origin + "/permission", StatusField: "result", SuccessValue: "OK", LoginValue: "LOGIN_REQUIRED", AccessField: "allowed", AccessGranted: true},
		Frame:         bridge.FrameRecipe{OperationField: "action", DataField: "payload", ColsField: "columns", RowsField: "lines", Stdin: "input", Stdout: "output", Header: "metadata", Resize: "window"}}
}

type demoAdapter struct{}

func (demoAdapter) Recipe(target string) (bridge.StreamRecipe, error) {
	if target != "echo" {
		return bridge.StreamRecipe{}, fmt.Errorf("unknown target")
	}
	return recipe(), nil
}

var _ bridge.PTYAdapter = demoAdapter{}

func validate(m bridge.Message) error {
	if m.Service != "demo" {
		return fmt.Errorf("provider denied")
	}
	if m.Operation == "browser" {
		_, err := bridge.DecodeBrowserCommand(m.Payload, origin)
		return err
	}
	if m.Operation != "pty" {
		return fmt.Errorf("operation denied")
	}
	p, err := bridge.DecodePTYOpen(m.Payload, "demo", origin)
	if err != nil {
		return err
	}
	want, err := (demoAdapter{}).Recipe(p.Target)
	if err != nil || p.Recipe != want {
		return fmt.Errorf("recipe denied")
	}
	return nil
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) > 0 && (args[0] == "native-host" || strings.HasPrefix(args[0], "chrome-extension://")) {
		return nativehost.Run(os.Stdin, os.Stdout, nativehost.Config{Namespace: "bridge-demo", ValidateAttach: validate})
	}
	if len(args) == 1 && args[0] == "serve" {
		return http.ListenAndServe("127.0.0.1:8765", handler())
	}
	if len(args) == 2 && args[0] == "install-host" {
		return install(args[1])
	}
	if len(args) == 1 && args[0] == "echo" {
		return echo()
	}
	if len(args) == 1 && args[0] == "browser" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		id, _ := bridge.NewSessionID()
		command := bridge.BrowserCommand{Version: 1, Origin: origin, BrowserSession: id, Action: "open", URL: origin + "/console"}
		_, err := bridge.ExecuteBrowser(ctx, "bridge-demo", "demo", command, validate, bridge.BrowserOptions{AllowTab: true})
		if err != nil {
			return err
		}
		command.Action = "eval"
		command.URL = ""
		command.Script = "return {title: document.title};"
		result, err := bridge.ExecuteBrowser(ctx, "bridge-demo", "demo", command, validate, bridge.BrowserOptions{})
		command.Action = "close"
		command.Script = ""
		_, closeErr := bridge.ExecuteBrowser(ctx, "bridge-demo", "demo", command, validate, bridge.BrowserOptions{})
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Println(string(result.Value))
		return nil
	}
	return fmt.Errorf("usage: bridge-demo serve | install-host EXTENSION_ID | echo | browser | native-host")
}
func install(id string) error {
	if !regexp.MustCompile(`^[a-p]{32}$`).MatchString(id) {
		return fmt.Errorf("invalid extension ID")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var directory string
	switch runtime.GOOS {
	case "darwin":
		directory = filepath.Join(home, "Library/Application Support/Google/Chrome/NativeMessagingHosts")
	case "linux":
		directory = filepath.Join(home, ".config/google-chrome/NativeMessagingHosts")
	default:
		return fmt.Errorf("unsupported native host platform")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(map[string]any{"name": host, "description": "Browser transport demo", "path": exe, "type": "stdio", "allowed_origins": []string{"chrome-extension://" + id + "/"}}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, host+".json"), append(data, '\n'), 0600)
}
func echo() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	id, err := bridge.NewSessionID()
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(bridge.PTYOpen{Version: 1, Provider: "demo", Target: "echo", Recipe: recipe(), Cols: 80, Rows: 24})
	m := bridge.Message{Type: "attach", Session: id, Service: "demo", Operation: "pty", Payload: payload, AllowTab: true}
	if err = validate(m); err != nil {
		return err
	}
	s, err := bridge.Connect(ctx, "bridge-demo", m, time.Second)
	if err != nil {
		return err
	}
	defer s.Close()
	stop := context.AfterFunc(ctx, func() { s.Close() })
	defer stop()
	for {
		m, err := s.Read()
		if err != nil {
			return err
		}
		if m.Type == "error" {
			return fmt.Errorf("%s", m.Error)
		}
		if m.Type == "status" && m.State == "connected" {
			break
		}
	}
	if err = s.Send(bridge.Message{Type: "stdin", Data: "hello from the public core\n"}); err != nil {
		return err
	}
	for {
		m, err := s.Read()
		if err != nil {
			return err
		}
		if m.Type == "stdout" {
			fmt.Print(m.Data)
			return nil
		}
		if m.Type == "error" {
			return fmt.Errorf("%s", m.Error)
		}
	}
}
func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/console", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!doctype html><title>Public bridge echo demo</title><h1>Local echo fixture</h1><p>No login, credentials or shell execution.</p>")
	})
	mux.HandleFunc("/permission", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":"OK","allowed":true}`)
	})
	mux.HandleFunc("/stream", stream)
	return mux
}
func stream(w http.ResponseWriter, r *http.Request) {
	key, err := base64.StdEncoding.DecodeString(r.Header.Get("Sec-WebSocket-Key"))
	if r.Header.Get("Origin") != origin || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || r.Header.Get("Sec-WebSocket-Version") != "13" || err != nil || len(key) != 16 {
		http.Error(w, "invalid upgrade", 403)
		return
	}
	conn, rw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	digest := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(digest[:]))
	if rw.Flush() != nil {
		return
	}
	for {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		payload, err := readFrame(rw.Reader)
		if err != nil {
			return
		}
		var m struct {
			Action  string `json:"action"`
			Payload string `json:"payload"`
		}
		if json.Unmarshal(payload, &m) != nil {
			return
		}
		if m.Action != "input" {
			continue
		}
		data, _ := json.Marshal(map[string]string{"action": "output", "payload": m.Payload})
		if err = writeFrame(rw.Writer, data); err != nil {
			log.Print("fixture write failed")
			return
		}
	}
}
func readFrame(r *bufio.Reader) ([]byte, error) {
	var h [2]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return nil, err
	}
	if h[0] != 0x81 || h[1]&0x80 == 0 {
		return nil, fmt.Errorf("fixture accepts masked unfragmented text only")
	}
	n := uint64(h[1] & 127)
	if n == 126 {
		var v uint16
		if err := binary.Read(r, binary.BigEndian, &v); err != nil {
			return nil, err
		}
		n = uint64(v)
	} else if n == 127 {
		if err := binary.Read(r, binary.BigEndian, &n); err != nil {
			return nil, err
		}
	}
	if n > 1<<20 {
		return nil, fmt.Errorf("oversized frame")
	}
	var mask [4]byte
	if _, err := io.ReadFull(r, mask[:]); err != nil {
		return nil, err
	}
	data := make([]byte, int(n))
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	for i := range data {
		data[i] ^= mask[i%4]
	}
	return data, nil
}
func writeFrame(w *bufio.Writer, data []byte) error {
	if err := w.WriteByte(0x81); err != nil {
		return err
	}
	switch {
	case len(data) < 126:
		w.WriteByte(byte(len(data)))
	case len(data) <= 65535:
		w.WriteByte(126)
		binary.Write(w, binary.BigEndian, uint16(len(data)))
	default:
		w.WriteByte(127)
		binary.Write(w, binary.BigEndian, uint64(len(data)))
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return w.Flush()
}
