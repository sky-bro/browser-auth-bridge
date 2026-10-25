// Package extension assembles browser components declared by installed plugins.
package extension

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sky-bro/browser-auth-bridge/plugins"
)

//go:embed browser-exec.js bab-background.js bab.html bab.js background.js config.js site-config.js services.js stream-transport.js pty-transport.js pty-content.js manifest.json permissions.html
var babAssets embed.FS
var coreFiles = []string{"browser-exec.js", "bab-background.js", "bab.html", "bab.js", "background.js", "config.js", "site-config.js", "services.js", "stream-transport.js", "pty-transport.js", "pty-content.js", "manifest.json", "permissions.html"}

type component struct {
	ID         string   `json:"id"`
	Origins    []string `json:"origins"`
	Files      []string `json:"files"`
	Background []string `json:"background"`
}

// Render contains no consumer-specific constants. Browser code is copied only
// from explicit installed snapshots, never executed during assembly.
func Render(installed []plugins.Plugin) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, name := range coreFiles {
		b, err := babAssets.ReadFile(name)
		if err != nil {
			return nil, err
		}
		out[name] = b
	}
	packages := []component{}
	providers := []map[string]any{}
	seen := map[string]bool{}
	hosts := map[string]bool{}
	ordered := append([]plugins.Plugin{}, installed...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Site < ordered[j].Site })
	for _, p := range ordered {
		if !plugins.Identifier.MatchString(p.Site) || seen[p.Site] || !plugins.ValidOrigin(p.Origin) {
			return nil, fmt.Errorf("invalid or duplicate plugin scope")
		}
		seen[p.Site] = true
		if p.Delegate != nil && p.Delegate.NativeHost != "" {
			if hosts[p.Delegate.NativeHost] {
				return nil, fmt.Errorf("duplicate native host")
			}
			hosts[p.Delegate.NativeHost] = true
		}
		assets, err := p.BrowserAssets()
		if err != nil {
			return nil, err
		}
		if p.Extension == nil {
			continue
		}
		prefix := "plugins/" + p.Site + "/"
		files := []string{}
		for name, data := range assets {
			out[prefix+name] = data
			files = append(files, prefix+name)
		}
		sort.Strings(files)
		background := []string{}
		for _, name := range p.Extension.Background {
			background = append(background, prefix+name)
		}
		packages = append(packages, component{p.Site, append([]string{p.Origin}, p.Origins...), files, background})
		if p.Extension.PTYAdapter != "" {
			scripts := []string{}
			for _, name := range p.Extension.PTYScripts {
				scripts = append(scripts, prefix+name)
			}
			// Origin arrives from the native registry, never an implicit deployment URL.
			providers = append(providers, map[string]any{"id": p.Site, "label": p.Description, "nativeHost": p.Delegate.NativeHost, "editableOrigin": true, "registryManaged": true, "ptyAdapter": p.Extension.PTYAdapter, "ptyScripts": scripts})
		}
	}
	p, _ := json.Marshal(packages)
	v, _ := json.Marshal(providers)
	out["config.js"] = []byte("// Generated from installed plugin snapshots. Reload after component changes.\nglobalThis.BridgeProviders = Object.freeze(" + string(v) + ");\nglobalThis.BridgePluginPackages = Object.freeze(" + string(p) + ");\n")
	return out, nil
}

func InstallBAB(directory string) error { return InstallPlugins(directory, nil) }

// InstallPlugins replaces only BAB-owned generated directories. Invalid input
// leaves the previous installation intact, and omitted plugins leave no assets.
func InstallPlugins(directory string, installed []plugins.Plugin) error {
	files, err := Render(installed)
	if err != nil {
		return err
	}
	exists := false
	if info, e := os.Lstat(directory); e == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("extension directory must be a regular directory")
		}
		exists = true
		entries, e := os.ReadDir(directory)
		if e != nil {
			return e
		}
		if len(entries) > 0 {
			if _, e = os.Stat(filepath.Join(directory, ".bab-extension")); e != nil {
				// Migrate only the exact old BAB-generated asset set; preserve arbitrary directories.
				known := map[string]bool{}
				for _, name := range coreFiles {
					known[name] = true
				}
				known["adapters"] = true
				var manifest struct {
					Name string `json:"name"`
				}
				b, _ := os.ReadFile(filepath.Join(directory, "manifest.json"))
				if json.Unmarshal(b, &manifest) != nil || manifest.Name != "Browser Auth Bridge — BAB" {
					return fmt.Errorf("refusing to replace an unmarked extension directory")
				}
				for _, entry := range entries {
					if !known[entry.Name()] {
						return fmt.Errorf("unrecognized file in old extension directory")
					}
					if entry.IsDir() {
						nested, _ := os.ReadDir(filepath.Join(directory, entry.Name()))
						if entry.Name() != "adapters" || len(nested) != 1 || nested[0].Name() != "ttyd.js" {
							return fmt.Errorf("unrecognized old adapter directory")
						}
					}
				}
			}
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	parent := filepath.Dir(directory)
	if err = os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(parent, ".bab-extension-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	for name, data := range files {
		path := filepath.Join(temp, name)
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			return err
		}
	}
	if err = os.WriteFile(filepath.Join(temp, ".bab-extension"), []byte("1\n"), 0600); err != nil {
		return err
	}
	backup := ""
	if exists {
		backup, err = os.MkdirTemp(parent, ".bab-previous-")
		if err != nil {
			return err
		}
		os.Remove(backup)
		if err = os.Rename(directory, backup); err != nil {
			return err
		}
	}
	if err = os.Rename(temp, directory); err != nil {
		if backup != "" {
			_ = os.Rename(backup, directory)
		}
		return err
	}
	if backup != "" {
		return os.RemoveAll(backup)
	}
	return nil
}
