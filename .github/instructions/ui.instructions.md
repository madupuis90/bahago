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
    . "bahago/internal/ui"                // Layout(), shared components — dot-imported
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
| `If(bool, Node)` | Render node only if true |
| `Iff(bool, func() Node)` | Lazy conditional — avoids evaluating node when false |
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

## Project Layout convention

- All pages use `Layout(LayoutArgs{Title, User}, body...)` from `internal/ui/` (dot-imported)
- Page functions return `Node` (pure functions, no side effects)
- Handlers call `page().Render(w)` for full-page responses
- Components extracted to `internal/ui/` only when used by 2+ pages
- **Internal UI packages (`internal/ui/`) are dot-imported** — their exported functions (`Layout`, shared components) are used without a package prefix, just like the gomponents html functions

```go
// A page function — returns Node, reads like a template
func myPage(data SomeData) Node {
    return Layout(
        LayoutArgs{Title: "My Page", User: user},
        H1(Text("Hello")),
        myCard(data),  // reusable component in same or ui package
    )
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

**When generating new UI**, omit styles unless explicitly asked — focus on structure and correctness. When styles are needed later, they go in `web/static/styles.css`. Do not suggest inline styles or CSS-in-Go approaches.

**Use CSS variables** for any value that appears more than once or is likely to be reused — especially spacing sizes, colors, dimensions, and border definitions. Define them in `:root` in `styles.css`. For example, prefer `var(--spacing-md)` over a hardcoded `1rem`, and `var(--border)` over a repeated `1px solid var(--border-color)`.

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

Signal names are shared between the HTML template (via `ds.Bind`) and the handler (via `datastar.ReadSignals`). Define a struct for deserialization and immediately declare a variable of that type holding the field names as values. Reference the variable in both the template and the handler — the compiler catches typos.

```go
// Define once per form/feature
type LoginForm struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}
var loginSignals = LoginForm{Email: "email", Password: "password"}

// Template — references variable fields, not string literals
Input(ds.Bind(loginSignals.Email))
Input(Type("password"), ds.Bind(loginSignals.Password))

// Handler — deserializes using the same struct
data := &LoginForm{}
datastar.ReadSignals(r, data)  // json tags match the signal names
```

The `json` tags on the struct define the actual signal name on the wire. The variable provides compile-time–checked references to those names in templates.

## gomponents-datastar (`ds`) — reactive attributes

Signals use `$signalName` syntax in expressions. Signal names cannot start with or contain `__`.

### Signals

```go
// Initialize signals on an element (values defined later in DOM override earlier)
ds.Signals(map[string]any{"count": 0, "open": false})

// Two-way bind input/select/textarea to a signal
ds.Bind("signalName")
```

**Signal naming pattern** — define a struct for compile-time checked names:
```go
type LoginForm struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}
var loginSignals = LoginForm{Email: "email", Password: "password"}
// Usage: ds.Bind(loginSignals.Email)
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

### Error feedback via SSE

```go
// Component must have a stable ID for morphing to work
func errorComponent(errs []error) g.Node {
    return Div(
        ID("error-msg"),
        g.Map(errs, func(e error) g.Node {
            return P(Text(e.Error()))
        }),
    )
}

// In SSE handler:
sse.PatchElementGostar(errorComponent(errs))
```

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
