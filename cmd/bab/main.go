// bab hosts trusted, independently installed browser adapters.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/sky-bro/browser-auth-bridge/bridge"
	assets "github.com/sky-bro/browser-auth-bridge/internal/extension"
	"github.com/sky-bro/browser-auth-bridge/nativehost"
	"github.com/sky-bro/browser-auth-bridge/plugins"
)

const host = "org.browserauthbridge.bab"

var version = "dev"
var namespace = "bab"

func home() string {
	if h := os.Getenv("BAB_HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".bab")
}
func root() string { return filepath.Join(home(), "plugins") }
func all() ([]plugins.Plugin, error) {
	ps, err := plugins.All(root())
	if err != nil {
		return nil, err
	}
	disabled, err := disabledPlugins()
	if err != nil {
		return nil, err
	}
	for i := range ps {
		ps[i].Disabled = disabled[ps[i].Site]
	}
	return ps, nil
}
func catalog() ([]plugins.Site, error) { ps, e := all(); return plugins.Catalog(ps), e }
func find(site string) (plugins.Plugin, error) {
	if !plugins.Identifier.MatchString(site) {
		return plugins.Plugin{}, fmt.Errorf("invalid site")
	}
	p, err := plugins.Load(filepath.Join(root(), site))
	if err != nil {
		return p, err
	}
	disabled, err := disabledPlugins()
	if err != nil {
		return p, err
	}
	p.Disabled = disabled[site]
	return p, nil
}
func validate(m bridge.Message) error {
	if m.Type != "attach" || !bridge.ValidSession(m.Session) {
		return fmt.Errorf("invalid attach")
	}
	if m.Service == "bab" {
		switch m.Operation {
		case "catalog":
			var got []plugins.Site
			if plugins.Decode(m.Payload, &got) != nil {
				return fmt.Errorf("invalid catalog")
			}
			want, e := catalog()
			if e != nil {
				return e
			}
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(want)
			if string(a) != string(b) {
				return fmt.Errorf("catalog mismatch")
			}
			return nil
		case "manage":
			var p management
			if plugins.Decode(m.Payload, &p) != nil || (p.Action != "status" && p.Action != "revoke") {
				return fmt.Errorf("invalid management action")
			}
			if p.Site != "" {
				_, e := find(p.Site)
				return e
			}
			if p.Action != "status" {
				return fmt.Errorf("site required")
			}
			return nil
		}
		return fmt.Errorf("unknown host operation")
	}
	if m.Operation != "browser" {
		return fmt.Errorf("operation denied")
	}
	p, e := find(m.Service)
	if e != nil {
		return e
	}
	cmd, e := bridge.DecodeBrowserCommand(m.Payload, p.Origin)
	if e == nil && cmd.Action != "close" {
		e = requireEnabled(p)
	}
	return e
}

type management struct {
	Action string `json:"action"`
	Site   string `json:"site,omitempty"`
}

func rpc(operation string, payload any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id, e := bridge.NewSessionID()
	if e != nil {
		return nil, e
	}
	b, e := json.Marshal(payload)
	if e != nil {
		return nil, e
	}
	m := bridge.Message{Type: "attach", Session: id, Service: "bab", Operation: operation, Payload: b}
	if e = validate(m); e != nil {
		return nil, e
	}
	s, e := bridge.Connect(ctx, namespace, m, time.Second)
	if e != nil {
		return nil, fmt.Errorf("bridge unavailable; run bab doctor and reconnect from the extension")
	}
	defer s.Close()
	for {
		m, e = s.Read()
		if e != nil {
			return nil, fmt.Errorf("bridge request interrupted")
		}
		if m.Session != id {
			return nil, fmt.Errorf("session mismatch")
		}
		if m.Type == "result" {
			return m.Result, nil
		}
		if m.Type == "error" {
			return nil, fmt.Errorf("bridge rejected request")
		}
	}
}
func syncCatalog() error {
	c, e := catalog()
	if e != nil {
		return e
	}
	_, e = rpc("catalog", c)
	return e
}
func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		var status interface{ ExitCode() int }
		if errors.As(e, &status) {
			os.Exit(status.ExitCode())
		}
		os.Exit(1)
	}
}
func printJSON(v any) error {
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
func run(args []string) error {
	if len(args) > 0 && args[0] == "native-consumer" {
		if len(args) < 3 {
			return fmt.Errorf("invalid native consumer invocation")
		}
		p, e := find(args[1])
		if e != nil {
			return e
		}
		if p.Delegate == nil || p.Delegate.NativeHost == "" {
			return fmt.Errorf("no registered native consumer")
		}
		return delegate(p, args[2:])
	}

	if len(args) > 0 && (args[0] == "native-host" || strings.HasPrefix(args[0], "chrome-extension://")) {
		if _, e := all(); e != nil {
			return e
		}
		c, _ := catalog()
		b, _ := json.Marshal(c)
		if e := nativehost.WriteMessage(os.Stdout, bridge.Message{Type: "catalog", Payload: b}); e != nil {
			return e
		}
		return nativehost.Run(os.Stdin, os.Stdout, nativehost.Config{Namespace: namespace, ValidateAttach: validate})
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Println("bab setup [--extension-id ID]\nbab doctor\nbab extension build [--output PATH]\nbab completion zsh [--install] [--enable]\nbab list [--json]\nbab plugin available [--json]\nbab plugin enable|disable SITE\nbab plugin init NAME --origin URL\nbab plugin install|update NAME|PATH... [--origin URL] [--disabled]\nbab plugin catalog add|remove PATH | list\nbab plugin list [--json]\nbab plugin sync\nbab plugin remove SITE\nbab auth login|status|revoke [SITE]\nbab SITE COMMAND [ARGS] [--json] [--output PATH]\nPlugins are trusted page code. Install only packages you trust.")
		return nil
	}
	switch args[0] {
	case "completion":
		return completionCommand(args[1:])
	case "__complete":
		return printCompletion(args[1:])
	case "--version", "version":
		fmt.Println(version)
		return nil
	case "extension":
		return extensionCommand(args[1:])
	case "setup":
		return setup(args[1:])
	case "doctor":
		return doctor()
	case "list":
		return listPlugins(args[1:])
	case "plugin":
		return pluginCommand(args[1:])
	case "auth":
		return auth(args[1:])
	}
	p, e := find(args[0])
	if e != nil {
		return e
	}
	if p.Delegate != nil {
		return delegate(p, args[1:])
	}
	if len(args) == 1 || args[1] == "--help" {
		return printJSON(p.Manifest)
	}
	c, ok := p.Commands[args[1]]
	if !ok {
		return fmt.Errorf("unknown command; run bab %s --help", p.Site)
	}
	if len(args) == 3 && args[2] == "--help" {
		return printJSON(c)
	}
	values, output, e := plugins.Parse(c, args[2:])
	if e != nil {
		return e
	}
	return execute(p, args[1], values, output)
}
func pluginCommand(args []string) error {
	if len(args) > 0 && args[0] == "init" {
		return initPlugin(args[1:])
	}
	if len(args) > 0 && args[0] == "catalog" {
		return sourceCatalogCommand(args[1:])
	}
	if len(args) > 0 && args[0] == "available" {
		return availablePlugins(args[1:])
	}
	if len(args) == 2 && (args[0] == "enable" || args[0] == "disable") {
		return pluginStateCommand(args[0], args[1])
	}
	if len(args) > 0 && (args[0] == "install" || args[0] == "update") {
		return installSelected(args[0], args[1:])
	}
	if len(args) > 0 && args[0] == "list" {
		return listPlugins(args[1:])
	}
	if len(args) == 1 && args[0] == "sync" {
		return syncCatalog()
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: bab plugin available | install|update NAME|PATH [--disabled] | enable|disable|remove SITE | list | sync")
	}
	if args[0] == "remove" {
		p, e := find(args[1])
		if e != nil {
			return e
		}
		if e = os.RemoveAll(p.Directory); e != nil {
			return e
		}
		if e = changePluginState(func(state map[string]bool) { delete(state, p.Site) }); e != nil {
			return e
		}
		if p.Extension != nil {
			if e = rebuildExtension(); e != nil {
				return e
			}
		}
		if e = syncCatalog(); e != nil {
			fmt.Fprintln(os.Stderr, "Removed locally. Reconnect the extension to refresh its catalog; existing Chrome grants can be removed in extension settings.")
		}
		return nil
	}
	return fmt.Errorf("unknown plugin operation")
}
func registration() string {
	h, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(h, "Library/Application Support/Google/Chrome/NativeMessagingHosts", host+".json")
	}
	return filepath.Join(h, ".config/google-chrome/NativeMessagingHosts", host+".json")
}
func setup(args []string) error {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return fmt.Errorf("setup currently supports Chrome on macOS and Linux")
	}
	id := ""
	if len(args) > 0 {
		if len(args) != 2 || args[0] != "--extension-id" {
			return fmt.Errorf("usage: bab setup [--extension-id ID]")
		}
		id = args[1]
		if !regexp.MustCompile(`^[a-p]{32}$`).MatchString(id) {
			return fmt.Errorf("invalid extension ID")
		}
	}
	if e := os.MkdirAll(home(), 0700); e != nil {
		return e
	}
	ps, loadErr := all()
	if loadErr != nil {
		return loadErr
	}
	if e := assets.InstallPlugins(filepath.Join(home(), "extension"), ps); e != nil {
		return e
	}
	if id == "" {
		fmt.Printf("Load %s in chrome://extensions (Developer mode → Load unpacked).\nFor an existing shared extension, update it with the BAB assets first.\nOpen its BAB page and copy the displayed setup command.\n", filepath.Join(home(), "extension"))
		return nil
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		return e
	}
	b, _ := json.MarshalIndent(map[string]any{"name": host, "description": "BAB plugin host", "path": exe, "type": "stdio", "allowed_origins": []string{"chrome-extension://" + id + "/"}}, "", "  ")
	path := registration()
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	if e = os.WriteFile(path, b, 0600); e != nil {
		return e
	}
	b, _ = json.Marshal(map[string]string{"extensionId": id})
	if e = os.WriteFile(filepath.Join(home(), "setup.json"), b, 0600); e != nil {
		return e
	}
	if e = registerDelegates(id); e != nil {
		return e
	}
	fmt.Println("Registered BAB and available CLI hosts. Click Reconnect on the extension page, then run bab doctor.")
	return nil
}
func extensionID() (string, error) {
	b, e := os.ReadFile(filepath.Join(home(), "setup.json"))
	if e != nil {
		return "", fmt.Errorf("run bab setup first")
	}
	var c struct {
		ID string `json:"extensionId"`
	}
	if plugins.Decode(b, &c) != nil || !regexp.MustCompile(`^[a-p]{32}$`).MatchString(c.ID) {
		return "", fmt.Errorf("invalid setup configuration")
	}
	return c.ID, nil
}
func auth(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: bab auth login|status|revoke [SITE]")
	}
	site := ""
	if len(args) == 2 {
		site = args[1]
		if _, e := find(site); e != nil {
			return e
		}
	}
	switch args[0] {
	case "login":
		if site == "" {
			return fmt.Errorf("site required")
		}
		id, e := extensionID()
		if e != nil {
			return e
		}
		url := "chrome-extension://" + id + "/bab.html#" + site
		fmt.Println("Authorize the site on the extension page, then click Open login page.")
		return openURL(url)
	case "revoke":
		if site == "" {
			return fmt.Errorf("site required")
		}
		r, e := rpc("manage", management{"revoke", site})
		if e != nil {
			return e
		}
		return printJSON(r)
	case "status":
		r, e := rpc("manage", management{"status", site})
		if e != nil {
			return e
		}
		return printJSON(r)
	}
	return fmt.Errorf("unknown auth operation")
}
func openURL(url string) error {
	var c *exec.Cmd
	if runtime.GOOS == "darwin" {
		c = exec.Command("open", "-a", "Google Chrome", url)
	} else {
		c = exec.Command("xdg-open", url)
	}
	return c.Run()
}
func doctor() error {
	ps, e := all()
	if e != nil {
		return e
	}
	_, regErr := os.Stat(registration())
	r, rpcErr := rpc("manage", management{Action: "status"})
	var status any
	if rpcErr == nil {
		_ = json.Unmarshal(r, &status)
	}
	result := map[string]any{"version": version, "plugins": len(ps), "nativeHostRegistered": regErr == nil, "bridgeConnected": rpcErr == nil, "sites": status}
	if rpcErr != nil {
		result["nextStep"] = "Run bab setup, then click Reconnect on the extension BAB page."
	}
	if e = printJSON(result); e != nil {
		return e
	}
	if rpcErr != nil || regErr != nil {
		return fmt.Errorf("setup incomplete")
	}
	return nil
}

const sdk = `const ctx = Object.freeze({
 async fetch(path) { const url = new URL(path, location.origin); if (url.origin !== location.origin || url.username || url.password) throw new Error('scope'); return fetch(url.href, {credentials:'include', redirect:'error', signal:AbortSignal.timeout(40000)}); },
 artifact(bytes) { if (!(bytes instanceof Uint8Array) || bytes.length > 16*1024*1024) throw new Error('artifact'); const key=crypto.randomUUID(); globalThis.__babArtifacts ??= new Map(); globalThis.__babArtifacts.set(key,bytes); setTimeout(()=>globalThis.__babArtifacts?.delete(key),180000); return {artifact:{key,size:bytes.length}}; }
});
`

func execute(p plugins.Plugin, name string, args map[string]string, output string) error {
	if err := requireEnabled(p); err != nil {
		return err
	}
	if p.Commands[name].Artifact {
		if _, e := os.Lstat(output); e == nil {
			return &os.PathError{Op: "create", Path: output, Err: os.ErrExist}
		} else if !os.IsNotExist(e) {
			return e
		}
	}
	script, e := p.Script(name)
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	id, e := bridge.NewSessionID()
	if e != nil {
		return e
	}
	cmd := bridge.BrowserCommand{Version: 1, Origin: p.Origin, BrowserSession: id, Action: "open", URL: p.Origin + "/", Match: "origin"}
	if _, e = bridge.ExecuteBrowser(ctx, namespace, p.Site, cmd, validate, bridge.BrowserOptions{AllowTab: true}); e != nil {
		return e
	}
	defer func() {
		closeCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_, ce := bridge.ExecuteBrowser(closeCtx, namespace, p.Site, bridge.BrowserCommand{Version: 1, Origin: p.Origin, BrowserSession: id, Action: "close"}, validate, bridge.BrowserOptions{})
		if ce != nil {
			fmt.Fprintln(os.Stderr, "Browser lease cleanup failed; reconnect if the page remains reserved.")
		}
	}()
	eval := func(script string, args any) (json.RawMessage, error) {
		b, e := json.Marshal(args)
		if e != nil {
			return nil, e
		}
		cmd.Action = "eval"
		cmd.URL = ""
		cmd.Match = ""
		cmd.Script = script
		cmd.Args = b
		r, e := bridge.ExecuteBrowser(ctx, namespace, p.Site, cmd, validate, bridge.BrowserOptions{})
		return r.Value, e
	}
	result, e := eval(sdk+script, args)
	if e != nil {
		return e
	}
	var failure struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(result, &failure)
	if failure.Error.Code != "" {
		if failure.Error.Code == "login_required" {
			return fmt.Errorf("login required; run bab auth login %s", p.Site)
		}
		return fmt.Errorf("adapter failed; inspect site and plugin before retrying")
	}
	if !p.Commands[name].Artifact {
		return printJSON(result)
	}
	var artifact struct {
		Artifact struct {
			Key  string `json:"key"`
			Size int    `json:"size"`
		} `json:"artifact"`
	}
	if plugins.Decode(result, &artifact) != nil || !regexp.MustCompile(`^[a-f0-9-]{36}$`).MatchString(artifact.Artifact.Key) || artifact.Artifact.Size < 1 || artifact.Artifact.Size > 16*1024*1024 {
		return fmt.Errorf("invalid artifact descriptor")
	}
	defer func() {
		_, _ = eval("globalThis.__babArtifacts?.delete(args.key); return null;", map[string]any{"key": artifact.Artifact.Key})
	}()
	f, e := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	success := false
	defer func() {
		f.Close()
		if !success {
			os.Remove(output)
		}
	}()
	for offset := 0; offset < artifact.Artifact.Size; {
		n := 192 * 1024
		if n > artifact.Artifact.Size-offset {
			n = artifact.Artifact.Size - offset
		}
		raw, e := eval(`const bytes=globalThis.__babArtifacts?.get(args.key); if (!bytes) throw new Error('missing'); const slice=bytes.subarray(args.offset,args.offset+args.length); let s=''; for (let i=0;i<slice.length;i+=8192) s+=String.fromCharCode(...slice.subarray(i,i+8192)); return btoa(s);`, map[string]any{"key": artifact.Artifact.Key, "offset": offset, "length": n})
		if e != nil {
			return e
		}
		var encoded string
		if json.Unmarshal(raw, &encoded) != nil {
			return fmt.Errorf("invalid artifact chunk")
		}
		data, e := base64.StdEncoding.DecodeString(encoded)
		if e != nil || len(data) != n {
			return fmt.Errorf("invalid artifact chunk length")
		}
		if _, e = f.Write(data); e != nil {
			return e
		}
		offset += n
	}
	if e = f.Close(); e != nil {
		return e
	}
	success = true
	return printJSON(map[string]any{"output": output, "bytes": artifact.Artifact.Size})
}

func delegate(p plugins.Plugin, args []string) error {
	if err := requireEnabled(p); err != nil {
		return err
	}
	executable, e := plugins.Executable(p.Delegate.Executable)
	if e != nil {
		return fmt.Errorf("%s is registered but its executable is unavailable: install it or update the plugin", p.Site)
	}
	cmd := exec.Command(executable, args...)
	if p.Delegate.OriginEnv != "" {
		cmd.Env = append(os.Environ(), p.Delegate.OriginEnv+"="+p.Origin)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Registers only declared local consumers against the already selected extension.
// No plugin hooks are executed. Missing consumers stay visible as unavailable.
func registerDelegates(id string) error {
	ps, e := all()
	if e != nil {
		return e
	}
	type entry struct{ host, path string }
	entries := []entry{}
	seen := map[string]bool{}
	for _, p := range ps {
		if p.Delegate == nil || p.Delegate.NativeHost == "" {
			continue
		}
		name := p.Delegate.NativeHost
		if seen[name] {
			return fmt.Errorf("duplicate native host in plugin registry")
		}
		seen[name] = true
		executable, err := plugins.Executable(p.Delegate.Executable)
		if err != nil {
			continue
		}
		executable, err = filepath.Abs(executable)
		if err != nil {
			return err
		}
		if p.Delegate.OriginEnv != "" {
			wrapper, err := writeConsumerWrapper(p.Site)
			if err != nil {
				return err
			}
			executable = wrapper
		}
		entries = append(entries, entry{name, executable})
	}
	for _, entry := range entries {
		path := filepath.Join(filepath.Dir(registration()), entry.host+".json")
		data, _ := json.MarshalIndent(map[string]any{"name": entry.host, "description": "BAB registered CLI", "path": entry.path, "type": "stdio", "allowed_origins": []string{"chrome-extension://" + id + "/"}}, "", "  ")
		if e = os.WriteFile(path, data, 0600); e != nil {
			return e
		}
	}
	return nil
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func writeConsumerWrapper(site string) (string, error) {
	if !plugins.Identifier.MatchString(site) {
		return "", fmt.Errorf("invalid native consumer")
	}
	executable, e := os.Executable()
	if e != nil {
		return "", e
	}
	directory := filepath.Join(home(), "hosts")
	if e = os.MkdirAll(directory, 0700); e != nil {
		return "", e
	}
	path := filepath.Join(directory, site)
	script := "#!/bin/sh\nexec " + shellQuote(executable) + " native-consumer " + shellQuote(site) + " \"$@\"\n"
	if e = os.WriteFile(path, []byte(script), 0700); e != nil {
		return "", e
	}
	return path, nil
}
