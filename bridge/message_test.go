package bridge

import (
	"strings"
	"testing"
)

func TestPathsAreNamespaced(t *testing.T) {
	session := strings.Repeat("a", 64)
	console, err := BrokerSocketPath("demo-a")
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := BrokerSocketPath("demo-b")
	if err != nil {
		t.Fatal(err)
	}
	if console == inventory {
		t.Fatal("app broker sockets must be isolated")
	}
	path, err := SessionSocketPath("demo-b", session)
	if err != nil || !strings.Contains(path, session[:32]) {
		t.Fatalf("path=%q err=%v", path, err)
	}
}

func TestRejectsUnsafeNamespace(t *testing.T) {
	if ValidNamespace("../demo-a") || ValidNamespace("DEMO") || !ValidNamespace("browser-bridge") {
		t.Fatal("namespace validation failed")
	}
}
