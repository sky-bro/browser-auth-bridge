package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
)

// PTYAdapter owns target selection and site protocol knowledge. Implementations
// live in consumers, never in the transport. Recipes contain no credentials.
type PTYAdapter interface {
	Recipe(target string) (StreamRecipe, error)
}

type StreamRecipe struct {
	Version       int                 `json:"version"`
	Origin        string              `json:"origin"`
	PageURL       string              `json:"pageUrl"`
	Authorization AuthorizationRecipe `json:"authorization"`
	SocketURL     string              `json:"socketUrl"`
	Frame         FrameRecipe         `json:"frame"`
}
type AuthorizationRecipe struct {
	URL           string `json:"url"`
	StatusField   string `json:"statusField"`
	SuccessValue  string `json:"successValue"`
	LoginValue    string `json:"loginValue"`
	AccessField   string `json:"accessField"`
	AccessGranted bool   `json:"accessGranted"`
}
type FrameRecipe struct {
	OperationField string `json:"operationField"`
	DataField      string `json:"dataField"`
	ColsField      string `json:"colsField"`
	RowsField      string `json:"rowsField"`
	Stdin          string `json:"stdin"`
	Stdout         string `json:"stdout"`
	Header         string `json:"header"`
	Resize         string `json:"resize"`
}

// PTYOpen is the versioned pty-v1 payload. Target is opaque to the core.
type PTYOpen struct {
	Version  int          `json:"version"`
	Provider string       `json:"provider"`
	Target   string       `json:"target"`
	Recipe   StreamRecipe `json:"recipe,omitzero"`
	Endpoint *PTYEndpoint `json:"endpoint,omitempty"`
	Cols     int          `json:"cols"`
	Rows     int          `json:"rows"`
}

// PTYEndpoint selects a consumer-packaged browser adapter. It carries only
// scoped URLs; adapter code and credentials never arrive over the socket.
type PTYEndpoint struct {
	Adapter   string `json:"adapter"`
	Origin    string `json:"origin"`
	PageURL   string `json:"pageUrl"`
	SocketURL string `json:"socketUrl"`
}

func (e PTYEndpoint) Validate(origin string) error {
	if !ValidNamespace(e.Adapter) || e.Origin != origin || !ValidOrigin(origin) {
		return fmt.Errorf("invalid PTY endpoint")
	}
	o, _ := url.Parse(origin)
	for _, item := range []struct{ value, scheme string }{{e.PageURL, o.Scheme}, {e.SocketURL, map[string]string{"https": "wss", "http": "ws"}[o.Scheme]}} {
		u, err := url.Parse(item.value)
		if err != nil || len(item.value) > 8192 || u.Scheme != item.scheme || u.Host != o.Host || u.User != nil || u.Fragment != "" || !strings.HasPrefix(u.Path, "/") {
			return fmt.Errorf("PTY endpoint must stay within granted origin")
		}
	}
	return nil
}

func ValidOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme+"://"+u.Host != origin || u.Host == "" || u.User != nil {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()))
}
func (r StreamRecipe) Validate(origin string) error {
	if r.Version != 1 || r.Origin != origin || !ValidOrigin(origin) {
		return fmt.Errorf("invalid PTY origin or version")
	}
	check := func(value string, socket bool) bool {
		u, e := url.Parse(value)
		o, _ := url.Parse(origin)
		scheme := o.Scheme
		if socket {
			if scheme == "https" {
				scheme = "wss"
			} else {
				scheme = "ws"
			}
		}
		return e == nil && len(value) <= 8192 && u.Scheme == scheme && u.Host == o.Host && u.User == nil && u.Fragment == "" && strings.HasPrefix(u.Path, "/")
	}
	if !check(r.PageURL, false) || !check(r.Authorization.URL, false) || !check(r.SocketURL, true) {
		return fmt.Errorf("PTY URLs must stay within granted origin")
	}
	field := func(v string) bool {
		if len(v) < 1 || len(v) > 32 || v == "__proto__" {
			return false
		}
		for i, c := range v {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && (c >= '0' && c <= '9' || c == '_')) {
				return false
			}
		}
		return true
	}
	distinct := func(values ...string) bool {
		seen := map[string]bool{}
		for _, v := range values {
			if seen[v] {
				return false
			}
			seen[v] = true
		}
		return true
	}
	a, f := r.Authorization, r.Frame
	if !field(a.StatusField) || !field(a.AccessField) || !distinct(a.StatusField, a.AccessField) {
		return fmt.Errorf("invalid authorization fields")
	}
	for _, v := range []string{f.OperationField, f.DataField, f.ColsField, f.RowsField} {
		if !field(v) {
			return fmt.Errorf("invalid frame field")
		}
	}
	if !distinct(f.OperationField, f.DataField, f.ColsField, f.RowsField) || !distinct(f.Stdin, f.Stdout, f.Header, f.Resize) {
		return fmt.Errorf("duplicate frame fields or operations")
	}
	for _, v := range []string{a.SuccessValue, a.LoginValue, f.Stdin, f.Stdout, f.Header, f.Resize} {
		if len(v) < 1 || len(v) > 64 {
			return fmt.Errorf("invalid operation label")
		}
	}
	return nil
}
func DecodePTYOpen(raw json.RawMessage, provider, origin string) (PTYOpen, error) {
	var p PTYOpen
	if len(raw) > 16384 {
		return p, fmt.Errorf("PTY payload too large")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	if d.Decode(new(any)) != io.EOF {
		return p, fmt.Errorf("trailing PTY payload")
	}
	if p.Version != 1 || p.Provider != provider || !ValidNamespace(provider) || len(p.Target) == 0 || len(p.Target) > 256 || p.Cols < 1 || p.Cols > 10000 || p.Rows < 1 || p.Rows > 10000 {
		return p, fmt.Errorf("invalid PTY open")
	}
	if p.Endpoint != nil {
		if p.Recipe != (StreamRecipe{}) {
			return p, fmt.Errorf("recipe and endpoint are mutually exclusive")
		}
		return p, p.Endpoint.Validate(origin)
	}
	return p, p.Recipe.Validate(origin)
}
