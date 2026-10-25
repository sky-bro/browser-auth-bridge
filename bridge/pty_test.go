package bridge

import (
	"encoding/json"
	"os"
	"testing"
)

func fixture(t *testing.T) StreamRecipe {
	t.Helper()
	data, err := os.ReadFile("../testdata/pty-recipe.json")
	if err != nil {
		t.Fatal(err)
	}
	var r StreamRecipe
	if json.Unmarshal(data, &r) != nil {
		t.Fatal("fixture decode")
	}
	return r
}
func TestPTYScope(t *testing.T) {
	r := fixture(t)
	p := PTYOpen{Version: 1, Provider: "demo", Target: "opaque-target", Recipe: r, Cols: 80, Rows: 24}
	raw, _ := json.Marshal(p)
	if _, err := DecodePTYOpen(raw, "demo", r.Origin); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*PTYOpen){
		func(p *PTYOpen) { p.Provider = "other" }, func(p *PTYOpen) { p.Version = 2 }, func(p *PTYOpen) { p.Cols = 0 },
		func(p *PTYOpen) { p.Recipe.SocketURL = "wss://other.example.test/stream" },
		func(p *PTYOpen) { p.Recipe.Authorization.URL = "https://user:secret@terminal.example.test/auth" },
		func(p *PTYOpen) { p.Recipe.Frame.RowsField = p.Recipe.Frame.ColsField },
	} {
		bad := p
		mutate(&bad)
		raw, _ := json.Marshal(bad)
		if _, err := DecodePTYOpen(raw, "demo", r.Origin); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	raw, _ = json.Marshal(p)
	for _, bad := range [][]byte{append(raw, []byte(` {}`)...), append(raw[:len(raw)-1], []byte(`,"credential":"secret"}`)...)} {
		if _, err := DecodePTYOpen(bad, "demo", r.Origin); err == nil {
			t.Fatal("accepted unknown or trailing payload")
		}
	}
}
func TestOrigins(t *testing.T) {
	for _, o := range []string{"https://terminal.example.test", "http://127.0.0.1:8765", "http://localhost:8765"} {
		if !ValidOrigin(o) {
			t.Fatal(o)
		}
	}
	for _, o := range []string{"http://remote.example.test", "https://user:secret@terminal.example.test", "https://terminal.example.test/path", "file:///tmp"} {
		if ValidOrigin(o) {
			t.Fatal(o)
		}
	}
}

func TestConsumerEndpoint(t *testing.T) {
	e := PTYEndpoint{Adapter: "ttyd", Origin: "https://terminal.example.test", PageURL: "https://terminal.example.test/", SocketURL: "wss://terminal.example.test/ws"}
	p := PTYOpen{Version: 1, Provider: "ttyd", Target: "shell", Endpoint: &e, Cols: 80, Rows: 24}
	raw, _ := json.Marshal(p)
	if _, err := DecodePTYOpen(raw, "ttyd", e.Origin); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*PTYOpen){
		func(p *PTYOpen) { p.Recipe = fixture(t) },
		func(p *PTYOpen) { p.Endpoint.SocketURL = "wss://other.test/ws" },
		func(p *PTYOpen) { p.Endpoint.Adapter = "../script" },
	} {
		bad := p
		copy := e
		bad.Endpoint = &copy
		mutate(&bad)
		raw, _ := json.Marshal(bad)
		if _, err := DecodePTYOpen(raw, "ttyd", e.Origin); err == nil {
			t.Fatal("accepted invalid endpoint")
		}
	}
}
