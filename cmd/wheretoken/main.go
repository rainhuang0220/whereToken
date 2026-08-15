package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/httpapi"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "scan"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd = args[0]
		args = args[1:]
	}
	switch cmd {
	case "scan":
		fs := flag.NewFlagSet("scan", flag.ContinueOnError)
		_ = fs.Bool("json", true, "write JSON summary to stdout")
		home := fs.String("home", "", "override home directory")
		if err := fs.Parse(args); err != nil {
			return err
		}
		r := scan.Run(resolveHome(*home), scan.AllAdapters())
		return scan.EncodeSummary(os.Stdout, r)
	case "sources":
		fs := flag.NewFlagSet("sources", flag.ContinueOnError)
		home := fs.String("home", "", "override home directory")
		if err := fs.Parse(args); err != nil {
			return err
		}
		r := scan.Run(resolveHome(*home), scan.AllAdapters())
		for _, root := range r.Roots {
			fmt.Printf("%s\t%s\n", root.ID, root.Path)
		}
		return nil
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ContinueOnError)
		port := fs.Int("port", 8787, "port")
		home := fs.String("home", "", "override home directory")
		if err := fs.Parse(args); err != nil {
			return err
		}
		h := resolveHome(*home)
		start, end := *port, *port
		if *port == 8787 {
			end = 8797
		}
		var lastErr error
		for p := start; p <= end; p++ {
			addr := fmt.Sprintf("127.0.0.1:%d", p)
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				lastErr = err
				continue
			}
			fmt.Fprintf(os.Stderr, "http://%s\n", addr)
			srv := &http.Server{Addr: addr, Handler: httpapi.NewMux(h)}
			return srv.Serve(ln)
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("no port available")
		}
		return lastErr
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func resolveHome(override string) adapter.Home {
	if override != "" {
		return testhome.New(override)
	}
	return scan.RealHome()
}
