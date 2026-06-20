# Sprite + `<use>` icon system (not inlined or `<img>`)

Icons are rendered as small `<svg>` stubs that reference shared `<symbol>` definitions in a single `web/static/sprite.svg` via `<use href="/static/sprite.svg#id">`. The sprite is embedded into the binary at build time (`web.Static` `embed.FS`) and served with long-cache headers. All theming flows through `currentColor`, so colour variants (`.icon--active`, `.gem .icon { color:#fff }`) need no per-variant files or markup forks.

## Context

This codebase renders HTML in Go via gomponents, has no template/asset pipeline beyond `task css:build` (CSS concatenation), and re-renders fragments over SSE via Datastar. Icons appear across chrome, nav, cards, tables, and per-row unit tokens — often dozens on one page, and per-row on tick-updated rosters. Earlier versions had three parallel icon systems: a `<symbol>` sprite, a duplicate inline `<g>` def library in `internal/ui/layout.go` (`svgDefs`) used only for crests/nav, and copy-pasted raw crest SVG markup. The sprite symbols also baked a shield contour into every "heraldic" glyph, hidden via a `--_shield-stroke: transparent` CSS hack — which is what motivated revisiting the whole approach.

## Considered Options

**Inline every icon's `<svg>` markup at each call site (the common "drop-in library" pattern).** A nav with 9 icons becomes several KB of duplicated path data on every full-page render, and the same duplication ships again inside every Datastar SSE fragment swap that includes icons. With rosters and per-tick re-renders, this is repeated payload for no benefit. Rejected for the fragment-heavy, SSE-driven layout this app uses.

**`<img src="/icons/<name>.svg">` per icon.** Breaks `currentColor` theming — the one mechanism that makes active/dim/gem-white/meaning-colour variants a single CSS rule each. Without it, each colour variant needs either a separate file or a CSS filter hack. Also adds one HTTP request per icon versus one cached fetch for the whole sprite. Rejected.

**A JS-rendered icon library (e.g. a React-style `<Icon>` that injects paths).** There is no JS component runtime here; interactivity is Datastar SSE + signals, not a vdom. Introducing a JS icon renderer for markup that Go already emits cleanly would add a runtime dependency to solve a problem that doesn't exist. Rejected.

**Sprite + `<use>` (chosen).** One ~14KB file fetched once and cached; every icon instance is a ~50-byte `<svg><use>` stub in HTML; `currentColor` theming works; gomponents emits it as a trivial `El("svg", El("use", …))`. Composition (a glyph over a shared frame) is a second `<use>` in the same `<svg>` — no path duplication. This fits the rendering model and pays off precisely under fragment re-renders.

## Consequences

- **Art and mechanism are decoupled.** Replacing an icon's artwork is editing path data inside one `<symbol>`; the rendering helper, the CSS classes, and every call site are untouched. A library import and a custom redraw are the same operation at this level — both paste path data into a `<symbol>`.
- **Symbol viewBoxes must be known to size icons correctly.** `internal/ui/icons.go` parses every `<symbol id="…">` and its `viewBox` from the embedded sprite once at init, so `Icon(id, sizePx)` derives height from the real aspect ratio (heraldic glyphs and the frame are 20×23, resources are 24×24 square, the sandglass is 10×14). Adding a new symbol with a new aspect ratio requires only that its `viewBox` is set; no Go change.
- **Composition requires a shared coordinate space.** A frame `<symbol>` and the glyphs composed over it must share the same viewBox (20×23) so a second `<use>` overlays the first with no offset math. Glyphs drawn for a different frame, or a frame with a different viewBox, cannot be composed without a per-pair transform.
- **`<use>` cross-document reference needs the sprite served from the same origin** (it is — it's a static asset). It does not work from a `data:` URI or a different origin without CORS considerations.
- **The dev icon-audit page at `/dev/icons`** (`internal/handlers/iconpreview`) parses the same embedded sprite to list every symbol, so it is the review surface for art changes. It is a temporary tool — delete it once icon cleanup is complete.
