package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the documented lifecycle against an initially empty user registry.
// The isolated socket namespace cannot contact a developer's running Chrome.
func TestFreshInstallLifecycleBuildsOnlySelectedComponents(t *testing.T) {
	isolatedRegistry(t)
	namespace = "bab-fresh-install-test"
	directory := filepath.Join(home(), "extension")
	captureList(t, "setup")
	if _, err := os.Stat(filepath.Join(directory, "plugins")); !os.IsNotExist(err) {
		t.Fatal("unselected browser code installed")
	}

	// This executable would leave a marker if installation accidentally ran it.
	prefix := t.TempDir()
	executable := filepath.Join(prefix, "bridge-ttyd")
	marker := filepath.Join(prefix, "should-not-run")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf run > "+shellQuote(marker)+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", prefix+string(os.PathListSeparator)+os.Getenv("PATH"))
	origin := "https://chosen-terminal.example.test:9443"
	captureList(t, "plugin", "install", "ttyd", "--origin", origin)
	asset := filepath.Join(directory, "plugins/ttyd/browser/ttyd.js")
	if _, err := os.Stat(asset); err != nil {
		t.Fatal("selected adapter was not assembled", err)
	}
	p, err := find("ttyd")
	if err != nil || p.Origin != origin || p.Delegate.Executable != executable {
		t.Fatal("configured install is inconsistent", err)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("plugin executable ran during installation")
	}
	captureList(t, "plugin", "update", "ttyd")
	p, err = find("ttyd")
	if err != nil || p.Origin != origin {
		t.Fatal("update lost the selected deployment", err)
	}

	// A bundled page plugin installs without an origin override or browser rebuild.
	beforePageInstall, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	captureList(t, "plugin", "install", "tjupt")
	page, err := find("tjupt")
	if err != nil || page.Origin != "https://tjupt.org" {
		t.Fatal("bundled page install failed", err)
	}
	for _, name := range []string{"whoami", "search", "details", "torrent"} {
		if script, err := page.Script(name); err != nil || script == "" {
			t.Fatal("page command missing", name, err)
		}
	}
	captureList(t, "plugin", "update", "tjupt")
	captureList(t, "plugin", "remove", "tjupt")
	afterPageInstall, err := os.Stat(directory)
	if err != nil || !os.SameFile(beforePageInstall, afterPageInstall) {
		t.Fatal("page-only lifecycle rebuilt the extension", err)
	}

	// A page-script-only plugin does not add browser extension assets.
	captureList(t, "plugin", "install", "../../examples/plugins/echo")
	if _, err = os.Stat(filepath.Join(directory, "plugins/echo")); !os.IsNotExist(err) {
		t.Fatal("page scripts copied into extension")
	}
	captureList(t, "plugin", "remove", "ttyd")
	if _, err = os.Stat(asset); !os.IsNotExist(err) {
		t.Fatal("removed consumer code remains packaged")
	}
	config, err := os.ReadFile(filepath.Join(directory, "config.js"))
	if err != nil || strings.Contains(string(config), "chosen-terminal") || strings.Contains(string(config), "org.browserauthbridge.ttyd") {
		t.Fatal("removed deployment remains in extension routing", err)
	}
	ps, err := all()
	if err != nil || len(ps) != 1 || ps[0].Site != "echo" {
		t.Fatal("removal changed another plugin", err)
	}
	captureList(t, "plugin", "remove", "echo")
	if got := strings.TrimSpace(captureList(t, "plugin", "list", "--json")); got != "[]" {
		t.Fatal("registry is not empty after cleanup", got)
	}
}
