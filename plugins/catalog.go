package plugins

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed catalog.json builtin
var builtins embed.FS

type CatalogEntry struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Path           string `json:"path"`
	OriginRequired bool   `json:"originRequired,omitempty"`
	Builtin        bool   `json:"-"`
}
type SourceCatalog struct {
	Version int            `json:"version"`
	Plugins []CatalogEntry `json:"plugins"`
}

// Available merges the distributed public catalog with explicitly configured
// local catalogs. A catalog path may point at a checked-out Git submodule.
// This function never clones repositories or reads the installed plugin registry.
func Available(paths []string) ([]CatalogEntry, error) {
	data, err := builtins.ReadFile("catalog.json")
	if err != nil {
		return nil, err
	}
	var base SourceCatalog
	if err = Decode(data, &base); err != nil {
		return nil, err
	}
	out := []CatalogEntry{}
	seen := map[string]bool{}
	add := func(c SourceCatalog, directory string, builtin bool) error {
		if c.Version != 1 || len(c.Plugins) > 100 {
			return fmt.Errorf("invalid source catalog")
		}
		for _, entry := range c.Plugins {
			if !Identifier.MatchString(entry.Name) || reserved[entry.Name] || entry.Path == "" || len(entry.Description) > 256 || seen[entry.Name] {
				return fmt.Errorf("invalid or duplicate catalog entry")
			}
			entry.Builtin = builtin
			if builtin {
				if entry.Path != "builtin/"+entry.Name {
					return fmt.Errorf("invalid builtin path")
				}
			} else if !filepath.IsAbs(entry.Path) {
				entry.Path = filepath.Join(directory, entry.Path)
			}
			seen[entry.Name] = true
			out = append(out, entry)
		}
		return nil
	}
	if err = add(base, "", true); err != nil {
		return nil, err
	}
	for _, path := range paths {
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read configured catalog: %s", path)
		}
		var c SourceCatalog
		if err = Decode(data, &c); err != nil {
			return nil, err
		}
		if err = add(c, filepath.Dir(path), false); err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func ResolveSource(name string, paths []string) (CatalogEntry, error) {
	entries, err := Available(paths)
	if err != nil {
		return CatalogEntry{}, err
	}
	for _, entry := range entries {
		if entry.Name == name {
			return entry, nil
		}
	}
	return CatalogEntry{}, fmt.Errorf("unknown plugin %s; run bab plugin available or provide an explicit local path", name)
}

// MaterializeBuiltin writes only distributed, trusted source into a temporary
// directory. Caller removes it after installing its own configured snapshot.
func MaterializeBuiltin(entry CatalogEntry) (string, func(), error) {
	if !entry.Builtin || entry.Path != "builtin/"+entry.Name {
		return "", func() {}, fmt.Errorf("not a builtin entry")
	}
	data, err := builtins.ReadFile(entry.Path + "/bab-plugin.json")
	if err != nil {
		return "", func() {}, err
	}
	directory, err := os.MkdirTemp("", "bab-source-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	if err = os.WriteFile(filepath.Join(directory, "bab-plugin.json"), data, 0600); err != nil {
		cleanup()
		return "", func() {}, err
	}

	var manifest Manifest
	if err = Decode(data, &manifest); err != nil {
		cleanup()
		return "", func() {}, err
	}
	names := map[string]bool{}
	for _, command := range manifest.Commands {
		names[command.Script] = true
	}
	if manifest.Extension != nil {
		for _, name := range manifest.Extension.Files {
			names[name] = true
		}
	}
	for name := range names {
		if name == "" || filepath.IsAbs(name) || filepath.Clean(name) != name || strings.HasPrefix(name, "..") {
			cleanup()
			return "", func() {}, fmt.Errorf("invalid bundled asset")
		}
		data, err := builtins.ReadFile(entry.Path + "/" + filepath.ToSlash(name))
		if err != nil {
			cleanup()
			return "", func() {}, err
		}
		path := filepath.Join(directory, name)
		if err = os.MkdirAll(filepath.Dir(path), 0700); err == nil {
			err = os.WriteFile(path, data, 0600)
		}
		if err != nil {
			cleanup()
			return "", func() {}, err
		}
	}
	if _, err = Load(directory); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return directory, cleanup, nil
}

// BuiltinManifest supplies static example metadata without materializing files,
// executing an adapter, reading a private catalog, or contacting a website.
func BuiltinManifest(name string) (Manifest, error) {
	var manifest Manifest
	if !Identifier.MatchString(name) {
		return manifest, fmt.Errorf("invalid builtin name")
	}
	data, err := builtins.ReadFile("builtin/" + name + "/bab-plugin.json")
	if err != nil {
		return manifest, err
	}
	if err = Decode(data, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}
