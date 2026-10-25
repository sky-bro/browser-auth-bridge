package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const completionStart = "# >>> bab zsh completion >>>"
const completionEnd = "# <<< bab zsh completion <<<"

func completionCommand(args []string) error {
	if len(args) == 0 || args[0] != "zsh" {
		return fmt.Errorf("usage: bab completion zsh [--install] [--enable] [--prefix PATH]")
	}
	flags := flag.NewFlagSet("completion zsh", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	install := flags.Bool("install", false, "Install the completion file")
	enable := flags.Bool("enable", false, "Enable completion in zsh startup configuration")
	prefix := flags.String("prefix", "", "Installation prefix")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return fmt.Errorf("usage: bab completion zsh [--install] [--enable] [--prefix PATH]")
	}
	if !*install {
		if *enable || *prefix != "" {
			return fmt.Errorf("--enable and --prefix require --install")
		}
		_, err := fmt.Fprint(os.Stdout, zshCompletion)
		return err
	}
	if *prefix == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		*prefix = filepath.Join(home, ".local")
	}
	absolute, err := filepath.Abs(*prefix)
	if err != nil {
		return err
	}
	directory := filepath.Join(absolute, "share/zsh/site-functions")
	if err = os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	path := filepath.Join(directory, "_bab")
	if info, e := os.Lstat(path); e == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("completion output must be a regular file")
	}
	if err = writeAtomic(path, []byte(zshCompletion), 0644); err != nil {
		return err
	}
	fmt.Printf("Installed zsh completion: %s\n", path)
	if !*enable {
		fmt.Println("Enable once with: bab completion zsh --install --enable --prefix " + shellQuote(absolute))
		return nil
	}
	dotdir := os.Getenv("ZDOTDIR")
	if dotdir == "" {
		dotdir, err = os.UserHomeDir()
		if err != nil {
			return err
		}
	}
	rc := filepath.Join(dotdir, ".zshrc")
	if err = enableZshCompletion(rc, directory); err != nil {
		return err
	}
	fmt.Printf("Enabled in %s. Open a new zsh session to use completion.\n", rc)
	return nil
}
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".bab-completion-")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func zshStartupBlock(directory string) string {
	return completionStart + "\n" +
		"fpath=(" + shellQuote(directory) + " $fpath)\n" +
		"if (( ! $+functions[compdef] )); then\n" +
		"  autoload -Uz compinit\n" +
		"  compinit\n" +
		"fi\n" +
		"autoload -Uz _bab\n" +
		"compdef _bab bab\n" +
		completionEnd + "\n"
}
func patchStartup(data, block string) (string, error) {
	starts, ends := strings.Count(data, completionStart), strings.Count(data, completionEnd)
	if starts == 0 && ends == 0 {
		if data != "" && !strings.HasSuffix(data, "\n") {
			data += "\n"
		}
		return data + block, nil
	}
	if starts != 1 || ends != 1 {
		return "", fmt.Errorf("ambiguous BAB startup markers; existing configuration preserved")
	}
	start, end := strings.Index(data, completionStart), strings.Index(data, completionEnd)
	if end < start {
		return "", fmt.Errorf("invalid BAB startup marker order; existing configuration preserved")
	}
	end += len(completionEnd)
	if end < len(data) && data[end] == '\n' {
		end++
	}
	return data[:start] + block + data[end:], nil
}
func enableZshCompletion(path, directory string) error {
	mode := os.FileMode(0644)
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			path, err = filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			info, err = os.Stat(path)
			if err != nil {
				return err
			}
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("zsh startup configuration must be a regular file")
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	updated, err := patchStartup(string(data), zshStartupBlock(directory))
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return writeAtomic(path, []byte(updated), mode)
}
