package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"
)

// BrowserCommand runs CLI-owned JavaScript on an explicitly authorized origin.
// Script is an async function body; args is its JSON argument object.
type BrowserCommand struct {
	Version        int             `json:"version"`
	Origin         string          `json:"origin"`
	BrowserSession string          `json:"browserSession"`
	Action         string          `json:"action"`
	URL            string          `json:"url,omitempty"`
	Match          string          `json:"match,omitempty"`
	Script         string          `json:"script,omitempty"`
	Args           json.RawMessage `json:"args,omitempty"`
}
type BrowserResult struct {
	Version int             `json:"version"`
	TabID   int             `json:"tabId,omitempty"`
	Owned   bool            `json:"owned,omitempty"`
	Value   json.RawMessage `json:"value,omitempty"`
}

type BrowserOptions struct {
	// AllowTab explicitly permits creating an inactive tab when no page exists.
	AllowTab bool
}

func DecodeBrowserCommand(raw json.RawMessage, origin string) (BrowserCommand, error) {
	var cmd BrowserCommand
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&cmd); err != nil {
		return cmd, err
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return cmd, fmt.Errorf("trailing browser command")
	}
	if cmd.Version != 1 || cmd.Origin != origin || !ValidSession(cmd.BrowserSession) {
		return cmd, fmt.Errorf("browser scope denied")
	}
	switch cmd.Action {
	case "open":
		u, err := url.Parse(cmd.URL)
		if err != nil || u.Scheme+"://"+u.Host != origin || u.User != nil || len(cmd.URL) > 8192 ||
			(cmd.Match != "" && cmd.Match != "origin") || cmd.Script != "" || len(cmd.Args) > 0 {
			return cmd, fmt.Errorf("browser navigation denied")
		}
	case "eval":
		if cmd.URL != "" || cmd.Match != "" || len(cmd.Script) == 0 || len(cmd.Script) > 256*1024 {
			return cmd, fmt.Errorf("invalid browser script")
		}
		if len(cmd.Args) > 0 {
			var args map[string]any
			if json.Unmarshal(cmd.Args, &args) != nil {
				return cmd, fmt.Errorf("args must be a JSON object")
			}
		}
	case "close":
		if cmd.URL != "" || cmd.Match != "" || cmd.Script != "" || len(cmd.Args) > 0 {
			return cmd, fmt.Errorf("invalid browser close")
		}
	default:
		return cmd, fmt.Errorf("unknown browser action")
	}
	return cmd, nil
}

// ExecuteBrowser has no automatic retry: scripts may have side effects.
func ExecuteBrowser(ctx context.Context, namespace, service string, command BrowserCommand, validate func(Message) error, options BrowserOptions) (BrowserResult, error) {
	var result BrowserResult
	if validate == nil {
		return result, fmt.Errorf("browser validator required")
	}
	id, err := NewSessionID()
	if err != nil {
		return result, err
	}
	payload, err := json.Marshal(command)
	if err != nil {
		return result, err
	}
	attach := Message{Type: "attach", Session: id, Service: service, Operation: "browser", Payload: payload, AllowTab: options.AllowTab, NoOpenTab: !options.AllowTab}
	if err := validate(attach); err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	s, err := Connect(ctx, namespace, attach, 300*time.Millisecond)
	if err != nil {
		return result, fmt.Errorf("connect browser bridge: %w", err)
	}
	defer s.conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = s.conn.Close() })
	defer stop()
	for {
		m, err := s.Read()
		if err != nil {
			if ctx.Err() != nil {
				return result, fmt.Errorf("browser outcome unknown: %w; inspect page before retrying", ctx.Err())
			}
			return result, fmt.Errorf("browser transport failed: %w; reload the compatible extension and updated Native Host; do not blindly replay scripts", err)
		}
		if m.Session != id {
			return result, fmt.Errorf("browser response session mismatch")
		}
		switch m.Type {
		case "result":
			if err = json.Unmarshal(m.Result, &result); err != nil {
				return result, err
			}
			if result.Version != 1 {
				return result, fmt.Errorf("unsupported browser response")
			}
			return result, nil
		case "error":
			return result, fmt.Errorf("browser: %s", m.Error)
		case "status":
			if m.State == "closed" {
				return result, fmt.Errorf("browser closed before result; outcome unknown")
			}
		}
	}
}
