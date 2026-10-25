package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	assets "github.com/sky-bro/browser-auth-bridge/internal/extension"
	"github.com/sky-bro/browser-auth-bridge/plugins"
)

func catalogPaths() ([]string, error) {
	data, err := os.ReadFile(filepath.Join(home(), "catalogs.json"))
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	if plugins.Decode(data, &paths) != nil {
		return nil, fmt.Errorf("invalid local catalog configuration")
	}
	return paths, nil
}
func sourceCatalogCommand(args []string) error {
	if len(args) == 1 && args[0] == "list" {
		paths, e := catalogPaths()
		if e != nil {
			return e
		}
		return printJSON(paths)
	}
	if len(args) != 2 || (args[0] != "add" && args[0] != "remove") {
		return fmt.Errorf("usage: bab plugin catalog add|remove PATH | list")
	}
	path, e := filepath.Abs(args[1])
	if e != nil {
		return e
	}
	paths, e := catalogPaths()
	if e != nil {
		return e
	}
	next := []string{}
	for _, p := range paths {
		if p != path {
			next = append(next, p)
		}
	}
	if args[0] == "add" {
		next = append(next, path)
		if _, e = plugins.Available(next); e != nil {
			return e
		}
	}
	data, _ := json.MarshalIndent(next, "", "  ")
	if e = os.MkdirAll(home(), 0700); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(home(), "catalogs.json"), data, 0600); e != nil {
		return e
	}
	return printJSON(map[string]any{"catalogs": next})
}
func availablePlugins(args []string) error {
	asJSON := len(args) == 1 && args[0] == "--json"
	if len(args) == 1 && args[0] == "--help" {
		fmt.Println("Usage: bab plugin available [--json]")
		return nil
	}
	if len(args) > 0 && !asJSON {
		return fmt.Errorf("usage: bab plugin available [--json]")
	}
	paths, e := catalogPaths()
	if e != nil {
		return e
	}
	entries, e := plugins.Available(paths)
	if e != nil {
		return e
	}
	installed, e := all()
	if e != nil {
		return e
	}
	present := map[string]plugins.Plugin{}
	for _, p := range installed {
		present[p.Site] = p
	}
	out := []map[string]any{}
	for _, entry := range entries {
		ready := entry.Builtin
		if !ready {
			_, e = plugins.Load(entry.Path)
			ready = e == nil
		}
		source := entry.Path
		if entry.Builtin {
			source = "bundled"
		}
		p, exists := present[entry.Name]
		out = append(out, map[string]any{"name": entry.Name, "description": entry.Description, "installed": exists, "enabled": exists && !p.Disabled, "sourceReady": ready, "originRequired": entry.OriginRequired, "source": source})
	}
	if asJSON {
		return printJSON(out)
	}
	table := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "PLUGIN\tSTATE\tSOURCE\tDESCRIPTION")
	for _, item := range out {
		state := "not installed"
		if item["installed"].(bool) {
			state = "enabled"
			if !item["enabled"].(bool) {
				state = "disabled"
			}
		}
		if !item["sourceReady"].(bool) {
			state += " (source unavailable)"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", item["name"], state, terminalText(item["source"].(string)), terminalText(item["description"].(string)))
	}
	return table.Flush()
}
func installSelected(operation string, args []string) error {
	names := []string{}
	origin := ""
	installDisabled := false
	for i := 0; i < len(args); i++ {
		if args[i] == "--disabled" {
			installDisabled = true
		} else if args[i] == "--origin" {
			i++
			if i >= len(args) {
				return fmt.Errorf("--origin requires a URL")
			}
			origin = args[i]
		} else if strings.HasPrefix(args[i], "--origin=") {
			origin = strings.TrimPrefix(args[i], "--origin=")
		} else if strings.HasPrefix(args[i], "--") {
			return fmt.Errorf("unknown installation option")
		} else {
			names = append(names, args[i])
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("usage: bab plugin %s NAME|PATH... [--origin URL]", operation)
	}
	if origin != "" && (len(names) != 1 || !plugins.ValidOrigin(origin)) {
		return fmt.Errorf("--origin requires one selected plugin and an exact HTTPS origin")
	}
	paths, e := catalogPaths()
	if e != nil {
		return e
	}
	// Validate the complete selection before installing any package. This also
	// rejects an uninitialized submodule before changing the installed registry.
	type selected struct {
		path, origin string
		cleanup      func()
	}
	selection := []selected{}
	defer func() {
		for _, s := range selection {
			s.cleanup()
		}
	}()
	browserComponentsChanged := false
	sites := map[string]bool{}
	prospective := map[string]plugins.Plugin{}
	existing, err := all()
	if err != nil {
		return err
	}
	for _, p := range existing {
		prospective[p.Site] = p
	}
	for _, name := range names {
		path := name
		required := false
		cleanup := func() {}
		if plugins.Identifier.MatchString(name) {
			entry, err := plugins.ResolveSource(name, paths)
			if err != nil {
				return err
			}
			path = entry.Path
			required = entry.OriginRequired
			if entry.Builtin {
				path, cleanup, err = plugins.MaterializeBuiltin(entry)
				if err != nil {
					return err
				}
			}
		}
		p, err := plugins.Load(path)
		if err != nil {
			cleanup()
			return fmt.Errorf("plugin source unavailable; initialize its submodule or check its local path: %w", err)
		}
		if p.Extension != nil || prospective[p.Site].Extension != nil {
			browserComponentsChanged = true
		}
		configuredOrigin := origin
		if required && configuredOrigin == "" && operation == "update" {
			if previous, e := find(p.Site); e == nil {
				configuredOrigin = previous.Origin
			}
		}
		if required && configuredOrigin == "" {
			cleanup()
			return fmt.Errorf("%s requires --origin https://your-site.example", name)
		}
		if sites[p.Site] {
			cleanup()
			return fmt.Errorf("duplicate selected namespace")
		}
		sites[p.Site] = true
		exists := false
		if _, err = os.Stat(filepath.Join(root(), p.Site)); err == nil {
			exists = true
		} else if !os.IsNotExist(err) {
			cleanup()
			return err
		}
		if exists != (operation == "update") {
			cleanup()
			return fmt.Errorf("%s: use plugin %s", p.Site, map[bool]string{true: "update", false: "install"}[exists])
		}
		selection = append(selection, selected{path, configuredOrigin, cleanup})
		if configuredOrigin != "" {
			p.Origin = configuredOrigin
		}
		prospective[p.Site] = p
	}
	previews := []plugins.Plugin{}
	for _, p := range prospective {
		previews = append(previews, p)
	}
	if len(previews) > 100 {
		return fmt.Errorf("at most 100 plugins can be installed")
	}
	if _, err := assets.Render(previews); err != nil {
		return err
	}
	// Save an explicitly disabled selection before publishing any package.
	// A partial filesystem failure must never activate a requested-off plugin.
	if installDisabled {
		if err := changePluginState(func(state map[string]bool) {
			for site := range sites {
				state[site] = true
			}
		}); err != nil {
			return err
		}
	}
	for _, s := range selection {
		p, err := plugins.InstallConfigured(root(), s.path, operation == "update", s.origin)
		if err != nil {
			return err
		}
		fmt.Printf("Registered %s.\n", p.Site)
	}
	if browserComponentsChanged {
		if err := rebuildExtension(); err != nil {
			return err
		}
	}
	if id, err := extensionID(); err == nil {
		if err = registerDelegates(id); err != nil {
			return err
		}
	}
	if err := syncCatalog(); err != nil {
		fmt.Fprintln(os.Stderr, "Installed locally. Reconnect the extension to refresh its registered commands.")
	}
	return nil
}
