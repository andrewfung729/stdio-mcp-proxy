// Command stdio-mcp-proxy exposes one stdio MCP child over Streamable HTTP.
// Listen address, path, and the child command come from a TOML file (-config).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/andrewfung729/stdio-mcp-proxy/internal/child"
	"github.com/andrewfung729/stdio-mcp-proxy/internal/config"
	"github.com/andrewfung729/stdio-mcp-proxy/internal/httpmcp"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("stdio-mcp-proxy", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "stdio-mcp-proxy.toml", "path to the TOML config")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	proc, err := child.Start(ctx, cfg.Process())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	srv, err := httpmcp.New(cfg.Listen.Addr, cfg.Listen.Path, proc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(srv.URL())
	<-ctx.Done()
	_ = srv.Close()
	return 0
}
