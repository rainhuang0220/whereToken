package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	case "palette":
		return a.runProfilePalette(flags, home)
	default:
		fmt.Fprintln(a.Stderr, "profile requires build, validate, or palette")
		return ExitUsage
	}
}

func (a *App) runProfilePalette(flags Flags, home adapter.Home) int {
	path := a.ownerConfigPath(home)
	if strings.TrimSpace(flags.ProfilePath) == "" {
		owner := publicprofile.FallbackOwner(path)
		fmt.Fprintf(a.Stdout, "public_palette=%s\nrevision=%s\n", owner.PublicPalette, owner.Revision)
		if owner.BundleDir != "" {
			fmt.Fprintf(a.Stdout, "bundle=%s\n", owner.BundleDir)
		}
		status := publicprofile.StatusUnconfigured
		if publicprofile.OwnerSaved(owner) {
			status = publicprofile.StatusSavedLocally
		}
		fmt.Fprintf(a.Stdout, "status=%s\n", status)
		return ExitOK
	}
	owner := publicprofile.FallbackOwner(path)
	updated := a.Now().UTC().Format(time.RFC3339)
	if err := owner.ApplyPalette(flags.ProfilePath, updated); err != nil {
		fmt.Fprintln(a.Stderr, "public palette must be cobalt, magenta, or newsprint")
		return ExitUsage
	}
	if err := publicprofile.SaveOwner(path, owner); err != nil {
		fmt.Fprintln(a.Stderr, "could not save the public palette")
		return ExitFail
	}
	fmt.Fprintf(a.Stdout, "saved public_palette=%s revision=%s\n", owner.PublicPalette, owner.Revision)
	fmt.Fprintln(a.Stdout, publicprofile.ExportCommand(owner.BundleDir, owner.PublicPalette))
	return ExitOK
}

func (a *App) ownerConfigPath(home adapter.Home) string {
	lookup := a.LookupEnv
	if lookup == nil {
		lookup = os.Getenv
	}
	if v := strings.TrimSpace(lookup("WHERETOKEN_PUBLIC_PROFILE_FILE")); v != "" {
		return v
	}
	return publicprofile.ConfigPathIn(home)
}

func (a *App) resolvePresentation(flags Flags, home adapter.Home) publicprofile.Presentation {
	owner, err := publicprofile.LoadOwner(a.ownerConfigPath(home))
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintln(a.Stderr, "public profile palette config ignored; using newsprint")
		}
		owner = publicprofile.DefaultOwner()
	}
	p := owner.Presentation()
	if flags.PublicPalette != "" {
		p.PublicPalette = flags.PublicPalette
		if !publicprofile.OwnerSaved(owner) {
			p.Revision = "0"
			p.UpdatedAt = ""
		}
	}
	return publicprofile.CoercePresentation(p)
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
	files, err := publicprofile.BundleWith(snap, a.resolvePresentation(flags, home))
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	if err := publicprofile.Install(flags.ProfilePath, files); err != nil {
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
