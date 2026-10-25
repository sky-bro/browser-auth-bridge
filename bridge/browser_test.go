package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
)

func TestBrowserCommandScope(t *testing.T) {
	origin := "https://console.example.test"
	good := BrowserCommand{Version: 1, Origin: origin, BrowserSession: strings.Repeat("a", 64), Action: "eval", Script: "return fetch('/any-new-api')", Args: json.RawMessage(`{"product":"adb5"}`)}
	raw, _ := json.Marshal(good)
	if _, err := DecodeBrowserCommand(raw, origin); err != nil {
		t.Fatal(err)
	}
	open := BrowserCommand{Version: 1, Origin: origin, BrowserSession: strings.Repeat("a", 64), Action: "open", URL: origin + "/", Match: "origin"}
	raw, _ = json.Marshal(open)
	if _, err := DecodeBrowserCommand(raw, origin); err != nil {
		t.Fatal(err)
	}
	open.Match = "anything"
	raw, _ = json.Marshal(open)
	if _, err := DecodeBrowserCommand(raw, origin); err == nil {
		t.Fatal("accepted unknown match policy")
	}
	for _, change := range []func(*BrowserCommand){func(c *BrowserCommand) { c.Origin = "https://evil.test" }, func(c *BrowserCommand) { c.Version = 2 }, func(c *BrowserCommand) { c.Action = "cdp" }, func(c *BrowserCommand) { c.Args = json.RawMessage(`"code"`) }, func(c *BrowserCommand) {
		c.Action = "open"
		c.Script = ""
		c.Args = nil
		c.URL = "https://user@console.example.test/product"
	}} {
		c := good
		change(&c)
		raw, _ := json.Marshal(c)
		if _, err := DecodeBrowserCommand(raw, origin); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
}
func TestExecuteBrowserTransport(t *testing.T) {
	namespace := fmt.Sprintf("browser-test-%d", os.Getpid())
	path, _ := BrokerSocketPath(namespace)
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		var m Message
		if json.NewDecoder(c).Decode(&m) != nil {
			return
		}
		if m.Operation != "browser" || !m.AllowTab || m.NoOpenTab {
			t.Error("lost operation or tab policy")
		}
		_ = json.NewEncoder(c).Encode(Message{Type: "result", Session: m.Session, Result: json.RawMessage(`{"version":1,"tabId":12,"value":{"ok":true}}`)})
	}()
	origin := "https://console.example.test"
	cmd := BrowserCommand{Version: 1, Origin: origin, BrowserSession: strings.Repeat("a", 64), Action: "open", URL: origin + "/product"}
	result, err := ExecuteBrowser(context.Background(), namespace, "demo-a", cmd, func(m Message) error { _, err := DecodeBrowserCommand(m.Payload, origin); return err }, BrowserOptions{AllowTab: true})
	if err != nil || result.TabID != 12 {
		t.Fatalf("%+v %v", result, err)
	}
	<-done
}
