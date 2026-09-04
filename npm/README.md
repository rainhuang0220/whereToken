# wheretoken (npm)

Installs the [whereToken](https://github.com/rainhuang0220/whereToken) CLI without requiring Go. The postinstall script downloads the GitHub Release binary for this OS/arch.

This package is **experimental** — it is not the primary install channel and is **not yet published to the npm registry** (installing from the registry does not work today). The primary channels are the install script, Homebrew, and `go install`. Until this package is published, install with:

```bash
curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
```

If you already have Go:

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

Windows, macOS, and Linux. MIT. Read-only local ledgers; never prints JWTs, API keys, or cookies.
