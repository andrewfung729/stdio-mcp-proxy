// Package httpmcp exposes one stdio child as a Streamable HTTP MCP endpoint.
package httpmcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/andrewfung729/stdio-mcp-proxy/internal/child"
)

// Server is a single-session Streamable HTTP front for one child.
type Server struct {
	proc *child.Process
	http *http.Server
	ln   net.Listener
	path string
}

// New binds addr (for example "127.0.0.1:0") and serves POST path.
func New(addr, path string, proc *child.Process) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{proc: proc, ln: ln, path: path}
	mux := http.NewServeMux()
	mux.HandleFunc(path, s.handle)
	s.http = &http.Server{Handler: mux}
	go func() { _ = s.http.Serve(ln) }()
	return s, nil
}

// Addr is the bound listen address.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// URL is the Streamable HTTP endpoint clients should call.
func (s *Server) URL() string { return "http://" + s.ln.Addr().String() + s.path }

// Close stops HTTP and the child.
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := s.http.Shutdown(ctx)
	cerr := s.proc.Close()
	return errors.Join(err, cerr)
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	accept := r.Header.Get("Accept")
	if !strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/event-stream") {
		http.Error(w, "not acceptable", http.StatusNotAcceptable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var msg child.Message
	if err := json.Unmarshal(body, &msg); err != nil || msg.JSONRPC == "" {
		http.Error(w, "invalid json-rpc", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var out child.Message
	if len(msg.ID) == 0 {
		if err := s.proc.Notify(ctx, msg); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	out, err = s.proc.Call(ctx, msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	_ = enc.Encode(out)
}
