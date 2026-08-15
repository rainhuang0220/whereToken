# wheretoken (npm)

Installs the [whereToken](https://github.com/rainhuang0220/whereToken) CLI without requiring Go. The postinstall script downloads the GitHub Release binary for this OS/arch.

```bash
npm install -g wheretoken
wheretoken
```

If no release exists yet, postinstall prints the `go install` line. When a release exists, postinstall verifies **SHA-256** against `checksums.txt` before installing.

```text
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

Windows, macOS, and Linux. MIT. Read-only local ledgers; never prints JWTs.
