# macOS signing and notarization (names only)

GitHub Release binaries stay **unsigned** until these repository secrets exist. The release workflow **skips signing** when any of them is missing, and still publishes the unsigned archives as today. When all five are present, GoReleaser signs with Developer ID Application and notarizes via the App Store Connect API (cross-platform [quill](https://goreleaser.com/customization/sign/notarize/), so the job can stay on `ubuntu-latest`).

**Create Repository secrets** in `whereToken` → Settings → Secrets and variables → Actions. Use these **exact names**. Put values only in the GitHub UI. **Never paste certificates, passwords, `.p12`, `.p8`, or Apple IDs into chat, issues, or commit messages.**

| Secret name | What to store (in GitHub, not here) |
|-------------|-------------------------------------|
| `MACOS_SIGN_P12` | Developer ID Application certificate exported as `.p12`, then `base64` (`base64 -w0 < Certificates.p12` on Linux; `base64 -i Certificates.p12` on macOS) |
| `MACOS_SIGN_PASSWORD` | Password used when exporting that `.p12` |
| `MACOS_NOTARY_KEY` | App Store Connect API key `.p8`, then `base64` |
| `MACOS_NOTARY_KEY_ID` | Key ID (also in the `AuthKey_XXXXXXXXXX.p8` filename) |
| `MACOS_NOTARY_ISSUER_ID` | Issuer UUID from App Store Connect (Users and Access → Integrations → Team Keys) |

Empty Actions secrets on a new repo is expected. Adding the names above is what turns signing on.

Windows Authenticode is **not wired**. macOS first; Windows later.

These names match GoReleaser’s 2026 `notarize.macos` environment (`isEnvSet "MACOS_SIGN_P12"`). The workflow copies them into the job env only when all five are non-empty, so an incomplete set does not fail the release.
