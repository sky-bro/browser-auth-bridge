package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/sky-bro/browser-auth-bridge/plugins"
)

func listPlugins(args []string) error {
	if len(args) == 1 && args[0] == "--help" {
		fmt.Println("Usage: bab plugin list [--json]\nShow installed plugins and their local availability. bab list is an alias.")
		return nil
	}
	asJSON := len(args) == 1 && args[0] == "--json"
	if len(args) > 0 && !asJSON {
		return fmt.Errorf("usage: bab plugin list [--json]")
	}
	installed, err := all()
	if err != nil {
		return err
	}
	if asJSON {
		if installed == nil {
			installed = []plugins.Plugin{}
		}
		return printJSON(installed)
	}
	return renderPluginList(os.Stdout, installed)
}
func terminalText(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	return strings.Join(strings.Fields(text), " ")
}
func renderPluginList(out io.Writer, installed []plugins.Plugin) error {
	if len(installed) == 0 {
		_, err := fmt.Fprintln(out, "No plugins installed. Run bab plugin available to browse optional plugins.")
		return err
	}
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "COMMAND\tRUNTIME\tSTATE\tSTATUS\tDESCRIPTION"); err != nil {
		return err
	}
	for _, p := range installed {
		kind, status := "browser-script", "available"
		if p.Delegate != nil {
			kind = "external-cli"
			if _, err := plugins.Executable(p.Delegate.Executable); err != nil {
				status = "missing executable"
			}
		}
		if _, err := fmt.Fprintf(table, "bab %s\t%s\t%s\t%s\t%s\n", p.Site, kind, map[bool]string{true: "disabled", false: "enabled"}[p.Disabled], status, terminalText(p.Description)); err != nil {
			return err
		}
	}
	return table.Flush()
}
