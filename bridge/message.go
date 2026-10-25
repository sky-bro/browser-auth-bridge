package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const MaxMessageSize = 1 << 20

var (
	sessionPattern   = regexp.MustCompile(`^[a-f0-9]{64}$`)
	namespacePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
)

// Message is the transport envelope shared by browser-authenticated CLIs.
// Payload and Result are app-owned schemas and must be validated by each CLI.
type Message struct {
	Type      string          `json:"type"`
	Session   string          `json:"session,omitempty"`
	Service   string          `json:"service,omitempty"`
	Operation string          `json:"operation,omitempty"`
	AllowTab  bool            `json:"allowTab,omitempty"`
	NoOpenTab bool            `json:"noOpenTab,omitempty"`
	State     string          `json:"state,omitempty"`
	Error     string          `json:"error,omitempty"`
	Data      string          `json:"data,omitempty"`
	Cols      int             `json:"cols,omitempty"`
	Rows      int             `json:"rows,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

func NewSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func ValidSession(value string) bool {
	return sessionPattern.MatchString(value)
}

func ValidNamespace(value string) bool {
	return namespacePattern.MatchString(value)
}

func BrokerSocketPath(namespace string) (string, error) {
	if !ValidNamespace(namespace) {
		return "", fmt.Errorf("invalid bridge namespace %q", namespace)
	}
	return filepath.Join("/tmp", fmt.Sprintf("%s-%d-broker.sock", namespace, os.Getuid())), nil
}

func SessionSocketPath(namespace, session string) (string, error) {
	if !ValidNamespace(namespace) {
		return "", fmt.Errorf("invalid bridge namespace %q", namespace)
	}
	if !ValidSession(session) {
		return "", fmt.Errorf("invalid session id")
	}
	return filepath.Join("/tmp", fmt.Sprintf("%s-%d-%s.sock", namespace, os.Getuid(), session[:32])), nil
}
