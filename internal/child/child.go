// Package child owns one stdio MCP process: start, JSON-RPC on stdin/stdout, stop.
package child

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// Config is the exact child to wrap. Env is applied as-is (no inheritance merge).
type Config struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// Message is one JSON-RPC object. ID is nil for notifications.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// Process is a running stdio MCP child.
type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	done   chan struct{}
	mu     sync.Mutex
	closed bool

	respMu sync.Mutex
	resp   map[string]chan Message
	notes  chan Message
}

// Start launches cfg and reads newline-delimited JSON-RPC from stdout.
// Stderr is forwarded to os.Stderr. Logs mixed onto stdout are skipped.
func Start(ctx context.Context, cfg Config) (*Process, error) {
	if cfg.Command == "" {
		return nil, errors.New("child command is empty")
	}
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Env = envList(cfg.Env)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &Process{
		cmd:   cmd,
		stdin: stdin,
		done:  make(chan struct{}),
		resp:  make(map[string]chan Message),
		notes: make(chan Message, 32),
	}
	go p.readLoop(stdout)
	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
		close(p.done)
	}()
	return p, nil
}

func envList(env map[string]string) []string {
	if len(env) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func (p *Process) readLoop(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		if len(msg.ID) > 0 && (len(msg.Result) > 0 || len(msg.Error) > 0) {
			key := string(msg.ID)
			p.respMu.Lock()
			ch := p.resp[key]
			p.respMu.Unlock()
			if ch != nil {
				ch <- msg
				continue
			}
		}
		select {
		case p.notes <- msg:
		default:
		}
	}
}

// Call writes one request and waits for the matching response id.
func (p *Process) Call(ctx context.Context, msg Message) (Message, error) {
	if len(msg.ID) == 0 {
		return Message{}, errors.New("request id required")
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return Message{}, err
	}
	ch := make(chan Message, 1)
	key := string(msg.ID)
	p.respMu.Lock()
	p.resp[key] = ch
	p.respMu.Unlock()
	defer func() {
		p.respMu.Lock()
		delete(p.resp, key)
		p.respMu.Unlock()
	}()

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return Message{}, errors.New("child closed")
	}
	_, err = p.stdin.Write(append(body, '\n'))
	p.mu.Unlock()
	if err != nil {
		return Message{}, err
	}

	select {
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case <-p.done:
		return Message{}, errors.New("child exited")
	case resp := <-ch:
		return resp, nil
	}
}

// Notify writes a notification (no response expected).
func (p *Process) Notify(ctx context.Context, msg Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errors.New("child closed")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	_, err = p.stdin.Write(append(body, '\n'))
	return err
}

// Close closes stdin and waits for the process to exit.
func (p *Process) Close() error {
	p.mu.Lock()
	if !p.closed {
		_ = p.stdin.Close()
	}
	p.mu.Unlock()
	<-p.done
	if p.cmd.ProcessState != nil && !p.cmd.ProcessState.Success() {
		return fmt.Errorf("child exit: %s", p.cmd.ProcessState)
	}
	return nil
}
