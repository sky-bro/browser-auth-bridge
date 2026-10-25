package main

import (
	"bytes"
	"encoding/json"
	"github.com/sky-bro/browser-auth-bridge/bridge"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNativeHostPinsConsumerEndpoint(t *testing.T) {
	c := config{"https://terminal.example.test"}
	e := endpoint(c)
	p := bridge.PTYOpen{Version: 1, Provider: "ttyd", Target: "shell", Endpoint: &e, Cols: 80, Rows: 24}
	payload, _ := json.Marshal(p)
	m := bridge.Message{Service: "ttyd", Operation: "pty", Payload: payload}
	if err := validate(c, m); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*bridge.PTYOpen){func(p *bridge.PTYOpen) { p.Target = "host-shell" }, func(p *bridge.PTYOpen) { p.Endpoint.SocketURL = c.Origin + "/other" }, func(p *bridge.PTYOpen) { p.Endpoint.Adapter = "other" }, func(p *bridge.PTYOpen) { p.Endpoint.PageURL = c.Origin + "/admin" }} {
		bad := p
		copy := e
		bad.Endpoint = &copy
		change(&bad)
		m.Payload, _ = json.Marshal(bad)
		if validate(c, m) == nil {
			t.Fatal("accepted endpoint not chosen by consumer")
		}
	}
}

func TestHelpAndArgumentErrorsDoNotRequireInstallation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("BRIDGE_TTYD_ORIGIN", "")
	for _, args := range [][]string{nil, {"--help"}, {"help"}, {"--version"}} {
		var out bytes.Buffer
		if err := runCLI(args, &out); err != nil || out.Len() == 0 {
			t.Fatalf("%v: output=%q error=%v", args, out.String(), err)
		}
	}
	for _, args := range [][]string{{"unknown"}, {"shell", "extra"}, {"exec"}, {"exec", "--"}, {"install-host"}} {
		err := runCLI(args, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "Usage:") {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestDoctorReportsMissingSetupWithoutCreatingFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("BRIDGE_TTYD_ORIGIN", "")
	var out bytes.Buffer
	if err := doctor(&out); err == nil {
		t.Fatal("missing setup passed")
	}
	if !strings.Contains(out.String(), "FAIL origin configuration") || !strings.Contains(out.String(), "not checked") {
		t.Fatal(out.String())
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("doctor mutated home: %v %v", entries, err)
	}
}
