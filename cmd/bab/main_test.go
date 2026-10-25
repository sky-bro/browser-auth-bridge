package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sky-bro/browser-auth-bridge/bridge"
	"github.com/sky-bro/browser-auth-bridge/plugins"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func installed(t *testing.T, artifact bool) plugins.Plugin {
	t.Helper()
	t.Setenv("BAB_HOME", t.TempDir())
	source := t.TempDir()
	manifest := plugins.Manifest{Version: 1, Site: "example", Origin: "https://example.test", Description: "Test", Commands: map[string]plugins.Command{"run": {Description: "Test", Script: "run.js", Access: "read", Artifact: artifact}}}
	b, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(source, "bab-plugin.json"), b, 0600)
	os.WriteFile(filepath.Join(source, "run.js"), []byte("return {ok:true};"), 0600)
	p, e := plugins.Install(root(), source, false)
	if e != nil {
		t.Fatal(e)
	}
	return p
}

// A real Unix broker exercises CLI session framing, browser workflow cleanup,
// result validation and artifact chunks without touching Chrome or live accounts.
func broker(t *testing.T, handler func(bridge.BrowserCommand) any) {
	t.Helper()
	old := namespace
	namespace = fmt.Sprintf("bab-test-%d", os.Getpid())
	t.Cleanup(func() { namespace = old })
	path, _ := bridge.BrokerSocketPath(namespace)
	listener, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	var workers sync.WaitGroup
	go func() {
		for {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer conn.Close()
				var m bridge.Message
				if json.NewDecoder(conn).Decode(&m) != nil {
					return
				}
				if e := validate(m); e != nil {
					t.Error(e)
					return
				}
				var command bridge.BrowserCommand
				json.Unmarshal(m.Payload, &command)
				value := handler(command)
				v, _ := json.Marshal(value)
				r, _ := json.Marshal(bridge.BrowserResult{Version: 1, Value: v})
				json.NewEncoder(conn).Encode(bridge.Message{Type: "result", Session: m.Session, Result: r})
			}()
		}
	}()
	t.Cleanup(func() { listener.Close(); workers.Wait() })
}
func TestArtifactChunkTransferAndNoOverwrite(t *testing.T) {
	p := installed(t, true)
	data := bytes.Repeat([]byte("test-payload"), 75000)
	chunks := 0
	closed := false
	broker(t, func(c bridge.BrowserCommand) any {
		switch c.Action {
		case "open":
			return nil
		case "close":
			closed = true
			return nil
		}
		if strings.Contains(c.Script, "const ctx") {
			return map[string]any{"artifact": map[string]any{"key": strings.Repeat("a", 36), "size": len(data)}}
		}
		if strings.Contains(c.Script, "return btoa") {
			var args struct{ Offset, Length int }
			json.Unmarshal(c.Args, &args)
			chunks++
			return base64.StdEncoding.EncodeToString(data[args.Offset : args.Offset+args.Length])
		}
		return nil
	})
	output := filepath.Join(t.TempDir(), "payload.torrent")
	if e := execute(p, "run", map[string]string{}, output); e != nil {
		t.Fatal(e)
	}
	got, e := os.ReadFile(output)
	if e != nil || !bytes.Equal(data, got) || chunks < 4 || !closed {
		t.Fatal("artifact or lifecycle mismatch", e, chunks, closed)
	}
	info, _ := os.Stat(output)
	if info.Mode().Perm() != 0600 {
		t.Fatal("insecure artifact permissions")
	}
	if e = execute(p, "run", map[string]string{}, output); !os.IsExist(e) {
		t.Fatal("existing output was not protected", e)
	}
}
func TestInvalidArtifactChunkRemovesPartialFile(t *testing.T) {
	p := installed(t, true)
	broker(t, func(c bridge.BrowserCommand) any {
		if strings.Contains(c.Script, "const ctx") {
			return map[string]any{"artifact": map[string]any{"key": strings.Repeat("b", 36), "size": 500000}}
		}
		return "corrupt chunk"
	})
	output := filepath.Join(t.TempDir(), "partial")
	if e := execute(p, "run", map[string]string{}, output); e == nil {
		t.Fatal("accepted corrupt chunk")
	}
	if _, e := os.Stat(output); !os.IsNotExist(e) {
		t.Fatal("left partial file")
	}
}
func TestNativeHostRejectsUnknownServicesAndOriginChanges(t *testing.T) {
	p := installed(t, false)
	id := strings.Repeat("a", 64)
	command := bridge.BrowserCommand{Version: 1, Origin: p.Origin, BrowserSession: id, Action: "open", URL: p.Origin + "/"}
	b, _ := json.Marshal(command)
	m := bridge.Message{Type: "attach", Session: id, Service: p.Site, Operation: "browser", Payload: b}
	if e := validate(m); e != nil {
		t.Fatal(e)
	}
	m.Service = "unknown"
	if validate(m) == nil {
		t.Fatal("unknown service accepted")
	}
	m.Service = p.Site
	command.Origin = "https://other.test"
	b, _ = json.Marshal(command)
	m.Payload = b
	if validate(m) == nil {
		t.Fatal("other origin accepted")
	}
	m.Service = "bab"
	m.Operation = "catalog"
	m.Payload = []byte(`[{"id":"evil","origin":"https://evil.test","label":"evil"}]`)
	if validate(m) == nil {
		t.Fatal("uninstalled catalog accepted")
	}
}

func TestDelegatePreservesArgumentsAndExitStatus(t *testing.T) {
	p := installed(t, false)
	executable := filepath.Join(t.TempDir(), "fixture-cli")
	if e := os.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\nexit 7\n"), 0700); e != nil {
		t.Fatal(e)
	}
	p.Delegate = &plugins.Delegate{Executable: executable}
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	err := delegate(p, []string{"literal value", "$(touch " + marker + ")"})
	var status interface{ ExitCode() int }
	if !errors.As(err, &status) || status.ExitCode() != 7 {
		t.Fatal("exit status lost", err)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("arguments passed through a shell")
	}
}
