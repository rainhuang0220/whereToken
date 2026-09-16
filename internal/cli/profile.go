package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func (a *App) runProfile(flags Flags, home adapter.Home) int {
	switch flags.ProfileAction {
	case "validate":
		snap, err := publicprofile.LoadFile(flags.ProfilePath)
		if err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitFail
		}
		if flags.Production {
			if err := publicprofile.ValidateProduction(snap, flags.AllowPartial); err != nil {
				fmt.Fprintln(a.Stderr, err.Error())
				return ExitFail
			}
			if flags.AllowPartial {
				if warn := publicprofile.ProductionWarning(snap); warn != "" {
					fmt.Fprintln(a.Stderr, warn)
				}
			}
		}
		fmt.Fprintln(a.Stdout, "ok")
		return ExitOK
	case "build":
		return a.runProfileBuild(flags, home)
	default:
		fmt.Fprintln(a.Stderr, "profile requires build or validate")
		return ExitUsage
	}
}

func (a *App) runProfileBuild(flags Flags, home adapter.Home) int {
	res := a.doScan(home, flags.Quiet, flags.Offline, false)
	snap, err := publicprofile.Build(publicprofile.Input{
		Events:        res.Events,
		Turns:         res.Turns,
		Now:           a.Now(),
		Loc:           a.Loc,
		Version:       a.Version,
		IncludeModels: flags.IncludeModels,
		IncludeCost:   flags.IncludeCost,
		PortraitSeed:  a.PortraitSeed(home),
		Offline:       flags.Offline || res.Offline,
		Errors:        res.Errors,
	})
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	if err := publicprofile.Validate(snap); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	files, err := publicprofile.Bundle(snap)
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	if err := replaceGenerated(flags.ProfilePath, files); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	abs, err := filepath.Abs(flags.ProfilePath)
	if err != nil {
		abs = flags.ProfilePath
	}
	fmt.Fprintf(a.Stdout, "wrote %s\n", abs)
	return ExitOK
}

func replaceGenerated(dir string, files map[string][]byte) error {
	if dir == "" {
		return fmt.Errorf("profile: missing path")
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		return err
	}
	for _, name := range publicprofile.GeneratedFiles {
		payload, ok := files[name]
		if !ok {
			continue
		}
		dest := filepath.Join(dir, name)
		if err := replaceCardFile(dest, payload); err != nil {
			return err
		}
	}
	for _, name := range publicprofile.RetiredGeneratedFiles {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
