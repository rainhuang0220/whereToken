package cli

import (
	"context"
	"fmt"
	"io"
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
	case "publish":
		return a.runProfilePublish(flags, home)
	default:
		fmt.Fprintln(a.Stderr, "profile requires build, validate, palette, or publish")
		return ExitUsage
	}
}

func (a *App) runProfilePublish(flags Flags, home adapter.Home) int {
	pub := publicprofile.NewPublisher(home)
	if flags.ProductRepo != "" || flags.ProfileRepoFlag != "" || flags.PublishPages != "" || flags.PublishCheckout != "" {
		cfg, err := publicprofile.LoadPublishConfig(publicprofile.PublishConfigPath(home))
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(a.Stderr, "could not read the publish config")
			return ExitFail
		}
		if flags.ProductRepo != "" {
			cfg.ProductRepo = flags.ProductRepo
		}
		if flags.ProfileRepoFlag != "" {
			cfg.ProfileRepo = flags.ProfileRepoFlag
		}
		if flags.PublishPages != "" {
			cfg.PagesURL = flags.PublishPages
		}
		if flags.PublishCheckout != "" {
			abs, err := filepath.Abs(flags.PublishCheckout)
			if err != nil {
				fmt.Fprintln(a.Stderr, "could not resolve the checkout")
				return ExitFail
			}
			cfg.Checkout = abs
		}
		if err := pub.SaveConfig(cfg); err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitFail
		}
	}
	palette := flags.PublicPalette
	if palette == "" {
		palette = "newsprint"
	}
	pf, err := pub.Preflight(contextBackground(), palette)
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	writePreflight(a.Stdout, pf)
	if !flags.PublishYes {
		if pf.Idempotent {
			fmt.Fprintln(a.Stdout, "ALREADY_PUBLISHED")
			return ExitOK
		}
		if pf.Ready {
			fmt.Fprintln(a.Stdout, "WAITING_USER_APPROVAL")
			return ExitOK
		}
		return ExitFail
	}
	job, err := pub.Approve(contextBackground(), palette, func(job publicprofile.Job) {
		fmt.Fprintf(a.Stdout, "phase=%s %s\n", job.Phase, job.PhaseLabel)
	})
	writeJob(a.Stdout, job)
	if err != nil && job.Phase != publicprofile.PhaseVerified {
		if job.Phase == publicprofile.PhasePartiallyPublished {
			fmt.Fprintln(a.Stdout, "PARTIALLY_PUBLISHED")
		}
		return ExitFail
	}
	fmt.Fprintln(a.Stdout, "PUBLISHED")
	return ExitOK
}

func writePreflight(w io.Writer, pf publicprofile.Preflight) {
	fmt.Fprintf(w, "phase=%s %s\n", pf.Phase, pf.PhaseLabel)
	fmt.Fprintf(w, "palette=%s\n", pf.Palette)
	if pf.LivePalette != "" {
		fmt.Fprintf(w, "live_palette=%s\n", pf.LivePalette)
	}
	if pf.LocalPalette != "" {
		fmt.Fprintf(w, "local_palette=%s\n", pf.LocalPalette)
	}
	if pf.ProductRepo != "" {
		fmt.Fprintf(w, "product=%s %s public-profile/\n", pf.ProductRepo, pf.ProductBranch)
	}
	if pf.ProfileRepo != "" {
		fmt.Fprintf(w, "profile=%s %s %s\n", pf.ProfileRepo, pf.ProfileBranch, pf.ReadmePath)
	}
	if pf.SnapshotID != "" {
		fmt.Fprintf(w, "snapshot_id=%s\n", pf.SnapshotID)
	}
	if pf.AssetRevision != "" {
		fmt.Fprintf(w, "asset_revision=%s\n", pf.AssetRevision)
	}
	if pf.CacheKey != "" {
		fmt.Fprintf(w, "cache_key=%s\n", pf.CacheKey)
	}
	if pf.GitHubLogin != "" {
		fmt.Fprintf(w, "github=%s product_push=%t profile_push=%t\n", pf.GitHubLogin, pf.ProductPushRight, pf.ProfilePushRight)
	}
	if pf.Validation != "" {
		fmt.Fprintf(w, "validation=%s provenance=%s\n", pf.Validation, pf.Provenance)
	}
	fmt.Fprintln(w, "static_svg=true")
	fmt.Fprintln(w, pf.ReleaseNote)
	for _, subject := range pf.Ahead {
		fmt.Fprintf(w, "ahead=%s\n", subject)
	}
	for _, path := range pf.Files {
		fmt.Fprintf(w, "file=%s %s\n", path.Action, path.Path)
	}
	for _, edit := range pf.ReadmeEdits {
		if edit.Before == edit.After {
			continue
		}
		fmt.Fprintf(w, "readme=%s\n", edit.Slot)
		fmt.Fprintf(w, "  before=%s\n", edit.Before)
		fmt.Fprintf(w, "  after=%s\n", edit.After)
	}
	for _, block := range pf.Blockers {
		fmt.Fprintf(w, "blocker=%s\n", block)
	}
}

func writeJob(w io.Writer, job publicprofile.Job) {
	fmt.Fprintf(w, "result=%s %s\n", job.Phase, job.PhaseLabel)
	if job.ProductSHA != "" {
		fmt.Fprintf(w, "product_sha=%s\n", job.ProductSHA)
	}
	if job.ProductURL != "" {
		fmt.Fprintf(w, "product_url=%s\n", job.ProductURL)
	}
	if job.PagesRunURL != "" {
		fmt.Fprintf(w, "pages_run=%s\n", job.PagesRunURL)
	}
	if job.ProfileSHA != "" {
		fmt.Fprintf(w, "profile_sha=%s\n", job.ProfileSHA)
	}
	if job.ProfileURL != "" {
		fmt.Fprintf(w, "profile_url=%s\n", job.ProfileURL)
	}
	if job.Error != "" {
		fmt.Fprintf(w, "error=%s\n", job.Error)
	}
}

func contextBackground() context.Context { return context.Background() }

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
