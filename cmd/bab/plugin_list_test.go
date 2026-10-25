package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sky-bro/browser-auth-bridge/plugins"
)

func captureList(t *testing.T, args ...string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "list-")
	if err != nil {
		t.Fatal(err)
	}
	before := os.Stdout
	os.Stdout = file
	err = run(args)
	os.Stdout = before
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func TestPluginListHumanDefaultAndFullJSONOptIn(t *testing.T) {
	p := installed(t, false)
	p.Description = "Example\nplugin\x1b[31m"
	data, _ := json.Marshal(p.Manifest)
	os.WriteFile(filepath.Join(p.Directory, "bab-plugin.json"), data, 0600)
	human := captureList(t, "plugin", "list")
	if strings.HasPrefix(strings.TrimSpace(human), "[") || !strings.Contains(human, "bab example") || !strings.Contains(human, "browser") || !strings.Contains(human, "available") || strings.ContainsRune(human, '\x1b') {
		t.Fatal("human list is not readable or escaped", human)
	}
	for _, args := range [][]string{{"plugin", "list", "--json"}, {"list", "--json"}} {
		output := captureList(t, args...)
		var values []plugins.Manifest
		if err := json.Unmarshal([]byte(output), &values); err != nil {
			t.Fatal(err)
		}
		if len(values) != 1 || values[0].Site != p.Site || values[0].Origin != p.Origin || values[0].Description != p.Description || len(values[0].Commands) != 1 {
			t.Fatal("JSON lost original manifest data")
		}
	}
	alias := captureList(t, "list")
	if alias != human {
		t.Fatal("list alias uses a different display")
	}
}
func TestEmptyListJSONAndMissingExecutable(t *testing.T) {
	isolatedRegistry(t)
	if output := captureList(t, "plugin", "list", "--json"); strings.TrimSpace(output) != "[]" {
		t.Fatal("empty JSON is not an array", output)
	}
	if output := captureList(t, "plugin", "list"); !strings.Contains(output, "No plugins installed") {
		t.Fatal("empty list is unclear", output)
	}
	source := t.TempDir()
	m := plugins.Manifest{Version: 1, Site: "fixture", Origin: "https://fixture.example.test", Description: "Fixture CLI", Delegate: &plugins.Delegate{Executable: "bab-fixture-not-installed"}}
	data, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(source, "bab-plugin.json"), data, 0600)
	if _, err := plugins.Install(root(), source, false); err != nil {
		t.Fatal(err)
	}
	if output := captureList(t, "plugin", "list"); !strings.Contains(output, "missing executable") {
		t.Fatal("unavailable delegate is not distinguished", output)
	}
	if !values(completionCandidates([]string{"plugin", "list", "--"}))["--json"] {
		t.Fatal("JSON flag not discoverable through completion")
	}
}
