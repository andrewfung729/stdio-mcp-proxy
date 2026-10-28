// Package config loads the proxy settings from one TOML file.
package config

import (
	"errors"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/andrewfung729/stdio-mcp-proxy/internal/child"
)

// File is the on-disk proxy configuration.
type File struct {
	Listen Listen `toml:"listen"`
	Child  Child  `toml:"child"`
}

// Listen is the Streamable HTTP bind.
type Listen struct {
	Addr string `toml:"addr"`
	Path string `toml:"path"`
}

// Child is the stdio MCP process to wrap.
type Child struct {
	Name    string            `toml:"name"`
	Command string            `toml:"command"`
	Args    []string          `toml:"args"`
	Env     map[string]string `toml:"env"`
}

// Load reads path and fills defaults for omitted fields.
func Load(path string) (File, error) {
	var f File
	if _, err := toml.DecodeFile(path, &f); err != nil {
		return File{}, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := f.applyDefaults(); err != nil {
		return File{}, err
	}
	return f, nil
}

func (f *File) applyDefaults() error {
	if f.Listen.Addr == "" {
		f.Listen.Addr = "127.0.0.1:0"
	}
	if f.Listen.Path == "" {
		f.Listen.Path = "/mcp"
	}
	if f.Listen.Path[0] != '/' {
		return fmt.Errorf("listen.path must start with /: %q", f.Listen.Path)
	}
	if f.Child.Command == "" {
		return errors.New("child.command is required")
	}
	if f.Child.Env == nil {
		f.Child.Env = map[string]string{}
	}
	return nil
}

// Process is the child config the proxy launches.
func (f File) Process() child.Config {
	return child.Config{
		Name:    f.Child.Name,
		Command: f.Child.Command,
		Args:    f.Child.Args,
		Env:     f.Child.Env,
	}
}
