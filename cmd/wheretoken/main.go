package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
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
		fmt.Fprintln(os.Stderr, "not implemented")
		os.Exit(2)
		return nil
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
