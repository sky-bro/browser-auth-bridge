package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sky-bro/browser-auth-bridge/plugins"
)

func values(result completionResult) map[string]bool {
	out := map[string]bool{}
	for _, item := range result.items {
		out[item.value] = true
	}
	return out
}
func TestCompletionReadsInstalledPluginsAndDeclaredOptions(t *testing.T) {
	p := installed(t, false)
	candidates := values(completionCandidates([]string{""}))
	if !candidates["example"] || !candidates["completion"] || candidates["ttyd"] {
		t.Fatal("root completion did not reflect installed registry", candidates)
	}
	p.Commands["inspect"] = plugins.Command{Script: "run.js", Access: "read", Options: []plugins.Argument{{Name: "format", Choices: []string{"json", "yaml"}}}}
	data, _ := json.Marshal(p.Manifest)
	os.WriteFile(filepath.Join(p.Directory, "bab-plugin.json"), data, 0600)
	if !values(completionCandidates([]string{"example", ""}))["inspect"] {
		t.Fatal("new installed command needs completion regeneration")
	}
	if !values(completionCandidates([]string{"example", "inspect", "--"}))["--format"] {
		t.Fatal("declared option missing")
	}
	choices := values(completionCandidates([]string{"example", "inspect", "--format", ""}))
	if !choices["yaml"] || len(choices) != 2 {
		t.Fatal("option value choices missing", choices)
	}
	if !values(completionCandidates([]string{"example", "inspect", "--format=y"}))["--format=yaml"] {
		t.Fatal("inline option values missing")
	}
	if !completionCandidates([]string{"plugin", "catalog", "add", ""}).files {
		t.Fatal("catalog path does not request file completion")
	}
	if !values(completionCandidates([]string{"auth", "revoke", ""}))["example"] {
		t.Fatal("authorization completion missing installed sites")
	}
}
func TestCompletionNeverInvokesDelegatedExecutable(t *testing.T) {
	isolatedRegistry(t)
	source := t.TempDir()
	marker := filepath.Join(t.TempDir(), "executed")
	executable := filepath.Join(source, "fixture-cli")
	os.WriteFile(executable, []byte("#!/bin/sh\ntouch "+shellQuote(marker)+"\n"), 0700)
	manifest := plugins.Manifest{Version: 1, Site: "fixture", Origin: "https://fixture.example.test", Delegate: &plugins.Delegate{Executable: executable}, Completion: &plugins.CompletionSpec{Commands: map[string]plugins.CompletionSpec{"inspect": {Description: "Inspect fixture", Options: []plugins.Argument{{Name: "format", Choices: []string{"short", "full"}}}}}}}
	data, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(source, "bab-plugin.json"), data, 0600)
	if _, err := plugins.Install(root(), source, false); err != nil {
		t.Fatal(err)
	}
	if !values(completionCandidates([]string{"fixture", ""}))["inspect"] {
		t.Fatal("delegated completion metadata missing")
	}
	if values(completionCandidates([]string{"fixture", "inspect", "--"}))["--json"] {
		t.Fatal("browser-only option leaked into delegated CLI")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("completion ran delegated executable")
	}
}
func TestBuiltinTerminalMetadataWorksForOlderInstalledSnapshots(t *testing.T) {
	isolatedRegistry(t)
	entry, err := plugins.ResolveSource("ttyd", nil)
	if err != nil {
		t.Fatal(err)
	}
	source, cleanup, err := plugins.MaterializeBuiltin(entry)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	p, err := plugins.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	p.Completion = nil
	data, _ := json.Marshal(p.Manifest)
	os.WriteFile(filepath.Join(source, "bab-plugin.json"), data, 0600)
	if _, err = plugins.Install(root(), source, false); err != nil {
		t.Fatal(err)
	}
	commands := values(completionCandidates([]string{"ttyd", ""}))
	if !commands["shell"] || !commands["exec"] {
		t.Fatal("bundled metadata fallback missing", commands)
	}
	if !values(completionCandidates([]string{"ttyd", "exec", "--"}))["--timeout"] {
		t.Fatal("terminal timeout missing")
	}
}
func TestZshScriptParsesRealArgumentArraysWithoutEvaluation(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh unavailable")
	}
	directory := t.TempDir()
	script := filepath.Join(directory, "_bab")
	os.WriteFile(script, []byte(zshCompletion), 0600)
	if output, err := exec.Command(zsh, "-n", script).CombinedOutput(); err != nil {
		t.Fatal(err, string(output))
	}
	recorded := filepath.Join(directory, "argv")
	marker := filepath.Join(directory, "must-not-exist")
	fake := filepath.Join(directory, "bab fixture")
	candidate := "literal:$(touch " + marker + ")"
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellQuote(recorded) + "\nprintf 'value\\t%s\\t%s\\n' " + shellQuote(candidate) + " 'description: with spaces'\nprintf 'files\\n'\n"
	os.WriteFile(fake, []byte(body), 0700)
	code := `_describe(){ print -rl -- "${candidates[@]}"; }; _files(){ print -r -- file-fallback; }; words=("$1" plugin install ""); CURRENT=4; source "$2"`
	output, err := exec.Command(zsh, "-f", "-c", code, "completion-test", fake, script).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(output))
	}
	args, _ := os.ReadFile(recorded)
	if string(args) != "__complete\nplugin\ninstall\n\n" {
		t.Fatalf("completion cursor words changed: %q", args)
	}
	if !strings.Contains(string(output), "file-fallback") || !strings.Contains(string(output), "description\\: with spaces") {
		t.Fatalf("zsh completion response not decoded: %q", output)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("completion evaluated untrusted candidate text")
	}
}
func TestCompletionEnablePreservesStartupAndExistingCompinit(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh unavailable")
	}
	directory := t.TempDir()
	rc := filepath.Join(directory, "dotfiles", "zshrc")
	os.MkdirAll(filepath.Dir(rc), 0700)
	original := "# existing user configuration\nexport USER_PREFERENCE=keep\n"
	os.WriteFile(rc, []byte(original), 0600)
	link := filepath.Join(directory, ".zshrc")
	os.Symlink(rc, link)
	functions := filepath.Join(directory, "completion's directory")
	os.MkdirAll(functions, 0755)
	os.WriteFile(filepath.Join(functions, "_bab"), []byte(zshCompletion), 0644)
	if err = enableZshCompletion(link, functions); err != nil {
		t.Fatal(err)
	}
	if err = enableZshCompletion(link, functions); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(rc)
	if !strings.HasPrefix(string(data), original) || strings.Count(string(data), completionStart) != 1 {
		t.Fatal("modified unrelated configuration or duplicated block")
	}
	info, _ := os.Stat(rc)
	if info.Mode().Perm() != 0600 {
		t.Fatal("changed startup permissions")
	}
	info, _ = os.Lstat(link)
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("replaced user's symlink")
	}
	code := `compinit(){ return 99; }; compdef(){ [[ "$1 $2" == '_bab bab' ]] || return 98; }; source "$1"; [[ "$USER_PREFERENCE" == keep && $+functions[_bab] == 1 ]]`
	if output, err := exec.Command(zsh, "-f", "-c", code, "completion-test", link).CombinedOutput(); err != nil {
		t.Fatal("existing compinit or startup integration failed", err, string(output))
	}
	bad := "user configuration\n" + completionStart + "\n"
	os.WriteFile(rc, []byte(bad), 0600)
	if err = enableZshCompletion(link, functions); err == nil {
		t.Fatal("accepted broken managed block")
	}
	after, _ := os.ReadFile(rc)
	if string(after) != bad {
		t.Fatal("modified malformed existing configuration")
	}
}
