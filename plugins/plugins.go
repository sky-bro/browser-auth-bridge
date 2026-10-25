// Package plugins implements the versioned, local BAB browser-plugin contract.
// Plugins contain trusted page scripts, never native executables or install hooks.
package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var Identifier = regexp.MustCompile(`^[a-z][a-z0-9-]{0,47}$`)
var reserved = map[string]bool{"bab": true, "completion": true, "extension": true, "auth": true, "plugin": true, "setup": true, "doctor": true, "list": true, "native-host": true, "help": true, "version": true}

type Argument struct {
	Name     string   `json:"name"`
	Required bool     `json:"required,omitempty"`
	Default  string   `json:"default,omitempty"`
	Choices  []string `json:"choices,omitempty"`
}
type Command struct {
	Description string     `json:"description"`
	Script      string     `json:"script"`
	Access      string     `json:"access"`
	Positional  []Argument `json:"positional,omitempty"`
	Options     []Argument `json:"options,omitempty"`
	Artifact    bool       `json:"artifact,omitempty"`
}

// Delegate registers an existing local CLI under a BAB site namespace.
// No executable or installation hook runs during package installation.
type Delegate struct {
	Executable string `json:"executable"`
	NativeHost string `json:"nativeHost,omitempty"`
	OriginEnv  string `json:"originEnv,omitempty"`
}

// CompletionSpec describes delegated CLI syntax without invoking that CLI.
type CompletionSpec struct {
	Description string                    `json:"description,omitempty"`
	Commands    map[string]CompletionSpec `json:"commands,omitempty"`
	Positional  []Argument                `json:"positional,omitempty"`
	Options     []Argument                `json:"options,omitempty"`
}

// BrowserExtension declares consumer-owned scripts packaged at build time.
// All output paths are namespaced by the plugin ID; no remote code is loaded.
type BrowserExtension struct {
	Files      []string `json:"files"`
	Background []string `json:"background,omitempty"`
	PTYAdapter string   `json:"ptyAdapter,omitempty"`
	PTYScripts []string `json:"ptyScripts,omitempty"`
}

type Manifest struct {
	Extension   *BrowserExtension  `json:"extension,omitempty"`
	Version     int                `json:"version"`
	Site        string             `json:"site"`
	Origin      string             `json:"origin"`
	Origins     []string           `json:"origins,omitempty"`
	Delegate    *Delegate          `json:"delegate,omitempty"`
	Completion  *CompletionSpec    `json:"completion,omitempty"`
	Description string             `json:"description"`
	Commands    map[string]Command `json:"commands"`
}
type Plugin struct {
	Manifest
	Directory string `json:"-"`
	Disabled  bool   `json:"disabled,omitempty"`
}

// Site is the only plugin metadata sent to the extension. Scripts remain CLI-owned.
type Site struct {
	ID        string   `json:"id"`
	Origin    string   `json:"origin"`
	Label     string   `json:"label"`
	Origins   []string `json:"origins,omitempty"`
	Available *bool    `json:"available,omitempty"`
	Disabled  bool     `json:"disabled,omitempty"`
}

func Decode(data []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return fmt.Errorf("invalid JSON schema")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
func ValidOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.User != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.Opaque != "" || strings.ContainsAny(origin, "*\\% \t\r\n") || u.Scheme+"://"+u.Host != origin {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
}
func file(root, name string) ([]byte, error) {
	if name == "" || filepath.IsAbs(name) || filepath.Clean(name) != name || strings.HasPrefix(name, "..") {
		return nil, fmt.Errorf("invalid plugin path")
	}
	p := filepath.Join(root, name)
	info, err := os.Lstat(p)
	if err != nil {
		return nil, fmt.Errorf("missing plugin file: %s", name)
	}
	if !info.Mode().IsRegular() || info.Size() > 256*1024 {
		return nil, fmt.Errorf("plugin file must be a bounded regular file")
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return nil, err
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(base, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("plugin file escapes package")
	}
	return os.ReadFile(p)
}
func Load(directory string) (Plugin, error) {
	var p Plugin
	p.Directory = directory
	data, err := file(directory, "bab-plugin.json")
	if err != nil {
		return p, err
	}
	if err = Decode(data, &p.Manifest); err != nil {
		return p, err
	}
	if p.Version != 1 || !Identifier.MatchString(p.Site) || reserved[p.Site] || !ValidOrigin(p.Origin) || (len(p.Commands) == 0 && p.Delegate == nil) || len(p.Commands) > 100 || len(p.Description) > 256 {
		return p, fmt.Errorf("invalid plugin manifest")
	}
	if p.Delegate != nil {
		d := p.Delegate
		if len(p.Commands) != 0 || d.Executable == "" || strings.ContainsAny(d.Executable, "\x00\r\n") || len(d.Executable) > 4096 || (d.NativeHost != "" && !regexp.MustCompile(`^[a-z0-9_]+(?:\.[a-z0-9_]+)+$`).MatchString(d.NativeHost)) || d.NativeHost == "org.browserauthbridge.bab" || (d.OriginEnv != "" && !regexp.MustCompile(`^[A-Z][A-Z0-9_]*_ORIGIN$`).MatchString(d.OriginEnv)) {
			return p, fmt.Errorf("invalid CLI delegate")
		}
	}
	if err := p.validateExtension(); err != nil {
		return p, err
	}
	if p.Completion != nil {
		nodes := 0
		if p.Delegate == nil || validateCompletion(*p.Completion, 0, &nodes) != nil {
			return p, fmt.Errorf("invalid delegated completion metadata")
		}
	}
	seenOrigins := map[string]bool{p.Origin: true}
	for _, origin := range p.Origins {
		if !ValidOrigin(origin) || seenOrigins[origin] {
			return p, fmt.Errorf("invalid additional origin")
		}
		seenOrigins[origin] = true
	}
	if len(p.Origins) > 8 {
		return p, fmt.Errorf("too many plugin origins")
	}
	for name, c := range p.Commands {
		if !Identifier.MatchString(name) || (c.Access != "read" && c.Access != "write") || !strings.HasSuffix(c.Script, ".js") || len(c.Description) > 512 {
			return p, fmt.Errorf("invalid command: %s", name)
		}
		if _, err = file(directory, c.Script); err != nil {
			return p, err
		}
		seen := map[string]bool{}
		for _, a := range append(append([]Argument{}, c.Positional...), c.Options...) {
			if !Identifier.MatchString(a.Name) || a.Name == "json" || a.Name == "output" || a.Name == "help" || seen[a.Name] {
				return p, fmt.Errorf("invalid or duplicate argument")
			}
			seen[a.Name] = true
			if len(a.Choices) > 0 && a.Default != "" && !contains(a.Choices, a.Default) {
				return p, fmt.Errorf("invalid argument default")
			}
		}
	}
	return p, nil
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
func (p Plugin) Script(command string) (string, error) {
	c, ok := p.Commands[command]
	if !ok {
		return "", fmt.Errorf("unknown command")
	}
	b, e := file(p.Directory, c.Script)
	return string(b), e
}
func All(root string) ([]Plugin, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []Plugin{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Plugin
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if !e.IsDir() {
			return nil, fmt.Errorf("unexpected plugin entry")
		}
		p, err := Load(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("plugin %s: %w", e.Name(), err)
		}
		if p.Site != e.Name() {
			return nil, fmt.Errorf("plugin directory/site mismatch")
		}
		out = append(out, p)
	}
	return out, nil
}
func Catalog(ps []Plugin) []Site {
	out := []Site{}
	for _, p := range ps {
		available := true
		if p.Delegate != nil {
			_, err := Executable(p.Delegate.Executable)
			available = err == nil
		}
		out = append(out, Site{ID: p.Site, Origin: p.Origin, Label: p.Description, Origins: p.Origins, Available: &available, Disabled: p.Disabled})
	}
	return out
}

// Install copies only manifest and referenced scripts; source edits take effect on explicit update.
// Installation stages complete files and keeps a rollback backup. Directory
// replacement has a brief reader gap; installation never runs plugin code.
func Install(root, source string, update bool) (Plugin, error) {
	return InstallConfigured(root, source, update, "")
}
func InstallConfigured(root, source string, update bool, origin string) (Plugin, error) {
	p, err := Load(source)
	if err != nil {
		return p, err
	}
	if origin != "" {
		if !ValidOrigin(origin) {
			return p, fmt.Errorf("invalid configured origin")
		}
		p.Origin = origin
	}
	if p.Delegate != nil {
		if executable, e := Executable(p.Delegate.Executable); e == nil {
			absolute, e := filepath.Abs(executable)
			if e != nil {
				return p, e
			}
			p.Delegate.Executable = absolute
		}
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return p, err
	}
	target := filepath.Join(root, p.Site)
	if info, e := os.Lstat(target); e == nil {
		if !update || !info.IsDir() {
			return p, fmt.Errorf("already installed; use plugin update PATH")
		}
	} else if !os.IsNotExist(e) {
		return p, e
	} else if update {
		return p, fmt.Errorf("plugin is not installed")
	}
	temp, err := os.MkdirTemp(root, ".install-")
	if err != nil {
		return p, err
	}
	defer os.RemoveAll(temp)
	names := map[string]bool{"bab-plugin.json": true}
	for _, c := range p.Commands {
		names[c.Script] = true
	}
	if p.Extension != nil {
		for _, name := range p.Extension.Files {
			names[name] = true
		}
	}
	for name := range names {
		b, e := file(source, name)
		if e != nil {
			return p, e
		}
		if name == "bab-plugin.json" {
			b, e = json.MarshalIndent(p.Manifest, "", "  ")
			if e != nil {
				return p, e
			}
			b = append(b, '\n')
		}
		dest := filepath.Join(temp, name)
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return p, e
		}
		if e = os.WriteFile(dest, b, 0600); e != nil {
			return p, e
		}
	}
	// os.Rename cannot atomically replace a nonempty directory. Keep the old package
	// for rollback if the second rename fails; no native host persists loaded scripts.
	backup := ""
	if update {
		b, e := os.MkdirTemp(root, ".previous-")
		if e != nil {
			return p, e
		}
		os.Remove(b)
		backup = b
		if e = os.Rename(target, backup); e != nil {
			return p, e
		}
	}
	if err = os.Rename(temp, target); err != nil {
		if backup != "" {
			_ = os.Rename(backup, target)
		}
		return p, err
	}
	if backup != "" {
		_ = os.RemoveAll(backup)
	}
	p.Directory = target
	return p, nil
}
func Parse(c Command, tokens []string) (map[string]string, string, error) {
	out := map[string]string{}
	options := map[string]Argument{}
	for _, a := range c.Options {
		options[a.Name] = a
		out[a.Name] = a.Default
	}
	positionals := []string{}
	output := ""
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t == "--" {
			positionals = append(positionals, tokens[i+1:]...)
			break
		}
		if t == "--json" {
			continue
		}
		if strings.HasPrefix(t, "--") {
			name, value, has := strings.Cut(strings.TrimPrefix(t, "--"), "=")
			if name != "output" {
				if _, ok := options[name]; !ok {
					return nil, "", fmt.Errorf("unknown option --%s", name)
				}
			}
			if !has {
				i++
				if i >= len(tokens) || strings.HasPrefix(tokens[i], "--") {
					return nil, "", fmt.Errorf("--%s requires a value", name)
				}
				value = tokens[i]
			}
			if name == "output" {
				output = value
			} else {
				out[name] = value
			}
		} else {
			positionals = append(positionals, t)
		}
	}
	if len(positionals) > len(c.Positional) {
		return nil, "", fmt.Errorf("too many positional arguments")
	}
	for i, a := range c.Positional {
		out[a.Name] = a.Default
		if i < len(positionals) {
			out[a.Name] = positionals[i]
		}
	}
	for _, a := range append(append([]Argument{}, c.Positional...), c.Options...) {
		v := out[a.Name]
		if a.Required && v == "" {
			return nil, "", fmt.Errorf("missing argument: %s", a.Name)
		}
		if v != "" && len(a.Choices) > 0 && !contains(a.Choices, v) {
			return nil, "", fmt.Errorf("invalid value for %s", a.Name)
		}
	}
	if output != "" && !c.Artifact {
		return nil, "", fmt.Errorf("command does not produce an artifact")
	}
	if c.Artifact && output == "" {
		return nil, "", fmt.Errorf("artifact command requires --output PATH")
	}
	return out, output, nil
}
func Names(p Plugin) []string {
	out := []string{}
	for n := range p.Commands {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Executable resolves an explicitly registered CLI, without a shell or evaluation.
func Executable(name string) (string, error) {
	if strings.HasPrefix(name, "~/") {
		h, e := os.UserHomeDir()
		if e != nil {
			return "", e
		}
		name = filepath.Join(h, name[2:])
	}
	return exec.LookPath(name)
}

func validateCompletion(spec CompletionSpec, depth int, nodes *int) error {
	*nodes++
	if depth > 8 || *nodes > 256 || len(spec.Description) > 512 || len(spec.Options) > 100 || len(spec.Positional) > 20 {
		return fmt.Errorf("completion metadata limit exceeded")
	}
	seen := map[string]bool{}
	for _, arg := range append(append([]Argument{}, spec.Positional...), spec.Options...) {
		if !Identifier.MatchString(arg.Name) || seen[arg.Name] || len(arg.Choices) > 100 {
			return fmt.Errorf("invalid completion argument")
		}
		seen[arg.Name] = true
	}
	for name, child := range spec.Commands {
		if !Identifier.MatchString(name) {
			return fmt.Errorf("invalid completion command")
		}
		if err := validateCompletion(child, depth+1, nodes); err != nil {
			return err
		}
	}
	return nil
}

var extensionPath = regexp.MustCompile(`^[a-zA-Z0-9_/-]+\.js$`)

func (p Plugin) validateExtension() error {
	e := p.Extension
	if e == nil {
		return nil
	}
	if len(e.Files) == 0 || len(e.Files) > 64 || len(e.Background) > 16 || len(e.PTYScripts) > 16 {
		return fmt.Errorf("invalid browser component files")
	}
	files := map[string]bool{}
	for _, name := range e.Files {
		if !extensionPath.MatchString(name) || strings.Contains(name, "..") || filepath.IsAbs(name) || filepath.Clean(name) != name || files[name] {
			return fmt.Errorf("invalid browser component path")
		}
		if _, err := file(p.Directory, name); err != nil {
			return err
		}
		files[name] = true
	}
	for _, group := range [][]string{e.Background, e.PTYScripts} {
		seen := map[string]bool{}
		for _, name := range group {
			if !files[name] || seen[name] {
				return fmt.Errorf("browser entrypoint is not a unique packaged file")
			}
			seen[name] = true
		}
	}
	if (e.PTYAdapter != "") != (len(e.PTYScripts) > 0) || (e.PTYAdapter != "" && (!Identifier.MatchString(e.PTYAdapter) || p.Delegate == nil || p.Delegate.NativeHost == "")) {
		return fmt.Errorf("invalid browser PTY adapter")
	}
	return nil
}

// BrowserAssets reads only explicitly declared, validated package files.
func (p Plugin) BrowserAssets() (map[string][]byte, error) {
	out := map[string][]byte{}
	if err := p.validateExtension(); err != nil {
		return nil, err
	}
	if p.Extension != nil {
		for _, name := range p.Extension.Files {
			b, err := file(p.Directory, name)
			if err != nil {
				return nil, err
			}
			out[name] = b
		}
	}
	return out, nil
}
