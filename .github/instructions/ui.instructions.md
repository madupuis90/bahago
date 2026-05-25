---
description: "Use when working with gomponents, gomponents-datastar, datastar signals, SSE responses, data-bind, data-on, data-show, reactive UI, or HTML templates in Go"
---

# UI Stack Reference

This project renders all HTML in Go using gomponents. There is no template language. Datastar handles reactivity via SSE.

## Imports

```go
import (
    . "maragu.dev/gomponents"               // Node, El, Attr, Text, If, Map, Group, etc.
    . "maragu.dev/gomponents/components"   // HTML5(), Classes{}
    . "maragu.dev/gomponents/html"         // Div(), P(), Input(), Class(), Href(), etc.
    ds "maragu.dev/gomponents-datastar"    // reactive attributes — kept aliased for clarity
    "github.com/starfederation/datastar-go/datastar" // SSE + ReadSignals
    . "bahago/internal/ui"               // HomeLayout(), KingdomLayout(), AlertError(), AlertSuccess(), shared components — dot-imported
)
```

All gomponents packages and internal UI packages use dot-imports so their functions read like a templating language:
```go
// Good
Div(Class("container"), H1(Text("Hello")))

// Not used in this project
g.El("div", html.Class("container"), g.Text("Hello"))
```

## gomponents core (dot-imported)

| Function | Description |
|----------|-------------|
| `Node` | Interface — anything with `Render(io.Writer) error` |
| `El(tag, ...Node)` | Create any element not in html package |
| `Attr(name, value...)` | Create any attribute not in html package |
| `Text(s)` | HTML-escaped text node |
| `Raw(s)` | Unescaped HTML (use carefully) |
| `If(bool, Node)` | Render node only if true — arguments are always evaluated |
| `Iff(bool, func() Node)` | Lazy conditional — node is only evaluated when condition is true |

> **`If` vs `Iff`**: Use `Iff` when the condition guards a nil pointer — i.e., the pattern is `x != nil` and the node expression dereferences `x`. Using `If` there will panic because Go evaluates all arguments before calling the function. For conditions that don't involve nil-guarding (e.g., `len(slice) > 0`, a plain bool), `If` is fine and is less verbose.
| `Map(slice, func(T) Node)` | Map a slice to nodes |
| `Group([]Node)` | Flatten multiple nodes into one |

## gomponents/html elements (dot-imported)

Common elements, all accept `...Node` children:

```
A  Article  Aside  Body  Button  Datalist  Details  Dialog
Div  Dl  Dt  Dd  Em  FieldSet  Figure  Footer  Form
H1–H6  Head  Header  Hr  HTML  IFrame  Img  Input
Label  Legend  Li  Link  Main  Meta  Nav  NoScript
Ol  Option  P  Pre  Progress  Script  Section  Select
Span  Strong  Summary  Table  TBody  Td  TextArea  TFoot
Th  THead  TitleEl  Tr  Ul
```

Void elements (`Br`, `Hr`, `Img`, `Input`, `Link`, `Meta`) ignore non-attribute children.

## gomponents/html attributes (dot-imported)

All return `Node` and are passed as children to elements:

```
Accept  Action  Alt  Aria(name,v)  AutoComplete  Charset  Checked
Class(v)  Cols  ColSpan  Content  CrossOrigin  Data(name,v)
DateTime  Defer  Dir  Disabled  Download  Draggable  EncType
For  FormAction  FormAttr  Height  Hidden  Href  ID  Integrity
Lang  List  Loading  Max  MaxLength  Method  Min  MinLength
Multiple  Muted  Name  Pattern  Placeholder  ReadOnly  Rel
Required  Role  Rows  RowSpan  Scope  Selected  Src  SrcSet
Step  Style(v)  TabIndex  Target  Title  Type  Value  Width
```

Boolean attrs take no arguments: `Disabled()`, `Required()`, `Checked()`, `ReadOnly()`, `Multiple()`, `Selected()`.

## gomponents/components (dot-imported)

```go
// Full HTML5 document with doctype
HTML5(HTML5Props{
    Title:       "Page Title",
    Description: "optional meta description",
    Language:    "en",
    Head:        []Node{...},
    Body:        []Node{...},
})

// Conditional classes map — renders as class attribute
Classes{
    "active":   isActive,
    "disabled": isDisabled,
    "text-red": hasError,
}
```

> **Never mix `Class()` and `Classes{}` on the same element.** Both render as a `class="..."` attribute independently, producing two `class` attributes in the HTML — which is invalid. Browsers silently ignore all but the first, so the `Classes{}` conditions would have no effect. Always fold everything into a single `Classes{}`, using `true` for static classes:
> ```go
> // Wrong — two class attributes rendered
> Td(Class("allocation-total"), Classes{"text-positive": net > 0})
>
> // Correct — one class attribute
> Td(Classes{"allocation-total": true, "text-positive": net > 0, "text-negative": net < 0})
> ```

## Project Layout convention

- Home/auth/chat pages use `HomeLayout(r, title, content...)` — reads user from context
- Kingdom game pages use `KingdomLayout(r, title, content...)` — reads user and kingdom from context
- Content functions return `Node` with only domain data as parameters — no user, path, or request
- Handlers call the appropriate layout function directly and chain `.Render(w)` for full-page responses
- Components extracted to `internal/ui/` only when used by 2+ handler packages
- **`internal/ui` is dot-imported** — `HomeLayout`, `KingdomLayout`, `AlertError`, `AlertSuccess`, nav components, and shared helpers are used without a package prefix

```go
// A content function — takes domain data, returns just the body
func myContent(data SomeData) Node {
    return Group([]Node{
        H1(Text("Hello")),
        myCard(data),  // reusable component in same or layout package
    })
}

// Handler assembles the full page
func (h *handler) handleMyPage() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        KingdomLayout(r, "My Page", myContent(data)).Render(w)
    }
}

// A reusable component function — same pattern
func myCard(data SomeData) Node {
    return Div(Class("card"),
        H2(Text(data.Title)),
        P(Text(data.Body)),
    )
}
```

## Component extraction over duplication

Whenever a piece of HTML would appear more than once, extract it into a function returning `Node`. There is no cost to doing this — functions are the templating primitive. Prefer small, focused components over large monolithic page functions.

```go
// Instead of repeating this structure in multiple places:
func alertBanner(msg string) Node {
    return Div(Class("alert"),
        P(Text(msg)),
    )
}
```

If a component is used only within one feature package, keep it in that package. Move it to `internal/ui/` only when a second package needs it.

## Styling

All styles are hand-written in `web/static/styles.css`. There are no CSS frameworks (no Tailwind, Bootstrap, etc.). Classes applied via `Class("foo")` must exist in that file.

### CSS file structure

`styles.css` is organized into named sections, ordered from most general to most specific. Each section is introduced by a banner comment of the form:

```css
/* =============================================================================
   Section name
   ============================================================================= */
```

Always add new rules in the correct section. Current order:

```
Reset                — *, box-sizing
Tokens               — :root custom properties
Base                 — html, body, a, input (element selectors only)
Home shell           — top-nav, content-area, side-nav, nav-group
Shared components    — .panel, .btn, .btn-text, .form-fields, .password-field, .alert-*
Auth                 — .auth-card and auth-specific styles
Kingdom chrome       — .kingdom-page, parchment helpers (.uppr, .rubric, .marg, .italic, .rule-dbl, .shield, .sandglass), .topbar, .bottom-nav, .nav-stone — defined before any feature that uses them
Kingdom overview     — .folio, .overview-grid, .chronicle, .demesne, .stat-row
Allocation           — .allocation-* (grid table + slider)
Buildings            — .buildings-* / .building-*
Units                — .units-* / .unit-*
World map            — .map-*
Army                 — .army-*
Home (flip card)     — .flip-* (home/about page widget)
Messages             — .messages-* / .message-*
Guild                — .guild-* / .guilds-*
Prayers              — .prayer-* / .prayers-*
Utilities            — .text-positive, .text-negative (single-purpose overrides) — last
```

New feature modules get their own named section, placed between the existing feature sections and `Utilities`. Shared components (used by 2+ features) belong in `Shared components`. Anything shared across all kingdom pages (chrome, parchment typography helpers, icons) belongs in `Kingdom chrome` so it is defined before the features that reference it.

**When generating new UI**, omit styles unless explicitly asked — focus on structure and correctness. When styles are needed later, they go in `web/static/styles.css`. Do not suggest inline styles or CSS-in-Go approaches.

**Use CSS variables** for any value that appears more than once or is likely to be reused — especially spacing sizes, colors, dimensions, and border definitions. Define them in `:root` in `styles.css`. For example, prefer `var(--spacing-md)` over a hardcoded `1rem`, and `var(--border)` over a repeated `1px solid var(--border-color)`.

**Treat each class as a component** — nest pseudo-classes (`:hover`, `:active`, `:focus`) and child element selectors inside the parent rule using native CSS nesting (`&`). Do not write them as separate top-level rules:
```css
/* Good */
.allocation-btn {
  background: var(--panel-bg);

  &:hover { box-shadow: ...; }
  &:active { transform: translateY(1px); }
}

/* Bad */
.allocation-btn { background: var(--panel-bg); }
.allocation-btn:hover { box-shadow: ...; }
.allocation-btn:active { transform: translateY(1px); }
```

**CSS naming follows BEM modifier convention** — use double-dash (`--`) for state and variant modifiers, single-dash (`-`) for sub-elements and structural parts:
- `.btn` — block
- `.btn--locked` — modifier (state/variant of the block) ✓
- `.building-card--locked` — modifier of `.building-card` ✓
- `.map-cell-content` — sub-element of `.map-cell` (structural part, single-dash) ✓
- `.map-nav-btn--disabled` — modifier (state) ✓
- Never `.btn-locked` or `.building-locked` for modifiers — those look like separate blocks

## Heading structure

Each page has exactly one `H1` — the page title. Never use heading elements (`H2`–`H6`) purely for visual styling (bold label, larger font inside a panel). Use `P(Class("panel-title"), ...)` instead, styled via `.panel-title` in CSS.

```go
// Good — H1 once per page, panel sections use .panel-title
H1(Text("Army")),
Div(Class("army-section panel"),
    P(Class("panel-title"), Text("Active Campaigns")),
    ...
)

// Bad — H2 used only for visual weight, no semantic hierarchy
Div(Class("army-section panel"),
    H2(Text("Active Campaigns")),
    ...
)
```

Never skip levels: `H1` → `H2` → `H3`. If a nested element needs a label, check whether a semantic heading is actually appropriate before reaching for an `H` element.

## No string literals for paths

All route path constants live in `internal/routes/`. Never write path strings inline in `Href`, `Action`, `ds.On`, `datastar.GetSSE`/`PostSSE`, or route registration — always reference the constant.

```go
// internal/routes/routes.go
const (
    LoginPath    = "/login"
    RegisterPath = "/register"
    LogoutPath   = "/logout"
)

// Route registration uses the same constants
r.HandleFunc("GET "+routes.LoginPath, h.loginPage())
r.HandleFunc("POST "+routes.LoginPath, h.login())

// Templates reference the same constants — no duplication
A(Href(routes.LoginPath), Text("Login"))
ds.On("click", datastar.PostSSE(routes.LoginPath))
Form(Method("POST"), Action(routes.LogoutPath), ...)
```

## Datastar attributes must be owned by the element they are placed on

A `ds.*` call produces an HTML attribute node. Always pass it as a child of the specific element you intend to attach it to — never pass it into a component function (like `Layout`) expecting it to land on some inner element. You cannot see what element a component renders internally, so the binding is implicit and fragile.

```go
// Wrong — ds.Init ends up on whatever Layout wraps body content in (<main>)
Layout(LayoutArgs{...},
    ds.Init(datastar.GetSSE(LoadPath)),
    Div(...),
)

// Correct — ds.Init is explicitly on the Div that owns the SSE connection
Layout(LayoutArgs{...},
    Div(ds.Init(datastar.GetSSE(LoadPath)),
        ...,
    ),
)
```

The same applies to `ds.Signals`, `ds.On`, `ds.Bind`, and all other `ds.*` attributes — they belong on an element you define in the same call expression.

## Element IDs

Only add an `ID()` to an element when it will be **explicitly targeted** — either:
- by Datastar SSE (`PatchElementGostar` uses the root element's ID to find its DOM target, `WithSelectorID` references it explicitly)
- by a CSS rule with an `#id` selector

With fat-morph (`WithSelector("html")`), the entire page is morphed without needing individual IDs on content containers. Do not add an ID just because an element is a major section or landmark.

When an ID is needed, write it as a plain string literal inline — do not declare a `const`. The ID string only needs to appear in the component function that owns the element (and the SSE patch call passes the component, not the ID string).

```go
// Wrong — ID not targeted by SSE or CSS; const is unnecessary overhead
const contentID = "my-content"
func myContent(...) Node {
    return Div(ID(contentID), ...)
}

// Wrong — ID needed for SSE target but wrapped in a const
const errorID = "my-error"
func errorComponent(err error) Node {
    return Div(ID(errorID), Text(msg))
}

// Correct — ID only where needed, inline string
func errorComponent(err error) Node {
    return Div(ID("my-error"), Text(msg))
}
// SSE handler passes the component — ID string never repeated
sse.PatchElementGostar(errorComponent(err))
```

## Signals and input structs

Use a plain struct with `json` tags for reading client signals via `datastar.ReadSignals`. Field types are the raw Go types (`string`, `int`, `bool`, etc.).

```go
// Input struct — plain types, json tags define signal names
type pageInput struct {
    WoodPct int    `json:"wood_pct"`
    Name    string `json:"name"`
}

// Handler — read values from client
input := &pageInput{}
datastar.ReadSignals(r, input)
// use input.WoodPct, input.Name directly

// Content function — initialise signals with a map literal
ds.Signals(map[string]any{
    "wood_pct": kingdom.WoodPct,
    "name":     "",
})
ds.Bind("wood_pct")           // two-way bind
ds.Text("$wood_pct")          // reactive text
ds.On("click", "$wood_pct++") // JS expression
```

Signal name strings appear in two places: the `json` tag and the `ds.Bind`/`ds.Text`/`ds.On` call. Keep them adjacent inside a single file; this is always the case for self-contained page components.

## gomponents-datastar (`ds`) — reactive attributes

Signals use `$signalName` syntax in expressions. Signal names cannot start with or contain `__`.

### Signals

```go
// Initialize signals on an element (values defined later in DOM override earlier)
// Always use multi-line map literals, even for a single entry.
ds.Signals(map[string]any{
    "count": 0,
    "open":  false,
})

// Two-way bind input/select/textarea to a signal
ds.Bind("signalName")
```

**Do not use `ds.Signals` to initialize signals that are already covered by `ds.Bind`.** `ds.Bind` on an input creates the signal automatically with the element's current value. Only use `ds.Signals` when:
- The signal has a non-zero/non-empty initial value (e.g. a token pre-populated from the URL)
- The signal has no corresponding `ds.Bind` (e.g. a `showPassword` bool toggled by a button)

> **`<select>` elements with string option values must always be pre-initialized in `ds.Signals`.** Datastar preserves the type of a pre-existing signal — if the signal is already a string, bound values stay strings. But if no signal exists yet, Datastar has no type to preserve and coerces the element's value numerically, turning `"recruit"` into `NaN`. Setting `Value("recruit")` as an HTML attribute does not help — the type comes from the signal store, not the element. Always declare the signal in `ds.Signals` before the select's `ds.Bind` is encountered in the DOM:
> ```go
> ds.Signals(map[string]any{
>     "unit_type": unitNames[0], // seeds the signal as a string
> }),
> Select(ds.Bind("unit_type"), ...)
> ```

> **`<select>` elements bound to an `int` signal need no JavaScript coercion.** Datastar's bind plugin reads `typeof <current signal value>` on every change event and calls `+el.value` (numeric coercion) automatically when the existing signal is a number. Initialize the signal with a Go `int` value and use `ds.Bind` alone — never write `$field = parseInt(evt.target.value, 10)` manually:
> ```go
> // Correct — int signal, ds.Bind handles coercion
> ds.Signals(map[string]any{
>     "target_kingdom_id": members[0].KingdomID, // Go int → JSON number
> }),
> Select(ds.Bind("target_kingdom_id"), ...)
>
> // Wrong — parseInt is redundant and adds noise
> Select(
>     ds.On("change", "$target_kingdom_id = parseInt(evt.target.value, 10)"),
>     ...
> )
> ```

> **In `ds.On` expressions, the DOM event is `evt`, not `$event`.** Datastar compiles `data-on-*` expressions as `Function("el", "$", "__action", "evt", ..., expression)`. Writing `$event` triggers Datastar's signal-substitution and becomes `$['event']` — an undefined signal — at runtime. Always use `evt`:
> ```go
> // Correct
> ds.On("change", "doSomething(evt.target.checked)")
>
> // Wrong — $event is not the DOM event
> ds.On("change", "doSomething($event.target.checked)")
> ```

```go
// Wrong — email/password are initialized to "" by ds.Bind; ds.Signals is redundant
ds.Signals(map[string]any{
    "email":    "",
    "password": "",
}),
Input(Type("email"), ds.Bind("email")),

// Correct — only declare signals that ds.Bind won't create, or have real initial values
ds.Signals(map[string]any{
    "token":        token,  // real value from URL
    "showPassword": false,  // no ds.Bind for this one
}),
Input(ds.Bind("password"), ...),
```

### Reactivity

```go
ds.Show("$visible")                        // show/hide element
ds.Text("$count")                          // reactive text content
ds.Attr("disabled", "$loading", "title", "$tooltip")  // key-value pairs
ds.Class("active", "$isActive", "hidden", "$hidden")  // key-value pairs — wrap hyphenated names in single quotes: ds.Class("'flip-card--flipped'", "$flipped")
ds.Style("color", "$usingRed ? 'red' : 'green'")      // key-value pairs
ds.Computed("total", "$price * $quantity")             // read-only derived signal
ds.Effect("$debug = $a + $b")             // side effects on signal change
```

### Events

```go
ds.On("click", "$count++")
ds.On("click", "$count++", ds.ModifierPrevent)       // with modifier
ds.On("submit", datastar.PostSSE("/endpoint"))        // trigger SSE request
ds.OnIntersect("$visible = true")                     // intersection observer
ds.OnInterval("$tick++", ds.Duration(500*time.Millisecond)) // repeating
ds.Init(datastar.GetSSE("/load"))                     // run on DOM load
```

### Loading indicators

```go
// In the element triggering the request:
ds.Indicator("fetching")
ds.Attr("disabled", "$fetching")

// Show a spinner:
ds.Show("$fetching")
```

### Modifiers

| Constant | Effect |
|----------|--------|
| `ds.ModifierDebounce` | Debounce event |
| `ds.ModifierThrottle` | Throttle event |
| `ds.ModifierPrevent` | preventDefault |
| `ds.ModifierStop` | stopPropagation |
| `ds.ModifierOnce` | Fire once only |
| `ds.ModifierPassive` | Passive listener |
| `ds.ModifierCapture` | Capture phase |
| `ds.ModifierWindow` | Listen on window |
| `ds.ModifierOutside` | Click outside |
| `ds.ModifierSelf` | Only if target is self |
| `ds.Duration(d)` | Duration in ms (e.g. debounce time) |

### Morph control

```go
ds.Ignore()           // skip datastar processing for element and descendants
ds.IgnoreMorph()      // skip morphing for this element
ds.PreserveAttr("open", "value")  // preserve attributes during morph
ds.Ref("myEl")        // signal holding reference to this element ($myEl)
```

## datastar-go SSE server (`datastar`)

### URL helpers (used in `ds.On` expressions)

```go
datastar.GetSSE("/path")              // "@get('/path')"
datastar.PostSSE("/path")             // "@post('/path')"
datastar.GetSSE("/items/%d", id)      // supports fmt.Sprintf formatting
```

### SSE handler pattern

`datastar.NewSSE` flushes HTTP headers immediately, which has irreversible consequences:
- the response headers are sent — **cookies must be set before this call**
- the request body is consumed — **signals must be read before this call**
- any `http.Redirect` or `http.Error` after this point will not work

Always follow this order in SSE handlers:

```go
func (h *handler) myAction() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Read signals — must happen before NewSSE (body is consumed after)
        data := &MyForm{}
        if err := datastar.ReadSignals(r, data); err != nil {
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }

        // 2. Set cookies or headers — must happen before NewSSE (headers are flushed after)
        http.SetCookie(w, &http.Cookie{...})

        // 3. Create SSE stream — point of no return
        sse := datastar.NewSSE(w, r)

        // 4. Patch elements or signals
        sse.PatchElementGostar(myComponent(data))
        sse.MarshalAndPatchSignals(updatedSignals)
    }
}
```

### Patching elements

```go
// Default: morph element with matching ID in the DOM
sse.PatchElementGostar(myComponent(data))

// Target a specific CSS selector
sse.PatchElementGostar(myComponent(data), datastar.WithSelector("#some-id"))

// Patch modes
datastar.WithModeOuter()    // morph element (default)
datastar.WithModeInner()    // replace inner HTML
datastar.WithModeAppend()   // append inside target
datastar.WithModePrepend()  // prepend inside target
datastar.WithModeAfter()    // insert after target
datastar.WithModeBefore()   // insert before target
datastar.WithModeRemove()   // remove target from DOM
```

### Patching signals

```go
// Marshal struct to JSON and patch
sse.MarshalAndPatchSignals(MySignals{Count: 5})

// Only patch signals that don't already exist on the client
sse.MarshalAndPatchSignalsIfMissing(MySignals{Count: 0})
```

### Reading signals from requests

```go
type MyForm struct {
    Email    string `json:"email"`
    Password string `json:"password"`
    Count    int    `json:"count"`
}

data := &MyForm{}
if err := datastar.ReadSignals(r, data); err != nil {
    // handle error
}
```

Signals prefixed with `_` are excluded from SSE requests by default.

## Common patterns

### Alert feedback via SSE

Use the shared `AlertError`, `AlertSuccess`, and `AlertContainer` functions from `internal/ui/` (dot-imported). Each feature defines a one-line wrapper that supplies its stable DOM ID, then passes inner content from the shared helpers:

```go
// One-liner per feature — provides the stable DOM ID
func guildAlert(inner Node) Node { return AlertContainer("guild-alert", inner) }

// In page content — empty placeholder so the element exists before any SSE patch
guildAlert(nil)

// In SSE handlers — always patch the feature wrapper, never the inner components directly
sse.PatchElementGostar(guildAlert(AlertError(err)))                     // single error
sse.PatchElementGostar(guildAlert(AlertError(errs...)))                 // multiple errors
sse.PatchElementGostar(guildAlert(AlertSuccess("Settings saved.")))     // success
sse.PatchElementGostar(guildAlert(nil))                                 // clear
```

Patching success replaces any prior error and vice versa — one DOM target, mutual clearing.

**Validation strategy:**

Multi-error accumulation for independent form fields (show all problems at once):
```go
var errs []error
if len(name) < 5 || len(name) > 60 {
    errs = append(errs, errors.New("name must be between 5 and 60 characters"))
}
if len(desc) > 500 {
    errs = append(errs, errors.New("description cannot exceed 500 characters"))
}
if len(errs) > 0 {
    sse.PatchElementGostar(guildAlert(AlertError(errs...)))
    return
}
```

Immediate single-error return for state/authorization checks (order matters, no accumulation):
```go
if !viewerRole.CanManage() {
    sse.PatchElementGostar(guildAlert(AlertError(errors.New("not authorized"))))
    return
}
```

CSS classes `alert--success` and `alert--error` are defined in section 5 of `styles.css`.

### Redirect after action

```go
// Execute JS to navigate
sse.ExecuteScript(`window.location = '/dashboard'`)
```

### Toggle password visibility

```go
ds.Signals(map[string]any{
    "showPass": false,
}),
Input(ds.Bind("password"), ds.Attr("type", "$showPass ? 'text' : 'password'")),
Button(
    Type("button"),
    ds.Text("$showPass ? 'Hide' : 'Show'"),
    ds.On("click", "$showPass = !$showPass"),
),
```
