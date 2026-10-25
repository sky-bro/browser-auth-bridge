package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sky-bro/browser-auth-bridge/plugins"
)

func initPlugin(args []string) error {
	usage := "bab plugin init NAME --origin URL [--dir PATH] [--executable PATH] [--native-host NAME]"
	if len(args) == 1 && args[0] == "--help" {
		fmt.Println(usage)
		return nil
	}
	if len(args) == 0 || !plugins.Identifier.MatchString(args[0]) {
		return fmt.Errorf("usage: %s", usage)
	}
	name := args[0]
	values := map[string]string{}
	for i := 1; i < len(args); i++ {
		key, value, has := strings.Cut(args[i], "=")
		if key != "--origin" && key != "--dir" && key != "--executable" && key != "--native-host" {
			return fmt.Errorf("unknown scaffold option: %s", key)
		}
		if _, ok := values[key]; ok {
			return fmt.Errorf("duplicate scaffold option: %s", key)
		}
		if !has {
			i++
			if i >= len(args) || strings.HasPrefix(args[i], "--") {
				return fmt.Errorf("%s requires a value", key)
			}
			value = args[i]
		}
		if value == "" {
			return fmt.Errorf("%s requires a value", key)
		}
		values[key] = value
	}
	origin := values["--origin"]
	if !plugins.ValidOrigin(origin) {
		return fmt.Errorf("--origin requires an exact HTTPS origin (HTTP is allowed on loopback)")
	}
	directory := values["--dir"]
	if directory == "" {
		directory = filepath.Join(home(), "sources", name)
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(absolute); err == nil {
		return fmt.Errorf("destination already exists; choose an empty new path")
	} else if !os.IsNotExist(err) {
		return err
	}
	manifest := plugins.Manifest{Version: 1, Site: name, Origin: origin, Description: name + " commands", Commands: map[string]plugins.Command{}}
	if executable := values["--executable"]; executable != "" {
		manifest.Delegate = &plugins.Delegate{Executable: executable, NativeHost: values["--native-host"]}
	} else {
		if values["--native-host"] != "" {
			return fmt.Errorf("--native-host requires --executable")
		}
		manifest.Commands["status"] = plugins.Command{Description: "Read the current page origin and title", Script: "status.js", Access: "read"}
	}
	parent := filepath.Dir(absolute)
	if err = os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(parent, ".bab-plugin-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(temp, "bab-plugin.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	if manifest.Delegate == nil {
		if err = os.WriteFile(filepath.Join(temp, "status.js"), []byte("// Edit this trusted page script; ctx.fetch(path) uses the current origin.\n// This reports page metadata, not account login validity.\nreturn {origin: location.origin, title: document.title};\n"), 0644); err != nil {
			return err
		}
	}
	if _, err = plugins.Load(temp); err != nil {
		return err
	}
	if err = os.Rename(temp, absolute); err != nil {
		return err
	}
	fmt.Printf("Created %s.\nInstall with: bab plugin install %s\n", absolute, shellQuote(absolute))
	return nil
}
