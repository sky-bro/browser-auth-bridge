package nativehost

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"

	"github.com/sky-bro/browser-auth-bridge/bridge"
)

type Config struct {
	Namespace      string
	ValidateAttach func(bridge.Message) error
}

func (config Config) validate() error {
	if !bridge.ValidNamespace(config.Namespace) {
		return fmt.Errorf("invalid bridge namespace %q", config.Namespace)
	}
	if config.ValidateAttach == nil {
		return fmt.Errorf("ValidateAttach is required")
	}
	return nil
}

func ReadMessage(reader io.Reader) (bridge.Message, error) {
	var size uint32
	if err := binary.Read(reader, binary.LittleEndian, &size); err != nil {
		return bridge.Message{}, err
	}
	if size == 0 || size > bridge.MaxMessageSize {
		return bridge.Message{}, fmt.Errorf("invalid native message size %d", size)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return bridge.Message{}, err
	}
	var message bridge.Message
	if err := json.Unmarshal(payload, &message); err != nil {
		return bridge.Message{}, fmt.Errorf("decode native message: %w", err)
	}
	return message, nil
}

func WriteMessage(writer io.Writer, message bridge.Message) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(payload) == 0 || len(payload) > bridge.MaxMessageSize {
		return fmt.Errorf("native message too large: %d", len(payload))
	}
	if err := binary.Write(writer, binary.LittleEndian, uint32(len(payload))); err != nil {
		return err
	}
	_, err = writer.Write(payload)
	return err
}

func Run(stdin io.Reader, stdout io.Writer, config Config) error {
	if err := config.validate(); err != nil {
		return err
	}
	first, err := ReadMessage(stdin)
	if err != nil {
		return err
	}
	if first.Type == "broker" {
		return runBroker(stdin, stdout, config)
	}
	if first.Type != "attach" || !bridge.ValidSession(first.Session) || config.ValidateAttach(first) != nil {
		return fmt.Errorf("first message must be a valid attach")
	}
	path, err := bridge.SessionSocketPath(config.Namespace, first.Session)
	if err != nil {
		return err
	}
	if err := validateSocket(path); err != nil {
		return err
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		return fmt.Errorf("connect CLI session: %w", err)
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(first); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		for {
			message, readErr := ReadMessage(stdin)
			if readErr != nil {
				_ = conn.Close()
				errCh <- readErr
				return
			}
			message.Session = first.Session
			if writeErr := json.NewEncoder(conn).Encode(message); writeErr != nil {
				_ = conn.Close()
				errCh <- writeErr
				return
			}
		}
	}()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), bridge.MaxMessageSize)
	for scanner.Scan() {
		var message bridge.Message
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			return fmt.Errorf("decode CLI message: %w", err)
		}
		if err := WriteMessage(stdout, message); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	select {
	case err := <-errCh:
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	default:
		return nil
	}
}

type brokerClient struct {
	conn    net.Conn
	encoder *json.Encoder
	mu      sync.Mutex
}

func (client *brokerClient) send(message bridge.Message) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.encoder.Encode(message)
}

func runBroker(stdin io.Reader, stdout io.Writer, config Config) error {
	path, _ := bridge.BrokerSocketPath(config.Namespace)
	listener, err := listenBroker(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(path)
	}()

	var clientsMu sync.Mutex
	clients := make(map[string]*brokerClient)
	var stdoutMu sync.Mutex
	writeNative := func(message bridge.Message) error {
		stdoutMu.Lock()
		defer stdoutMu.Unlock()
		return WriteMessage(stdout, message)
	}
	errCh := make(chan error, 2)
	go func() {
		for {
			message, readErr := ReadMessage(stdin)
			if readErr != nil {
				errCh <- readErr
				return
			}
			if !bridge.ValidSession(message.Session) {
				continue
			}
			clientsMu.Lock()
			client := clients[message.Session]
			if message.Type == "status" && message.State == "closed" {
				delete(clients, message.Session)
			}
			clientsMu.Unlock()
			if client != nil {
				_ = client.send(message)
				if message.Type == "status" && message.State == "closed" {
					_ = client.conn.Close()
				}
			}
		}
	}()
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				errCh <- acceptErr
				return
			}
			go serveBrokerClient(conn, config, &clientsMu, clients, writeNative)
		}
	}()

	err = <-errCh
	_ = listener.Close()
	clientsMu.Lock()
	for _, client := range clients {
		_ = client.conn.Close()
	}
	clientsMu.Unlock()
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
}

func serveBrokerClient(conn net.Conn, config Config, clientsMu *sync.Mutex, clients map[string]*brokerClient, writeNative func(bridge.Message) error) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)
	var attach bridge.Message
	if err := decoder.Decode(&attach); err != nil || attach.Type != "attach" || !bridge.ValidSession(attach.Session) || config.ValidateAttach(attach) != nil {
		return
	}
	client := &brokerClient{conn: conn, encoder: json.NewEncoder(conn)}
	clientsMu.Lock()
	if _, exists := clients[attach.Session]; exists {
		clientsMu.Unlock()
		return
	}
	clients[attach.Session] = client
	clientsMu.Unlock()
	defer func() {
		clientsMu.Lock()
		if clients[attach.Session] == client {
			delete(clients, attach.Session)
		}
		clientsMu.Unlock()
		_ = writeNative(bridge.Message{Type: "close", Session: attach.Session})
	}()
	if err := writeNative(attach); err != nil {
		return
	}
	for {
		var message bridge.Message
		if err := decoder.Decode(&message); err != nil {
			return
		}
		message.Session = attach.Session
		if err := writeNative(message); err != nil || message.Type == "close" {
			return
		}
	}
}

func listenBroker(path string) (net.Listener, error) {
	listener, err := net.Listen("unix", path)
	if err != nil {
		if removeErr := removeStaleBrokerSocket(path); removeErr != nil {
			return nil, fmt.Errorf("listen on browser broker socket: %w", removeErr)
		}
		listener, err = net.Listen("unix", path)
	}
	if err != nil {
		return nil, fmt.Errorf("listen on browser broker socket: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return listener, nil
}

func removeStaleBrokerSocket(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to replace non-socket broker path")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("refusing to replace broker socket owned by another user")
	}
	probe, dialErr := net.Dial("unix", path)
	if dialErr == nil {
		_ = probe.Close()
		return fmt.Errorf("browser broker is already running")
	}
	return os.Remove(path)
}

func validateSocket(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat CLI socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("refusing unsafe socket %s", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("refusing socket not owned by current user")
	}
	return nil
}
