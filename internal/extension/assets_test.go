package extension

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sky-bro/browser-auth-bridge/plugins"
)

func componentFixture(t *testing.T, site string) plugins.Plugin {
	t.Helper()
	directory := t.TempDir()
	manifest := plugins.Manifest{Version: 1, Site: site, Origin: "https://terminal.example.test", Description: "Optional terminal", Commands: map[string]plugins.Command{}, Delegate: &plugins.Delegate{Executable: "optional-console", NativeHost: "org.example." + site}, Extension: &plugins.BrowserExtension{Files: []string{"browser/adapter.js", "browser/background.js"}, Background: []string{"browser/background.js"}, PTYAdapter: site, PTYScripts: []string{"browser/adapter.js"}}}
	b, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(directory, "bab-plugin.json"), b, 0600)
	os.Mkdir(filepath.Join(directory, "browser"), 0700)
	os.WriteFile(filepath.Join(directory, "browser/adapter.js"), []byte("// private protocol sentinel\n"), 0600)
	os.WriteFile(filepath.Join(directory, "browser/background.js"), []byte("// private background sentinel\n"), 0600)
	p, err := plugins.Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestAssemblyIsSelectedNamespacedAndRemovable(t *testing.T) {
	p := componentFixture(t, "console")
	files, err := Render(nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(files["config.js"]), "console") || len(files) != len(coreFiles) {
		t.Fatal("implicit plugin dependency")
	}
	output := filepath.Join(t.TempDir(), "extension")
	if err = InstallPlugins(output, []plugins.Plugin{p}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(output, "plugins/console/browser/adapter.js")
	if b, err := os.ReadFile(path); err != nil || !strings.Contains(string(b), "sentinel") {
		t.Fatal("consumer-owned script missing", err)
	}
	config, _ := os.ReadFile(filepath.Join(output, "config.js"))
	for _, want := range []string{"plugins/console/browser/adapter.js", "plugins/console/browser/background.js", "org.example.console"} {
		if !strings.Contains(string(config), want) {
			t.Fatal("missing routing", want)
		}
	}
	if err = InstallPlugins(output, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("removed plugin remains in extension")
	}
}
func TestInvalidAssemblyPreservesInstalledExtensionAndUserDirectories(t *testing.T) {
	output := filepath.Join(t.TempDir(), "extension")
	if err := InstallPlugins(output, nil); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(output, "config.js"))
	p := componentFixture(t, "console")
	p.Extension.Background = []string{"../escape.js"}
	if err := InstallPlugins(output, []plugins.Plugin{p}); err == nil {
		t.Fatal("invalid component accepted")
	}
	after, _ := os.ReadFile(filepath.Join(output, "config.js"))
	if string(before) != string(after) {
		t.Fatal("invalid build changed installation")
	}
	directory := t.TempDir()
	os.WriteFile(filepath.Join(directory, "personal.txt"), []byte("preserve"), 0600)
	if err := InstallPlugins(directory, nil); err == nil {
		t.Fatal("unowned directory overwritten")
	}
	if b, _ := os.ReadFile(filepath.Join(directory, "personal.txt")); string(b) != "preserve" {
		t.Fatal("user file changed")
	}
}
func TestDuplicateProviderHostsAndEscapingAssetsAreRejected(t *testing.T) {
	a, b := componentFixture(t, "console"), componentFixture(t, "inventory")
	b.Delegate.NativeHost = a.Delegate.NativeHost
	if _, err := Render([]plugins.Plugin{a, b}); err == nil {
		t.Fatal("ambiguous native host accepted")
	}
	a.Extension.Files = []string{"../outside.js"}
	if _, err := Render([]plugins.Plugin{a}); err == nil {
		t.Fatal("escaping asset accepted")
	}
}
func TestBundledExampleAndExternalTemplateHaveSameConsumerCode(t *testing.T) {
	entry, err := plugins.ResolveSource("ttyd", nil)
	if err != nil {
		t.Fatal(err)
	}
	directory, cleanup, err := plugins.MaterializeBuiltin(entry)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	a, err := plugins.Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	b, err := plugins.Load("../../examples/plugins/ttyd")
	if err != nil {
		t.Fatal(err)
	}
	aa, _ := a.BrowserAssets()
	bb, _ := b.BrowserAssets()
	for name, data := range aa {
		if string(data) != string(bb[name]) {
			t.Fatal("bundled example drift", name)
		}
	}
}
