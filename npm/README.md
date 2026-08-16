# wheretoken (npm)

Installs the [whereToken](https://github.com/rainhuang0220/whereToken) CLI without requiring Go. The postinstall script downloads the GitHub Release binary for this OS/arch.

This package is **not on the npm registry** yet. Until it is published, install with:

```bash
curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
```

If you already have Go:

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

Windows, macOS, and Linux. MIT. Read-only local ledgers; never prints JWTs.
