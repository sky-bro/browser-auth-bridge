package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, site string) string {
	t.Helper()
	d := t.TempDir()
	m := Manifest{Version: 1, Site: site, Origin: "https://example.test", Description: "test", Commands: map[string]Command{"search": {Script: "search.js", Access: "read", Positional: []Argument{{Name: "query", Required: true}}, Options: []Argument{{Name: "limit", Default: "10"}}}}}
	b, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(d, "bab-plugin.json"), b, 0600)
	os.WriteFile(filepath.Join(d, "search.js"), []byte("return args;"), 0600)
	return d
}
func TestNewCommandNeedsOnlyPluginUpdate(t *testing.T) {
	src := fixture(t, "example")
	root := t.TempDir()
	p, e := Install(root, src, false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Install(root, src, false); e == nil {
		t.Fatal("overwrote installed plugin")
	}
	m := p.Manifest
	m.Commands["details"] = Command{Script: "details.js", Access: "read"}
	b, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(src, "bab-plugin.json"), b, 0600)
	os.WriteFile(filepath.Join(src, "details.js"), []byte("return {title:'new'};"), 0600)
	p, e = Install(root, src, true)
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Commands) != 2 {
		t.Fatal("new command missing")
	}
	ps, e := All(root)
	if e != nil || len(ps) != 1 {
		t.Fatal(e)
	}
	s, e := ps[0].Script("details")
	if e != nil || s != "return {title:'new'};" {
		t.Fatal("new script unavailable", e)
	}
}
func TestInvalidPackages(t *testing.T) {
	for _, site := range []string{"../escape", "bab", "auth", "setup"} {
		if _, e := Load(fixture(t, site)); e == nil {
			t.Fatal("accepted reserved/invalid site", site)
		}
	}
	d := fixture(t, "example")
	os.Remove(filepath.Join(d, "search.js"))
	os.Symlink("/etc/hosts", filepath.Join(d, "search.js"))
	if _, e := Load(d); e == nil {
		t.Fatal("accepted symlink")
	}
}
func TestParsing(t *testing.T) {
	p, e := Load(fixture(t, "example"))
	if e != nil {
		t.Fatal(e)
	}
	v, out, e := Parse(p.Commands["search"], []string{"movie", "--limit", "5", "--json"})
	if e != nil || v["query"] != "movie" || v["limit"] != "5" || out != "" {
		t.Fatal(v, out, e)
	}
	for _, a := range [][]string{{}, {"movie", "extra"}, {"movie", "--oops", "x"}, {"movie", "--limit"}, {"movie", "--output", "x"}} {
		if _, _, e = Parse(p.Commands["search"], a); e == nil {
			t.Fatal("accepted invalid args", a)
		}
	}
}
func TestInstallCopiesSnapshotAndLeavesNoUnexpectedFiles(t *testing.T) {
	d := fixture(t, "example")
	os.WriteFile(filepath.Join(d, "secret.txt"), []byte("not packaged"), 0600)
	p, e := Install(t.TempDir(), d, false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(p.Directory, "secret.txt")); !os.IsNotExist(e) {
		t.Fatal("copied unreferenced file")
	}
	os.WriteFile(filepath.Join(d, "search.js"), []byte("changed"), 0600)
	script, _ := p.Script("search")
	if script != "return args;" {
		t.Fatal("installed package changed with source")
	}
}

func TestDelegateRegistrationAndAvailability(t *testing.T) {
	source := t.TempDir()
	m := Manifest{Version: 1, Site: "terminal", Origin: "https://terminal.example.test", Origins: []string{"https://catalog.example.test"}, Description: "Terminal", Delegate: &Delegate{Executable: "bab-test-nonexistent-executable"}}
	data, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(source, "bab-plugin.json"), data, 0600)
	p, err := Install(t.TempDir(), source, false)
	if err != nil {
		t.Fatal(err)
	}
	sites := Catalog([]Plugin{p})
	if len(sites) != 1 || sites[0].Available == nil || *sites[0].Available || len(sites[0].Origins) != 1 {
		t.Fatal("wrong registry metadata", sites)
	}
	m.Origins = []string{"https://*.example.test"}
	data, _ = json.Marshal(m)
	os.WriteFile(filepath.Join(source, "bab-plugin.json"), data, 0600)
	if _, err = Load(source); err == nil {
		t.Fatal("wildcard scope accepted")
	}
}

func TestInstalledDelegateSurvivesDifferentNativeHostPATH(t *testing.T) {
	bin := t.TempDir()
	executable := filepath.Join(bin, "fixture-delegate")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	source := t.TempDir()
	m := Manifest{Version: 1, Site: "terminal", Origin: "https://terminal.example.test", Description: "Terminal", Delegate: &Delegate{Executable: "fixture-delegate"}}
	data, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(source, "bab-plugin.json"), data, 0600)
	installed, err := Install(t.TempDir(), source, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/bin")
	reloaded, err := Load(installed.Directory)
	if err != nil {
		t.Fatal(err)
	}
	sites := Catalog([]Plugin{reloaded})
	if !*sites[0].Available || reloaded.Delegate.Executable != executable {
		t.Fatal("native-host PATH changed executable availability")
	}
}
