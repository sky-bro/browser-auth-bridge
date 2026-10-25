package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/sky-bro/browser-auth-bridge/bridge"
)

var version = "dev"

const usage = `Usage: bridge-ttyd COMMAND

  install-host EXTENSION_ID ORIGIN  Register this binary with the shared extension
  shell                            Open an interactive terminal
  exec [--timeout DURATION] [--] COMMAND [ARGS...]       Run a command in a new terminal
  doctor                           Check local configuration without opening a terminal
  --help                           Show this help
  --version                        Show build version

Log in to the configured site in Chrome and enable its provider in the shared
Browser Auth Bridge extension. BRIDGE_TTYD_ORIGIN overrides the configured origin.
Only macOS and Linux are supported. exec transports UTF-8 terminal text.`

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	return filepath.Join(home, ".config", namespace, "config.json"), err
}
func load() (config, error) {
	if origin := os.Getenv("BRIDGE_TTYD_ORIGIN"); origin != "" {
		return checked(config{origin})
	}
	path, err := configPath()
	if err != nil {
		return config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config{}, fmt.Errorf("run install-host EXTENSION_ID ORIGIN first: %w", err)
	}
	var c config
	if err = json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	return checked(c)
}
func checked(c config) (config, error) {
	if !bridge.ValidOrigin(c.Origin) {
		return c, fmt.Errorf("invalid origin")
	}
	return c, nil
}
func manifestPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	var dir string
	switch runtime.GOOS {
	case "darwin":
		dir = filepath.Join(home, "Library/Application Support/Google/Chrome/NativeMessagingHosts")
	case "linux":
		dir = filepath.Join(home, ".config/google-chrome/NativeMessagingHosts")
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return filepath.Join(dir, host+".json"), nil
}

func install(id, origin string) error {
	if !regexp.MustCompile(`^[a-p]{32}$`).MatchString(id) {
		return fmt.Errorf("invalid extension ID")
	}
	c, err := checked(config{origin})
	if err != nil {
		return err
	}
	manifest, err := manifestPath()
	if err != nil {
		return err
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	if err = os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	data, err = json.MarshalIndent(map[string]any{"name": host, "description": "Independent ttyd bridge consumer", "path": exe, "type": "stdio", "allowed_origins": []string{"chrome-extension://" + id + "/"}}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
		return err
	}
	return os.WriteFile(manifest, append(data, '\n'), 0600)
}

// doctor checks local prerequisites only. It never attaches or executes remotely.
func doctor(out io.Writer) error {
	var problems []error
	report := func(label string, err error) {
		if err != nil {
			fmt.Fprintf(out, "FAIL %s: %v\n", label, err)
			problems = append(problems, err)
		} else {
			fmt.Fprintf(out, "OK   %s\n", label)
		}
	}
	c, err := load()
	report("origin configuration", err)
	if err == nil {
		fmt.Fprintln(out, "     origin:", c.Origin)
	}
	_, err = exec.LookPath("stty")
	report("stty", err)
	path, err := manifestPath()
	if err == nil {
		var data []byte
		data, err = os.ReadFile(path)
		if err == nil {
			var manifest struct {
				Name           string   `json:"name"`
				Path           string   `json:"path"`
				Type           string   `json:"type"`
				AllowedOrigins []string `json:"allowed_origins"`
			}
			err = json.Unmarshal(data, &manifest)
			if err == nil {
				exe, executableErr := os.Executable()
				err = executableErr
				if err == nil && (manifest.Name != host || manifest.Type != "stdio" || manifest.Path != exe || len(manifest.AllowedOrigins) != 1 || !regexp.MustCompile(`^chrome-extension://[a-p]{32}/$`).MatchString(manifest.AllowedOrigins[0])) {
					err = fmt.Errorf("registration differs from this binary; rerun install-host")
				}
			}
		}
	}
	report("Chrome native host registration", err)
	socket, err := bridge.BrokerSocketPath(namespace)
	if err == nil {
		var info os.FileInfo
		info, err = os.Stat(socket)
		if err == nil && info.Mode()&os.ModeSocket == 0 {
			err = fmt.Errorf("broker path is not a socket")
		}
	}
	report("local broker socket (presence only)", err)
	fmt.Fprintln(out, "Browser permission, login and remote connectivity are not checked. Enable ttyd in the shared extension and keep an authenticated page open.")
	if len(problems) > 0 {
		return fmt.Errorf("local checks failed: %w", errors.Join(problems...))
	}
	return nil
}
