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
    . "bahago/internal/layout"            // HomeLayout(), KingdomLayout(), shared components — dot-imported
    "bahago/internal/signals"             // signals.Signal[T], signals.NewSignalDef[T](), signals.SignalMap() — regular import
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
- Components extracted to `internal/layout/` only when used by 2+ handler packages
- **`internal/layout` is dot-imported** — `HomeLayout`, `KingdomLayout`, nav components, and shared helpers are used without a package prefix
- **`internal/signals` is a regular import** — always reference as `signals.Signal[T]`, `signals.NewSignalDef[T]()`, `signals.SignalMap(sigs)`

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

If a component is used only within one feature package, keep it in that package. Move it to `internal/layout/` only when a second package needs it.

## Styling

All styles are hand-written in `web/static/styles.css`. There are no CSS frameworks (no Tailwind, Bootstrap, etc.). Classes applied via `Class("foo")` must exist in that file.

### CSS file structure

`styles.css` is organized into numbered sections, ordered from most general to most specific. Always add new rules in the correct section:

```
1. Reset          — *, box-sizing
2. Tokens         — :root custom properties
3. Base           — html, body, a, input (element selectors only)
4. Layout         — top-nav, content-area, side-nav, nav-group
5. Shared         — .panel, .btn, .btn-text, .form-fields, .password-field, .alert-*
6. Auth           — .auth-card and auth-specific styles
7. Kingdom        — .kingdom-* styles
8. Allocation     — .allocation-* styles
9. Utilities      — .text-positive, .text-negative (single-purpose overrides)
```

New feature modules get their own numbered section between Allocation and Utilities. Shared components (used by 2+ features) belong in section 5.

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

## No string literals for element IDs

Any element ID that is targeted by SSE (`PatchElementGostar`, `WithSelectorID`) or referenced from Go code must be declared as a `const` near the component that owns it. Never write ID strings inline in two places.

```go
// Declare next to the component that owns the element
const errorComponentID = "errors"

func errorComponent(errs []error) Node {
    return Div(ID(errorComponentID), ...)
}

// SSE handler targets the same const — compiler catches typos
sse.PatchElementGostar(errorComponent(errs))                          // morphs by ID automatically
sse.PatchElementGostar(myNode, datastar.WithSelectorID(someOtherID))  // explicit selector
```

## No string literals for signal names

Define a struct where every field is `Signal[T]` with a `json` tag. Use `NewSignalDef` to create a package-level definition — this populates `Key` and `Ref` on every field at startup. In handlers, call `.New()` to get a value copy you can set `.Value` on, then pass it to content functions. For reading signals from the client, use a plain zero-value struct.

```go
// Define once per page/feature
type PageSignals struct {
    WoodPct Signal[int]    `json:"wood_pct"`
    Name    Signal[string] `json:"name"`
}
var sigDef = NewSignalDef[PageSignals]()

// Handler — populate values and render
sigs := sigDef.New()
sigs.WoodPct.Value = kingdom.WoodPct
pageContent(sigs)

// Handler — read values from client
input := &PageSignals{}
datastar.ReadSignals(r, input)
// use input.WoodPct.Value

// Content function — initialise datastar signals and bind inputs
ds.Signals(SignalMap(sigs))           // initialise all signals from the struct
ds.Bind(sigs.WoodPct.Key)             // two-way bind
ds.Text(sigs.WoodPct.Ref)             // reactive JS expression
ds.On("click", sigs.WoodPct.Ref+"++") // JS expression in event handler
```

The `json` tags define the signal name on the wire. `Key` and `Ref` are derived from them at startup — no string literals anywhere in the template.

## gomponents-datastar (`ds`) — reactive attributes

Signals use `$signalName` syntax in expressions. Signal names cannot start with or contain `__`.

### Signals

```go
// Initialize signals on an element (values defined later in DOM override earlier)
ds.Signals(map[string]any{"count": 0, "open": false})

// Two-way bind input/select/textarea to a signal
ds.Bind("signalName")
```

**Signal naming pattern** — use `Signal[T]` fields and `NewSignalDef`:
```go
type PageSignals struct {
    WoodPct Signal[int] `json:"wood_pct"`
    Name    Signal[string] `json:"name"`
}
var sigDef = NewSignalDef[PageSignals]()
// In handler: sigs := sigDef.New(); sigs.WoodPct.Value = x
// In template: ds.Bind(sigs.WoodPct.Key), ds.Text(sigs.WoodPct.Ref)
// Init signals: ds.Signals(SignalMap(sigs))
```

### Reactivity

```go
ds.Show("$visible")                        // show/hide element
ds.Text("$count")                          // reactive text content
ds.Attr("disabled", "$loading", "title", "$tooltip")  // key-value pairs
ds.Class("active", "$isActive", "hidden", "$hidden")  // key-value pairs
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

Use a three-function pattern so that a single SSE patch always replaces any previous alert — error clears success, success clears error:

```go
// Declare one ID per feature
const myAlertID = "my-alert"

// alertComponent is the single patch target — always patch this, never the inner components
func alertComponent(inner Node) Node {
    return Div(ID(myAlertID), inner)
}

// Inner content — pass nil to clear
func errorComponent(errs []error) Node {
    if len(errs) == 0 {
        return nil
    }
    return Div(Class("alert-error"),
        Map(errs, func(e error) Node { return P(Text(e.Error())) }),
    )
}

func successComponent(msg string) Node {
    return Div(Class("alert-success"), P(Text(msg)))
}

// In page content — placeholder so the element exists in DOM before any patch
alertComponent(nil)

// In SSE handlers — always patch alertComponent
sse.PatchElementGostar(alertComponent(errorComponent(errs)))
sse.PatchElementGostar(alertComponent(successComponent("Done!")))
sse.PatchElementGostar(alertComponent(nil)) // clear
```

CSS classes `alert-success` and `alert-error` are defined in section 5 of `styles.css`.

### Redirect after action

```go
// Execute JS to navigate
sse.ExecuteScript(`window.location = '/dashboard'`)
```

### Toggle password visibility

```go
ds.Signals(map[string]any{"showPass": false}),
Input(ds.Bind("password"), ds.Attr("type", "$showPass ? 'text' : 'password'")),
Button(
    Type("button"),
    ds.Text("$showPass ? 'Hide' : 'Show'"),
    ds.On("click", "$showPass = !$showPass"),
),
```
