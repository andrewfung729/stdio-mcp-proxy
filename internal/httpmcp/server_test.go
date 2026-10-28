package httpmcp

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/andrewfung729/stdio-mcp-proxy/internal/child"
)

func TestInitializeProxied(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-mcp")
	body := "#!/bin/sh\nwhile IFS= read -r line; do\n  case \"$line\" in\n    *'\"method\":\"initialize\"'*)\n      printf '%s\\n' '{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2025-03-26\",\"capabilities\":{},\"serverInfo\":{\"name\":\"fake\",\"version\":\"0\"}}}'\n      ;;\n  esac\ndone\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	proc, err := child.Start(t.Context(), child.Config{Command: script})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New("127.0.0.1:0", "/mcp", proc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`
	req, err := http.NewRequest(http.MethodPost, "http://"+srv.Addr()+"/mcp", strings.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	var msg child.Message
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		t.Fatal(err)
	}
	if len(msg.Result) == 0 || !strings.Contains(string(msg.Result), "2025-03-26") {
		t.Fatalf("result: %s", msg.Result)
	}
}

func TestPencilInitialize(t *testing.T) {
	const bin = "/Applications/Pen.app/Contents/Resources/app.asar.unpacked/out/mcp-server-darwin-arm64"
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("pencil binary is darwin/arm64")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skip(err)
	}
	proc, err := child.Start(t.Context(), child.Config{
		Name:    "pencil",
		Command: bin,
		Args:    []string{"--app", "desktop"},
		Env:     map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New("127.0.0.1:0", "/mcp", proc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"smoke","version":"0.0.1"}}}`
	req, err := http.NewRequest(http.MethodPost, "http://"+srv.Addr()+"/mcp", strings.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(bufio.NewReader(resp.Body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, raw)
	}
	var msg child.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(msg.Result), `"name":"pencil"`) {
		t.Fatalf("result: %s", msg.Result)
	}
	t.Logf("listen %s initialize ok", srv.Addr())
}
