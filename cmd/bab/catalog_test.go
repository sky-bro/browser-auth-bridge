package main

import (
	"github.com/sky-bro/browser-auth-bridge/plugins"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func isolatedRegistry(t *testing.T) {
	t.Helper()
	t.Setenv("BAB_HOME", t.TempDir())
	old := namespace
	namespace = "bab-unconnected-catalog-test"
	t.Cleanup(func() { namespace = old })
}
func TestSelectedInstallRequiresOriginAndPreservesItOnUpdate(t *testing.T) {
	isolatedRegistry(t)
	if err := installSelected("install", []string{"ttyd"}); err == nil {
		t.Fatal("installed example without user origin")
	}
	ps, err := all()
	if err != nil || len(ps) != 0 {
		t.Fatal("unselected plugins were installed", err)
	}
	origin := "https://terminal-owner.example.test:9443"
	if err = installSelected("install", []string{"ttyd", "--origin", origin}); err != nil {
		t.Fatal(err)
	}
	ps, err = all()
	if err != nil || len(ps) != 1 || ps[0].Origin != origin {
		t.Fatal("incorrect selection or origin", err)
	}
	if err = installSelected("update", []string{"ttyd"}); err != nil {
		t.Fatal(err)
	}
	p, err := find("ttyd")
	if err != nil || p.Origin != origin {
		t.Fatal("update lost user deployment", err)
	}
}
func TestConsumerOriginUsesDeclaredEnvironmentAndShellQuoting(t *testing.T) {
	p := installed(t, false)
	executable := filepath.Join(t.TempDir(), "fixture-cli")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s' \"$FIXTURE_ORIGIN\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	p.Delegate = &plugins.Delegate{Executable: executable, OriginEnv: "FIXTURE_ORIGIN"}
	output, err := os.CreateTemp(t.TempDir(), "output-")
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = output
	err = delegate(p, nil)
	os.Stdout = previous
	output.Close()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output.Name())
	if err != nil || string(data) != p.Origin {
		t.Fatal("delegate used another deployment", err)
	}
}
func TestNativeWrapperHasRealNewlinesAndSafeQuoting(t *testing.T) {
	isolatedRegistry(t)
	path, err := writeConsumerWrapper("example")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(data), "#!/bin/sh\nexec ") {
		t.Fatal("invalid wrapper", err, string(data))
	}
	if output, err := exec.Command("sh", "-n", path).CombinedOutput(); err != nil {
		t.Fatal("invalid wrapper syntax", err, string(output))
	}
	value := "path with ' quotes; $(echo unexpected)"
	output, err := exec.Command("sh", "-c", "printf '%s' "+shellQuote(value)).Output()
	if err != nil || string(output) != value {
		t.Fatal("unsafe shell quoting", err, string(output))
	}
}
