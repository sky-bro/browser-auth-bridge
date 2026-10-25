package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicCatalogIsOptInAndDeploymentIndependent(t *testing.T) {
	entries, err := Available(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name != "tjupt" || entries[0].OriginRequired || entries[1].Name != "ttyd" || !entries[1].OriginRequired {
		t.Fatal("unexpected distributed catalog", entries)
	}
	source, cleanup, err := MaterializeBuiltin(entries[1])
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	before, _ := os.ReadFile(filepath.Join(source, "bab-plugin.json"))
	installed, err := InstallConfigured(t.TempDir(), source, false, "https://my-terminal.example.test:9443")
	if err != nil {
		t.Fatal(err)
	}
	if installed.Origin != "https://my-terminal.example.test:9443" || installed.Delegate.OriginEnv != "BRIDGE_TTYD_ORIGIN" {
		t.Fatal("deployment origin was not configured")
	}
	reloaded, err := Load(installed.Directory)
	if err != nil || reloaded.Origin != installed.Origin {
		t.Fatal("configured snapshot did not persist", err)
	}
	after, _ := os.ReadFile(filepath.Join(source, "bab-plugin.json"))
	if string(before) != string(after) {
		t.Fatal("modified distributed template")
	}
}
func TestExternalCatalogSupportsCheckedOutSubmoduleDirectories(t *testing.T) {
	directory := t.TempDir()
	source := fixture(t, "external-example")
	// A submodule's .git marker is metadata, never an installation input.
	os.WriteFile(filepath.Join(source, ".git"), []byte("gitdir: ../metadata"), 0600)
	relative, err := filepath.Rel(directory, source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "catalog.json")
	data, _ := json.Marshal(SourceCatalog{Version: 1, Plugins: []CatalogEntry{{Name: "external-example", Path: relative, Description: "External plugin"}}})
	os.WriteFile(path, data, 0600)
	entry, err := ResolveSource("external-example", []string{path})
	if err != nil {
		t.Fatal(err)
	}
	installed, err := Install(t.TempDir(), entry.Path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(installed.Directory, ".git")); !os.IsNotExist(err) {
		t.Fatal("installed submodule metadata")
	}
	data, _ = json.Marshal(SourceCatalog{Version: 1, Plugins: []CatalogEntry{{Name: "ttyd", Path: relative}}})
	os.WriteFile(path, data, 0600)
	if _, err = Available([]string{path}); err == nil {
		t.Fatal("ambiguous catalog name accepted")
	}
}

func TestBundledPagePluginMaterializesAndInstallsEveryCommand(t *testing.T) {
	entry, err := ResolveSource("tjupt", nil)
	if err != nil {
		t.Fatal(err)
	}
	source, cleanup, err := MaterializeBuiltin(entry)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	p, err := Install(t.TempDir(), source, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Origin != "https://tjupt.org" || p.Delegate != nil || p.Extension != nil || len(p.Commands) != 4 {
		t.Fatal("unexpected page plugin contract")
	}
	for _, name := range []string{"whoami", "search", "details", "torrent"} {
		script, err := p.Script(name)
		if err != nil || script == "" {
			t.Fatal("missing installed command", name, err)
		}
		original, err := builtins.ReadFile("builtin/tjupt/" + p.Commands[name].Script)
		if err != nil || script != string(original) {
			t.Fatal("bundled command changed", name, err)
		}
	}
	if !p.Commands["torrent"].Artifact {
		t.Fatal("torrent command lost protected artifact output")
	}
}
