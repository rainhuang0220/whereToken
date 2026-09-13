# Public Profile

`wheretoken profile build <dir>` writes a **public snapshot** of local coding-agent usage: JSON, GitHub-light and GitHub-dark preview SVGs, and a static live page. The interactive page is live in the browser. The usage data itself is a locally generated public snapshot, not a live cloud sync.

```bash
wheretoken profile build ./public-profile
wheretoken profile validate ./public-profile
```

Copy the directory to GitHub Pages (`/whereToken/profile/` on this project site). Then point a Profile README at the previews and the live page. The CLI never logs into GitHub, never commits, and never pushes.

Default build omits model breakdown and cost. Opt in:

```bash
wheretoken profile build ./public-profile --include-models --include-cost
```

`--today` / `--since` are rejected: the bundle always contains `all`, `today`, `7d`, `30d`, and `53w` from one scan and one clock.

Schema: [`docs/public-profile.schema.json`](./public-profile.schema.json). Token math is unchanged (`docs/token-accounting.md`). Missing usage is unavailable (`—`), never `$0`.

## Compatibility

`wheretoken card <path.svg>` still writes the 800×576 Vibe Coding Wall. It now projects from the same `ProfileSnapshot`. Prefer `profile build` for new READMEs.
