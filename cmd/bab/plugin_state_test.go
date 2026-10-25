package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sky-bro/browser-auth-bridge/bridge"
	"github.com/sky-bro/browser-auth-bridge/plugins"
)

func TestPluginStateLifecyclePreservesSourceAndBlocksExecution(t *testing.T) {
	isolatedRegistry(t)
	captureList(t, "plugin", "install", "tjupt", "--disabled")
	p, err := find("tjupt")
	if err != nil || !p.Disabled {
		t.Fatal("disabled installation failed", err)
	}
	before, _ := os.ReadFile(filepath.Join(p.Directory, "bab-plugin.json"))
	if err = execute(p, "whoami", nil, ""); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("disabled browser command ran", err)
	}
	cmd := bridge.BrowserCommand{Version: 1, Origin: p.Origin, BrowserSession: strings.Repeat("a", 64), Action: "open", URL: p.Origin + "/"}
	data, _ := json.Marshal(cmd)
	m := bridge.Message{Type: "attach", Session: strings.Repeat("b", 64), Service: p.Site, Operation: "browser", Payload: data}
	if validate(m) == nil {
		t.Fatal("native host accepted disabled browser attach")
	}
	if !strings.Contains(captureList(t, "plugin", "list"), "disabled") {
		t.Fatal("disabled state is not visible")
	}
	captureList(t, "plugin", "update", "tjupt")
	p, err = find("tjupt")
	if err != nil || !p.Disabled {
		t.Fatal("update re-enabled the plugin", err)
	}
	captureList(t, "plugin", "enable", "tjupt")
	p, err = find("tjupt")
	if err != nil || p.Disabled {
		t.Fatal("enable failed", err)
	}
	if err = validate(m); err != nil {
		t.Fatal("enabled browser attach rejected", err)
	}
	after, _ := os.ReadFile(filepath.Join(p.Directory, "bab-plugin.json"))
	if string(before) != string(after) {
		t.Fatal("state toggle rewrote package policy")
	}
	info, err := os.Stat(filepath.Join(home(), "disabled.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("state is not private", err)
	}
	captureList(t, "plugin", "disable", "tjupt")
	catalog, err := catalog()
	if err != nil || !catalog[0].Disabled {
		t.Fatal("catalog omitted state", err)
	}
	captureList(t, "plugin", "remove", "tjupt")
	captureList(t, "plugin", "install", "tjupt")
	p, err = find("tjupt")
	if err != nil || p.Disabled {
		t.Fatal("removed state leaked into fresh installation", err)
	}
}
func TestDisabledDelegateNeverExecutesAndInvalidStateFailsClosed(t *testing.T) {
	isolatedRegistry(t)
	marker := filepath.Join(t.TempDir(), "must-not-run")
	executable := filepath.Join(t.TempDir(), "client")
	os.WriteFile(executable, []byte("#!/bin/sh\nprintf ran > "+shellQuote(marker)+"\n"), 0700)
	p := plugins.Plugin{Manifest: plugins.Manifest{Site: "console", Delegate: &plugins.Delegate{Executable: executable}}, Disabled: true}
	if delegate(p, nil) == nil {
		t.Fatal("disabled delegate executed")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("delegate side effect")
	}
	os.MkdirAll(home(), 0700)
	os.WriteFile(filepath.Join(home(), "disabled.json"), []byte(`{"unexpected":true}`), 0600)
	if _, err := all(); err == nil {
		t.Fatal("invalid state silently enabled plugins")
	}
}
func TestAvailableHasHumanAndJSONViews(t *testing.T) {
	isolatedRegistry(t)
	human := captureList(t, "plugin", "available")
	if !strings.Contains(human, "not installed") || strings.HasPrefix(strings.TrimSpace(human), "[") {
		t.Fatal("catalog is not a CLI table", human)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(captureList(t, "plugin", "available", "--json")), &rows); err != nil || len(rows) != 2 {
		t.Fatal("JSON catalog changed", err)
	}
	for _, row := range rows {
		if row["installed"] != false || row["enabled"] != false {
			t.Fatal("unsolicited plugin activation")
		}
	}
}
