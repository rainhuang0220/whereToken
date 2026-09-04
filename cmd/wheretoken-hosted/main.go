package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/hosted"
)

var version = "dev"

func main() {
	os.Exit(run(os.Getenv))
}

func run(getenv func(string) string) int {
	cfg, err := hosted.ConfigFromEnv(getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	cfg.Version = resolveVersion(version)
	store, err := hosted.Open(cfg.MySQLDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := store.Migrate(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           hosted.NewMux(hosted.MuxOptions{Version: cfg.Version, Store: store, Config: cfg}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Fprintf(os.Stderr, "wheretoken-hosted %s %s\n", cfg.Version, ln.Addr())
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	return 0
}

func resolveVersion(ldflag string) string {
	if ldflag != "" && ldflag != "dev" {
		return strings.TrimSuffix(strings.TrimSpace(ldflag), "+dirty")
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimSuffix(v, "+dirty")
		}
	}
	return "dev"
}
