package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/sky-bro/browser-auth-bridge/plugins"
)

// State is user-owned configuration, separate from installable manifests.
func disabledPlugins() (map[string]bool, error) {
	out := map[string]bool{}
	data, err := os.ReadFile(filepath.Join(home(), "disabled.json"))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	if plugins.Decode(data, &names) != nil || len(names) > 1000 {
		return nil, fmt.Errorf("invalid plugin state")
	}
	for _, name := range names {
		if !plugins.Identifier.MatchString(name) || out[name] {
			return nil, fmt.Errorf("invalid plugin state")
		}
		out[name] = true
	}
	return out, nil
}
func changePluginState(change func(map[string]bool)) error {
	if err := os.MkdirAll(home(), 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(home(), ".plugin-state.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	state, err := disabledPlugins()
	if err != nil {
		return err
	}
	change(state)
	names := []string{}
	for name, disabled := range state {
		if disabled {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	data, err := json.Marshal(names)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(home(), ".plugin-state-")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(append(data, '\n')); err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(temp.Name(), filepath.Join(home(), "disabled.json"))
}
func pluginStateCommand(action, site string) error {
	if _, err := find(site); err != nil {
		return err
	}
	disabled := action == "disable"
	if err := changePluginState(func(state map[string]bool) {
		if disabled {
			state[site] = true
		} else {
			delete(state, site)
		}
	}); err != nil {
		return err
	}
	if !disabled {
		if id, err := extensionID(); err == nil {
			if err = registerDelegates(id); err != nil {
				return err
			}
		}
	}
	if err := syncCatalog(); err != nil {
		fmt.Fprintln(os.Stderr, "Saved locally. Reconnect or reload BAB in Chrome to apply the browser state.")
	}
	return printJSON(map[string]any{"site": site, "enabled": !disabled})
}
func requireEnabled(p plugins.Plugin) error {
	if p.Disabled {
		return fmt.Errorf("%s is disabled; run bab plugin enable %s", p.Site, p.Site)
	}
	return nil
}
