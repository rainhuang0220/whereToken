# Public Profile theme publication

The dashboard glaze and the public Profile palette are different catalogs. This is the path that is actually built.

## Ownership

| State | Where it lives | Who it affects |
| --- | --- | --- |
| My Token application theme | `localStorage['wheretoken.theme']` | The local or hosted dashboard chrome |
| Owner public palette | `public-profile.json` next to `community.json` (`WHERETOKEN_PUBLIC_PROFILE_FILE` overrides the path) | The default written into the next local bundle |
| Published bundle palette | `<bundle>/presentation.json` | A clean visitor of that static bundle, plus the light and dark preview SVGs |
| Visitor choice | `localStorage['wt-visitor-palette']`, only after an explicit click or key | That browser |
| Share link | `?palette=cobalt\|magenta\|newsprint` | That URL |

A dashboard glaze such as `kiln` is rejected. Unknown palette ids and unknown `schema_version` values fall back to `newsprint`.

## Precedence on the public page

1. Valid `?palette=`
2. Valid `wt-visitor-palette`
3. `presentation.json` `public_palette` when `schema_version` is 1
4. `newsprint`

The page does not write the visitor choice until the visitor uses the Activity color control. Loading the owner default does not fill localStorage. The old `wt-wall-palette` key is ignored because earlier builds stored the default there automatically.

## Publication

`wheretoken profile build <dir>` reads the owner file, or `--public-palette` for that build only. `BundleWith` renders the light and dark SVGs from that palette and writes `presentation.json`. `snapshot_id` stays a hash of the usage snapshot. `asset_revision` includes the previews, page assets, and `presentation.json`.

My Token, on the local `wheretoken serve` themes page, posts the palette to `POST /api/public-profile`. The route is localhost-only, same as the other local APIs. The hosted app does not expose it. Apply saves the owner file. If a bundle already exists at `WHERETOKEN_PUBLIC_PROFILE_DIR`, the saved `bundle_dir`, or `./public-profile`, Apply rewrites that bundle from its current `profile.json` without a new scan.

Statuses returned by the API:

- `unconfigured` — no saved owner palette
- `saved_locally` — owner file saved, no local bundle to refresh
- `pending` — owner file and bundle palette or revision differ
- `ready_to_publish` — the local bundle matches the owner file
- `failed` — the save or bundle write failed

None of these mean GitHub Pages changed. The UI shows `wheretoken profile build <dir> --public-palette <id>`. `仅保存本机` is that local write.

`发布到我的 GitHub 主页` is a separate, explicit step. The public page only links to `http://127.0.0.1:8787/themes?public_palette=<id>&intent=publish#public-profile`. Opening that URL does not save or push. The local page runs a read-only preflight, then waits for `确认发布到这两个仓库`. That POST is localhost-only, same-origin, and carries a one-time CSRF token. It updates `public-profile/` without rescanning usage, fast-forwards `whereToken`, waits for the Pages run of that commit, checks the live palette and asset revision, then rewrites the three preview URLs in the configured personal `README.md`. `gh` and the existing git credentials stay outside the page. Repository names live in the local `profile-publish.json`, not in the public bundle. `wheretoken profile publish` prints the same preflight; `--yes` is the terminal form of the same approval. A failed README step after a verified Pages deploy stays `PARTIALLY_PUBLISHED` and can be retried without another product commit.

`预览公开 Profile` opens `/preview/public-profile/` on the local server. A palette that is not yet in the bundle is passed as `?palette=`, so the preview does not overwrite the owner file.

## Newsprint sheet

The public page background is `assets/newsprint-surface.jpg`, baked by `go run ./scripts/gennewsprint` from the CC0 Paper001 color and displacement extracts. One height field produces the normals and the roughness. Shading is linear-light Oren–Nayar plus a very small specular term. The JPEG is the shipped material. Provenance and the art-direction scales are in `scripts/gennewsprint/vendor/SOURCE.md`.

## Cache

A palette-only rebuild changes `asset_revision` and the README query `SNAPSHOT_HEX-ASSET_HEX`. It does not change `snapshot_id`.

## Diagram

```text
My Token 公开 Profile
        |  POST /api/public-profile
        v
owner public-profile.json
        |  profile build / Apply refresh
        v
bundle presentation.json + preview-light.svg + preview-dark.svg
        |
        +--> clean visitor
        +--> ?palette= or wt-visitor-palette overrides the view only
```
