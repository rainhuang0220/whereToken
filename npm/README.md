# wheretoken (npm)

Installs the [whereToken](https://github.com/rainhuang0220/whereToken) CLI without requiring Go. The postinstall script downloads the GitHub Release binary for this OS/arch.

This package is **not on the npm registry** yet. Until it is published, install with:

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

When a GitHub Release exists, postinstall verifies **SHA-256** against `checksums.txt` before installing.

```text
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

Windows, macOS, and Linux. MIT. Read-only local ledgers; never prints JWTs.
