package main

import (
	"fmt"
	"os"
	"path/filepath"

	assets "github.com/sky-bro/browser-auth-bridge/internal/extension"
)

func extensionCommand(args []string) error {
	if len(args) == 1 && args[0] == "--help" {
		fmt.Println("Usage: bab extension build [--output PATH]\nPackage only installed plugins. Reload the same extension in Chrome after building.")
		return nil
	}
	if len(args) == 0 || args[0] != "build" || (len(args) != 1 && (len(args) != 3 || args[1] != "--output" || args[2] == "")) {
		return fmt.Errorf("usage: bab extension build [--output PATH]")
	}
	directory := filepath.Join(home(), "extension")
	if len(args) == 3 {
		directory = args[2]
	}
	ps, err := all()
	if err != nil {
		return err
	}
	if err = assets.InstallPlugins(directory, ps); err != nil {
		return err
	}
	fmt.Printf("Built %s. Reload this extension in chrome://extensions.\n", directory)
	return nil
}
func rebuildExtension() error {
	// Install/update/remove rebuild the standard installation automatically.
	// Existing custom/development installations remain explicitly built by their owner.
	directory := filepath.Join(home(), "extension")
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	ps, err := all()
	if err != nil {
		return err
	}
	if err = assets.InstallPlugins(directory, ps); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Rebuilt browser components. Reload your BAB extension in Chrome; custom installations must also be rebuilt.")
	return nil
}
