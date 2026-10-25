package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sky-bro/browser-auth-bridge/plugins"
)

type completionItem struct{ value, description string }
type completionResult struct {
	items []completionItem
	files bool
}

func suggestions(items ...string) completionResult {
	out := completionResult{}
	for _, item := range items {
		value, description, _ := strings.Cut(item, "\t")
		out.items = append(out.items, completionItem{value, description})
	}
	return out
}
func installedSuggestions() completionResult {
	out := completionResult{}
	ps, _ := all()
	for _, p := range ps {
		out.items = append(out.items, completionItem{p.Site, p.Description})
	}
	return out
}
func completionCandidates(words []string) completionResult {
	if len(words) == 0 {
		words = []string{""}
	}
	before := words[:len(words)-1]
	current := words[len(words)-1]
	if len(before) == 0 {
		out := suggestions("setup\tSet up the browser bridge", "extension\tAssemble installed browser components", "doctor\tCheck installation and transport", "list\tList registered commands", "plugin\tManage optional plugins", "auth\tManage site authorization", "completion\tGenerate shell completion", "help\tShow help", "--help\tShow help", "--version\tShow version")
		out.items = append(out.items, installedSuggestions().items...)
		return out
	}
	switch before[0] {
	case "plugin":
		if len(before) == 1 {
			return suggestions("init\tCreate a plugin starter", "available\tList optional sources", "enable\tEnable an installed plugin", "disable\tStop an installed plugin", "install\tInstall selected plugins", "update\tUpdate selected plugins", "list\tList installed plugins", "remove\tRemove an installed plugin", "sync\tRefresh the extension catalog", "catalog\tManage local source catalogs")
		}
		switch before[1] {
		case "init":
			if before[len(before)-1] == "--dir" || before[len(before)-1] == "--executable" {
				return completionResult{files: true}
			}
			if len(before) >= 3 {
				return suggestions("--origin\tSite origin", "--dir\tNew source directory", "--executable\tDelegate an existing CLI", "--native-host\tConsumer Native Host")
			}
			return completionResult{}
		case "catalog":
			if len(before) == 2 {
				return suggestions("add\tRegister a local catalog", "remove\tUnregister a catalog", "list\tList configured catalogs")
			}
			if len(before) == 3 && (before[2] == "add" || before[2] == "remove") {
				return completionResult{files: true}
			}
		case "install", "update":
			if before[len(before)-1] == "--origin" || strings.HasPrefix(current, "--origin=") {
				return completionResult{}
			}
			if strings.HasPrefix(current, "-") {
				return suggestions("--origin\tDeployment origin for one selected plugin", "--disabled\tInstall without enabling the plugin")
			}
			out := completionResult{files: true}
			paths, err := catalogPaths()
			if err != nil {
				return out
			}
			entries, err := plugins.Available(paths)
			if err != nil {
				return out
			}
			registered := map[string]bool{}
			ps, _ := all()
			for _, p := range ps {
				registered[p.Site] = true
			}
			used := map[string]bool{}
			for _, word := range before[2:] {
				used[word] = true
			}
			for _, entry := range entries {
				if used[entry.Name] || (before[1] == "install" && registered[entry.Name]) || (before[1] == "update" && !registered[entry.Name]) {
					continue
				}
				out.items = append(out.items, completionItem{entry.Name, entry.Description})
			}
			return out
		case "list", "available":
			if len(before) == 2 {
				return suggestions("--json\tFull plugin manifests as JSON", "--help\tShow list help")
			}
		case "enable", "disable", "remove":
			if len(before) == 2 {
				return installedSuggestions()
			}
		}
		return completionResult{}
	case "auth":
		if len(before) == 1 {
			return suggestions("login\tOpen site authorization", "status\tShow current origin grants", "revoke\tRevoke a site's grants")
		}
		if len(before) == 2 && (before[1] == "login" || before[1] == "status" || before[1] == "revoke") {
			return installedSuggestions()
		}
		return completionResult{}
	case "extension":
		if len(before) == 1 {
			return suggestions("build\tPackage installed browser components")
		}
		if before[len(before)-1] == "--output" {
			return completionResult{files: true}
		}
		if before[1] == "build" {
			return suggestions("--output\tGenerated extension directory")
		}
		return completionResult{}
	case "setup":
		if before[len(before)-1] == "--extension-id" {
			return completionResult{}
		}
		return suggestions("--extension-id\tChrome extension ID")
	case "completion":
		if len(before) == 1 {
			return suggestions("zsh\tZsh completion")
		}
		if before[len(before)-1] == "--prefix" {
			return completionResult{files: true}
		}
		if before[1] == "zsh" {
			return suggestions("--install\tInstall the completion file", "--enable\tEnable completion in zsh startup configuration", "--prefix\tInstallation prefix")
		}
		return completionResult{}
	case "list":
		if len(before) == 1 {
			return suggestions("--json\tFull plugin manifests as JSON", "--help\tShow list help")
		}
		return completionResult{}
	case "doctor", "help", "version":
		return completionResult{}
	}
	p, err := find(before[0])
	if err != nil {
		return completionResult{}
	}
	if p.Delegate == nil {
		if len(before) == 1 {
			out := suggestions("--help\tShow command schema")
			for _, name := range plugins.Names(p) {
				out.items = append(out.items, completionItem{name, p.Commands[name].Description})
			}
			return out
		}
		c, ok := p.Commands[before[1]]
		if !ok {
			return completionResult{}
		}
		return argumentCandidates(c.Positional, c.Options, c.Artifact, before[2:], current, true)
	}
	spec := p.Completion
	if spec == nil {
		// Builtin examples carry their own declarative defaults, so older installed
		// snapshots can complete without running the delegated executable.
		if template, err := plugins.BuiltinManifest(p.Site); err == nil && template.Delegate != nil && template.Delegate.NativeHost == p.Delegate.NativeHost && filepath.Base(template.Delegate.Executable) == filepath.Base(p.Delegate.Executable) {
			spec = template.Completion
		}
	}
	if spec == nil {
		return suggestions("--help\tShow delegated CLI help")
	}
	rest := before[1:]
	for len(rest) > 0 {
		child, ok := spec.Commands[rest[0]]
		if !ok {
			break
		}
		next := child
		spec = &next
		rest = rest[1:]
	}
	if len(rest) == 0 && len(spec.Commands) > 0 {
		out := suggestions("--help\tShow command help")
		for name, child := range spec.Commands {
			out.items = append(out.items, completionItem{name, child.Description})
		}
		for _, option := range spec.Options {
			out.items = append(out.items, completionItem{"--" + option.Name, "Option"})
		}
		return out
	}
	return argumentCandidates(spec.Positional, spec.Options, false, rest, current, false)
}
func argumentCandidates(positional, options []plugins.Argument, artifact bool, before []string, current string, jsonOutput bool) completionResult {
	opts := map[string]plugins.Argument{}
	for _, a := range options {
		opts["--"+a.Name] = a
	}
	if artifact {
		opts["--output"] = plugins.Argument{Name: "output"}
	}
	waiting := ""
	position := 0
	used := map[string]bool{}
	literal := false
	for _, word := range before {
		if waiting != "" {
			waiting = ""
			continue
		}
		if literal {
			position++
			continue
		}
		if word == "--" {
			literal = true
			continue
		}
		if strings.HasPrefix(word, "--") {
			name, _, inline := strings.Cut(word, "=")
			used[name] = true
			if _, ok := opts[name]; ok && !inline {
				waiting = name
			}
			continue
		}
		position++
	}
	if waiting != "" {
		if waiting == "--output" {
			return completionResult{files: true}
		}
		out := completionResult{}
		for _, choice := range opts[waiting].Choices {
			out.items = append(out.items, completionItem{choice, ""})
		}
		return out
	}
	if name, _, inline := strings.Cut(current, "="); inline && strings.HasPrefix(name, "--") {
		out := completionResult{}
		for _, choice := range opts[name].Choices {
			out.items = append(out.items, completionItem{name + "=" + choice, ""})
		}
		return out
	}
	out := completionResult{}
	if position < len(positional) && !strings.HasPrefix(current, "-") {
		for _, choice := range positional[position].Choices {
			out.items = append(out.items, completionItem{choice, ""})
		}
	}
	if !literal {
		for name := range opts {
			if !used[name] {
				out.items = append(out.items, completionItem{name, "Option"})
			}
		}
		if jsonOutput {
			out.items = append(out.items, completionItem{"--json", "JSON output"})
		}
		if len(before) == 0 {
			out.items = append(out.items, completionItem{"--help", "Show command schema"})
		}
	}
	return out
}
func printCompletion(words []string) error {
	result := completionCandidates(words)
	sort.Slice(result.items, func(i, j int) bool { return result.items[i].value < result.items[j].value })
	clean := func(value string) string {
		return strings.Map(func(r rune) rune {
			if r < 32 || r == 127 {
				return ' '
			}
			return r
		}, value)
	}
	seen := map[string]bool{}
	for _, item := range result.items {
		value := clean(item.value)
		if value != "" && !seen[value] {
			fmt.Printf("value\t%s\t%s\n", value, clean(item.description))
			seen[value] = true
		}
	}
	if result.files {
		fmt.Println("files")
	}
	return nil
}

const zshCompletion = `#compdef bab
# Read local metadata at completion time; never execute a delegated command.
_bab() {
  local -a query records candidates
  local record payload value description
  local wants_files=0
  query=("${words[@]:1:$((CURRENT - 1))}")
  records=("${(@f)$(command "${words[1]}" __complete "${query[@]}" 2>/dev/null)}")
  for record in "${records[@]}"; do
    case "$record" in
      files) wants_files=1 ;;
      value$'\t'*)
        payload="${record#*$'\t'}"
        value="${payload%%$'\t'*}"
        description="${payload#*$'\t'}"
        value="${value//\\/\\\\}"
        value="${value//:/\\:}"
        description="${description//\\/\\\\}"
        description="${description//:/\\:}"
        candidates+=("$value:$description")
        ;;
    esac
  done
  local matched=1
  if (( ${#candidates} )); then
    _describe -t bab-values 'bab' candidates && matched=0
  fi
  if (( wants_files )); then
    _files && matched=0
  fi
  return $matched
}
_bab "$@"
`
