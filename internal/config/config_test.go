package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFillsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proxy.toml")
	body := `
[child]
command = "/bin/echo"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Listen.Addr != "127.0.0.1:0" || f.Listen.Path != "/mcp" {
		t.Fatalf("listen: %+v", f.Listen)
	}
	if f.Child.Command != "/bin/echo" || f.Child.Env == nil {
		t.Fatalf("child: %+v", f.Child)
	}
}

func TestLoadRequiresCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proxy.toml")
	if err := os.WriteFile(path, []byte("[listen]\naddr = \"127.0.0.1:1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected missing command error")
	}
}

func TestLoadRejectsPathWithoutSlash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proxy.toml")
	body := "[listen]\npath = \"mcp\"\n[child]\ncommand = \"/bin/echo\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected path error")
	}
}

func TestLoadKeepsExplicitValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proxy.toml")
	body := `
[listen]
addr = "127.0.0.1:18765"
path = "/mcp"

[child]
name = "pencil"
command = "/bin/mcp"
args = ["--app", "desktop"]

[child.env]
FOO = "bar"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Listen.Addr != "127.0.0.1:18765" || f.Listen.Path != "/mcp" {
		t.Fatalf("listen: %+v", f.Listen)
	}
	if f.Child.Name != "pencil" || len(f.Child.Args) != 2 || f.Child.Env["FOO"] != "bar" {
		t.Fatalf("child: %+v", f.Child)
	}
}
