// bridge-ttyd is a reference consumer of the browser-authenticated transport SDK.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sky-bro/browser-auth-bridge/bridge"
	"github.com/sky-bro/browser-auth-bridge/commandstream"
	"github.com/sky-bro/browser-auth-bridge/nativehost"
)

const namespace = "bridge-ttyd"
const host = "org.browserauthbridge.ttyd"

type config struct {
	Origin string `json:"origin"`
}
type exitError int

func (e exitError) ExitCode() int { return int(e) }
func (e exitError) Error() string { return fmt.Sprintf("remote exit status %d", int(e)) }
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var status interface{ ExitCode() int }
		if errors.As(err, &status) {
			os.Exit(status.ExitCode())
		}
		os.Exit(1)
	}
}
func endpoint(c config) bridge.PTYEndpoint {
	return bridge.PTYEndpoint{Adapter: "ttyd", Origin: c.Origin, PageURL: c.Origin + "/", SocketURL: strings.Replace(c.Origin, "http", "ws", 1) + "/ws"}
}
func validate(c config, m bridge.Message) error {
	if m.Service != "ttyd" || m.Operation != "pty" {
		return fmt.Errorf("provider or operation denied")
	}
	p, err := bridge.DecodePTYOpen(m.Payload, "ttyd", c.Origin)
	if err != nil {
		return err
	}
	want := endpoint(c)
	if p.Target != "shell" || p.Endpoint == nil || *p.Endpoint != want {
		return fmt.Errorf("target or endpoint denied")
	}
	return nil
}
func run(args []string) error { return runCLI(args, os.Stdout) }

func runCLI(args []string, out io.Writer) error {
	if len(args) == 0 || len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		_, err := fmt.Fprintln(out, usage)
		return err
	}
	if len(args) == 1 && args[0] == "--version" {
		_, err := fmt.Fprintln(out, "bridge-ttyd", version)
		return err
	}
	if len(args) == 3 && args[0] == "install-host" {
		return install(args[1], args[2])
	}
	if len(args) == 1 && args[0] == "doctor" {
		return doctor(out)
	}
	native := args[0] == "native-host" || strings.HasPrefix(args[0], "chrome-extension://")
	shell := len(args) == 1 && args[0] == "shell"
	command := args[1:]
	var timeout time.Duration
	if args[0] == "exec" {
		flags := flag.NewFlagSet("exec", flag.ContinueOnError)
		flags.SetOutput(out)
		flags.DurationVar(&timeout, "timeout", 0, "overall timeout (0 disables it)")
		if err := flags.Parse(command); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if timeout < 0 {
			return fmt.Errorf("--timeout must not be negative")
		}
		command = flags.Args()
	}
	executeCommand := args[0] == "exec" && len(command) > 0
	if !native && !shell && !executeCommand {
		return fmt.Errorf("invalid command or arguments\n%s", usage)
	}
	c, err := load()
	if err != nil {
		return err
	}
	if native {
		return nativehost.Run(os.Stdin, os.Stdout, nativehost.Config{Namespace: namespace, ValidateAttach: func(m bridge.Message) error { return validate(c, m) }})
	}
	if shell {
		return terminal(c)
	}
	return execute(c, command, timeout, out)
}

func connect(ctx context.Context, c config) (*bridge.Session, error) {
	id, err := bridge.NewSessionID()
	if err != nil {
		return nil, err
	}
	e := endpoint(c)
	cols, rows := size()
	payload, err := json.Marshal(bridge.PTYOpen{Version: 1, Provider: "ttyd", Target: "shell", Endpoint: &e, Cols: cols, Rows: rows})
	if err != nil {
		return nil, err
	}
	m := bridge.Message{Type: "attach", Session: id, Service: "ttyd", Operation: "pty", Payload: payload, AllowTab: true}
	if err = validate(c, m); err != nil {
		return nil, err
	}
	s, err := bridge.Connect(ctx, namespace, m, time.Second)
	if err != nil {
		return nil, fmt.Errorf("enable ttyd in the shared Browser Auth Bridge extension and log in at %s: %w", c.Origin, err)
	}
	stop := context.AfterFunc(ctx, func() { s.Close() })
	timer := time.AfterFunc(20*time.Second, func() { s.Close() })
	for {
		m, err = s.Read()
		if err != nil {
			stop()
			timer.Stop()
			s.Close()
			return nil, fmt.Errorf("wait for terminal (check browser login): %w", err)
		}
		if m.Type == "error" || m.Type == "status" && m.State == "closed" {
			stop()
			timer.Stop()
			s.Close()
			return nil, fmt.Errorf("terminal unavailable: %s; log in at %s", m.Error, c.Origin)
		}
		if m.Type == "status" && m.State == "connected" {
			timer.Stop()
			return s, nil
		}
	}
}
func stty(args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = os.Stdin
	b, err := cmd.Output()
	return strings.TrimSpace(string(b)), err
}
func size() (int, int) {
	out, err := stty("size")
	if err == nil {
		var rows, cols int
		if _, err = fmt.Sscan(out, &rows, &cols); err == nil && cols > 0 && rows > 0 {
			return cols, rows
		}
	}
	return 80, 24
}
func terminal(c config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	s, err := connect(ctx, c)
	if err != nil {
		return err
	}
	defer s.Close()
	state, err := stty("-g")
	if err == nil {
		if _, err = stty("raw", "-echo"); err != nil {
			return err
		}
		defer stty(state)
	}
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-winch:
				cols, rows := size()
				if s.Send(bridge.Message{Type: "resize", Cols: cols, Rows: rows}) != nil {
					return
				}
			}
		}
	}()
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if s.Send(bridge.Message{Type: "stdin", Data: string(buf[:n])}) != nil {
					return
				}
			}
			if err != nil {
				s.Close()
				return
			}
		}
	}()
	for {
		m, err := s.Read()
		if err != nil {
			return err
		}
		switch m.Type {
		case "stdout":
			if _, err = io.WriteString(os.Stdout, m.Data); err != nil {
				return err
			}
		case "error":
			return errors.New(m.Error)
		case "status":
			if m.State == "closed" {
				return nil
			}
		}
	}
}
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func execute(c config, args []string, timeout time.Duration, out io.Writer) (result error) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if timeout > 0 {
		var stop context.CancelFunc
		ctx, stop = context.WithTimeout(ctx, timeout)
		defer stop()
	}
	defer func() {
		if ctx.Err() != nil {
			code := 130
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				code = 124
			}
			result = &commandError{code, ctx.Err()}
		}
	}()
	s, err := connect(ctx, c)
	if err != nil {
		return err
	}
	defer s.Close()
	return executeSession(ctx, s, args, out)
}

type commandError struct {
	code  int
	cause error
}

func (e *commandError) Error() string { return e.cause.Error() }
func (e *commandError) Unwrap() error { return e.cause }
func (e *commandError) ExitCode() int { return e.code }

type commandSession interface {
	Send(bridge.Message) error
	Read() (bridge.Message, error)
	Close() error
}

func executeSession(ctx context.Context, s commandSession, args []string, out io.Writer) error {
	stop := context.AfterFunc(ctx, func() { s.Close() })
	defer stop()
	output := commandstream.NewWriter(ctx, out)
	defer output.Close()
	token, err := bridge.NewSessionID()
	if err != nil {
		return err
	}
	start := "\x1e" + token + "\x1f"
	end := "\x1e" + token + ":"
	words := make([]string, len(args))
	for i, arg := range args {
		words[i] = quote(arg)
	}
	// Split markers avoid matching terminal echo. Commands are never retried.
	script := "stty -echo; printf '\\036%s\\037' " + quote(token) + "; ( " + strings.Join(words, " ") + " ); __bridge_rc=$?; printf '\\036%s:%d\\037' " + quote(token) + " \"$__bridge_rc\"\n"
	if err = s.Send(bridge.Message{Type: "stdin", Data: script}); err != nil {
		return err
	}
	decoder := commandstream.New(output, start, end, '\x1f', false)
	for {
		m, err := s.Read()
		if err != nil {
			return err
		}
		if m.Type == "error" || m.Type == "status" && m.State == "closed" {
			return fmt.Errorf("terminal closed before command completion: %s", m.Error)
		}
		if m.Type != "stdout" {
			continue
		}
		done, status, err := decoder.Feed(m.Data)
		if err != nil {
			return err
		}
		if done {
			if status != 0 {
				return exitError(status)
			}
			return nil
		}
	}
}
