package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sky-bro/browser-auth-bridge/plugins"
)

func TestPluginScaffoldCreatesInstallablePagePluginAndPreservesExistingDirectory(t *testing.T) {
	isolatedRegistry(t)
	path := filepath.Join(t.TempDir(), "my-site")
	if err := initPlugin([]string{"my-site", "--origin", "https://example.test", "--dir", path}); err != nil {
		t.Fatal(err)
	}
	p, err := plugins.Load(path)
	if err != nil || p.Delegate != nil || p.Commands["status"].Script != "status.js" {
		t.Fatal("invalid starter", err)
	}
	if _, err = plugins.Install(root(), path, false); err != nil {
		t.Fatal("starter is not installable", err)
	}
	before, _ := os.ReadFile(filepath.Join(path, "status.js"))
	if err = initPlugin([]string{"my-site", "--origin", "https://other.test", "--dir", path}); err == nil {
		t.Fatal("existing source overwritten")
	}
	after, _ := os.ReadFile(filepath.Join(path, "status.js"))
	if string(before) != string(after) {
		t.Fatal("existing file changed")
	}
}
func TestPluginScaffoldValidatesConsumerAndScope(t *testing.T) {
	for _, args := range [][]string{{"bad/name", "--origin", "https://example.test"}, {"auth", "--origin", "https://example.test"}, {"example", "--origin", "http://remote.test"}, {"example", "--origin", "https://example.test", "--native-host", "org.example.client"}} {
		args = append(args, "--dir", filepath.Join(t.TempDir(), "source"))
		if err := initPlugin(args); err == nil {
			t.Fatal("invalid scaffold accepted", args)
		}
	}
	path := filepath.Join(t.TempDir(), "source")
	if err := initPlugin([]string{"example", "--origin", "https://example.test", "--dir", path, "--executable", "my-cli", "--native-host", "org.example.client"}); err != nil {
		t.Fatal(err)
	}
	p, err := plugins.Load(path)
	if err != nil || p.Delegate == nil || p.Delegate.NativeHost != "org.example.client" {
		t.Fatal("invalid consumer scaffold", err)
	}
}

func TestPluginScaffoldDefaultsToUserSourcesOutsideRepository(t *testing.T) {
	isolatedRegistry(t)
	if err := initPlugin([]string{"my-site", "--origin", "https://example.test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := plugins.Load(filepath.Join(home(), "sources/my-site")); err != nil {
		t.Fatal("default source is not user-owned", err)
	}
}
