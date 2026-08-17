package ui

import (
	"fmt"
	"strconv"
	"strings"

	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"

	"bahago/internal/game"
)

// Cost pill — shared resource cost chip.
//
// Two constructors cover the two amount sources:
//
//   - StaticCostPill  — integer amounts read from a game.ResourceValues.
//   - DynamicCostPill — amounts bound to datastar expressions on the page.
//
// Both render one pill containing one inline entry per non-zero resource
// (gem + amount), ordered by game.ResourceOrder.
//
// Availability (the .is-short state, red pill) is opt-in via one of two
// options:
//
//   - WithStaticAvailability(have) — server-side comparison against a stockpile
//     map; used when costs are static ints.
//   - WithSignalAvailability()   — dynamic comparison against the
//     layout-scoped resource signals ($wood/$stone/$food/$mana/$devotion/
//     $knowledge) emitted by KingdomLayout and refresh-patched by the layout
//     refresh SSE handler. Only usable by pills rendered under KingdomLayout,
//     which all kingdom pages are. A pill rendered outside that scope silently
//     never goes short (undefined signals are falsy).
//
// Affordability is whole-pill: if any resource is short, the entire pill is
// red; there is no per-resource affordability state.

// CostEntry pairs a resource key (wood/stone/food/mana/devotion/knowledge)
// with a static integer amount.
type CostEntry struct {
	Resource string
	Amount   int
}

// DynamicCostEntry pairs a resource key with a datastar expression that
// evaluates to the amount (e.g. "$cost_wood_militia").
type DynamicCostEntry struct {
	Resource string
	Expr     string
}

// CostKind labels what the pill's values represent, so the pill can show a
// per-tick direction marker. It keys off the glossary's two per-tick flows —
// Production Rate and Upkeep — with Flat being the default (one-time cost).
//
//   - CostFlat       — one-time cost; no marker.
//   - CostUpkeep     — a per-tick drain; renders the sandglass + arrow-down
//     marker after the amount.
//   - CostProduction — a per-tick gain; renders the sandglass + arrow-up
//     marker after the amount.
//
// The marker is whole-pill: a cost is either flat or a rate, never mixed.
type CostKind int

const (
	CostFlat CostKind = iota
	CostUpkeep
	CostProduction
)

// CostOpt configures a cost pill.
type CostOpt func(*costConfig)

type costConfig struct {
	gemSize        int
	staticAvail    map[string]int
	staticAvailSet bool
	signalAvail    bool
	kind           CostKind
	percent        bool
}

// WithGemSize overrides the gem diameter (px). Default 18.
func WithGemSize(px int) CostOpt {
	return func(c *costConfig) { c.gemSize = px }
}

// WithStaticAvailability enables whole-pill affordability for a static pill:
// the pill renders .is-short when any entry's amount exceeds have[resource].
// have is keyed by resource key.
func WithStaticAvailability(have map[string]int) CostOpt {
	return func(c *costConfig) { c.staticAvail = have; c.staticAvailSet = true }
}

// WithSignalAvailability enables dynamic affordability for a dynamic pill.
// See the file comment for the layout-signal convention it relies on.
func WithSignalAvailability() CostOpt {
	return func(c *costConfig) { c.signalAvail = true }
}

// WithCostKind labels the pill as a per-tick rate (Upkeep or Production) so
// it renders the sandglass + arrow direction marker after the amount. The
// default is CostFlat (no marker). See CostKind for the semantics.
func WithCostKind(kind CostKind) CostOpt {
	return func(c *costConfig) { c.kind = kind }
}

// WithPercent formats each amount as "+N%" instead of "N". Use it for pills
// that show a production *modifier* (e.g. a prayer's +20% resource bonus)
// rather than a per-tick amount; pair it with WithCostKind(CostProduction)
// so the green up-arrow carries the direction. The pill's affordability
// options still compare the raw integer (need/expr) against the stockpile,
// so percent pills should not also use WithStaticAvailability/
// WithSignalAvailability — a percentage is not a spendable cost.
func WithPercent() CostOpt {
	return func(c *costConfig) { c.percent = true }
}

func defaultConfig() costConfig { return costConfig{gemSize: 18, kind: CostFlat} }

// StaticCostPill renders one cost pill from a ResourceValues, omitting
// zero-amount resources.
func StaticCostPill(rv game.ResourceValues, opts ...CostOpt) Node {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	var items []costItem
	for _, res := range game.ResourceOrder {
		if n := rv.Amount(res); n > 0 {
			items = append(items, costItem{Resource: res, need: n, amount: Text(formatAmount(n, cfg.percent))})
		}
	}
	if len(items) == 0 {
		return nil
	}
	var classNode Node
	if cfg.staticAvailSet {
		classNode = staticShortClass(items, cfg.staticAvail)
	} else {
		classNode = Class("cost-pill")
	}
	return renderCostPill(items, classNode, cfg.gemSize, cfg.kind)
}

// DynamicCostPill renders one cost pill whose amounts are datastar expressions.
// Entries with an empty Expr are omitted.
func DynamicCostPill(entries []DynamicCostEntry, opts ...CostOpt) Node {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	var items []costItem
	for _, e := range entries {
		if e.Expr == "" {
			continue
		}
		items = append(items, costItem{Resource: e.Resource, expr: e.Expr, amount: ds.Text(formatAmountExpr(e.Expr, cfg.percent))})
	}
	if len(items) == 0 {
		return nil
	}
	var classNode Node
	if cfg.signalAvail {
		classNode = dynamicShortClass(items)
	} else {
		classNode = Class("cost-pill")
	}
	return renderCostPill(items, classNode, cfg.gemSize, cfg.kind)
}

// costItem is the internal, source-agnostic representation of one entry.
// amount is always the node that renders the value; need (static) or expr
// (dynamic) drives the affordability check.
type costItem struct {
	Resource string
	amount   Node
	need     int    // static path: the fixed cost
	expr     string // dynamic path: expression yielding the cost
}

func renderCostPill(items []costItem, classNode Node, gemSize int, kind CostKind) Node {
	children := []Node{classNode}
	for _, it := range items {
		// The amount rides on its own inner span so datastar's data-text (dynamic
		// path) only rewrites that inner span's text, leaving the gem sibling intact.
		children = append(children,
			Span(Class("cost-pill-res"),
				ResourceGem(GemIDForResource(it.Resource), gemSize),
				Span(it.amount),
			),
		)
	}
	if m := rateMarker(kind, gemSize); m != nil {
		children = append(children, m)
	}
	return Span(children...)
}

// rateMarker renders the per-tick marker (sandglass + oriented arrow) for an
// Upkeep/Production pill, or nil for Flat. It is whole-pill: the marker lives
// at the pill level (after all resource entries), not per-entry, since a cost
// is either flat or a rate. The arrow carries direction; the sandglass carries
// "per tick" (the codebase's established tick symbol, see docs/design/icons.md).
func rateMarker(kind CostKind, gemSize int) Node {
	var arrowID string
	switch kind {
	case CostUpkeep:
		arrowID = "arrow-down"
	case CostProduction:
		arrowID = "arrow-up"
	default:
		return nil
	}
	// Marker glyphs scale with the pill's gem size so they read in proportion
	// across the gemSize variants used by call sites (17–22px).
	markerSize := gemSize * 13 / 18 // ~13px at the 18px default; tracks gemSize
	if markerSize < 11 {
		markerSize = 11
	}
	// The sandglass stays neutral ink (the "per tick" symbol); only the arrow
	// takes the kind colour — red for upkeep (drain), green for production
	// (gain), per the meaning-colour palette in docs/design/icons.md. The
	// arrow rides in its own span so the colour class targets just that glyph.
	arrowCls := "cost-pill-arrow"
	if kind == CostUpkeep {
		arrowCls += " cost-pill-arrow--down"
	} else {
		arrowCls += " cost-pill-arrow--up"
	}
	return Span(Class("cost-pill-rate"),
		Icon("sandglass", markerSize, false),
		Span(Class(arrowCls), Icon(arrowID, markerSize, false)),
	)
}

// formatAmount renders a static integer amount: "N", or "+N%" when percent
// is set (for production-modifier pills). The leading "+" signals a gain (a
// production modifier is always additive), and pairs with the CostProduction
// up-arrow marker.
func formatAmount(n int, percent bool) string {
	if percent {
		return fmt.Sprintf("+%d%%", n)
	}
	return strconv.Itoa(n)
}

// formatAmountExpr wraps a dynamic amount expression: the bare expression
// ("$cost_wood_militia") when not percent, or "+(<expr>)%" when percent
// (for dynamic production-modifier pills). String concatenation in the
// datastar expression keeps the +/% literals out of the bound expr itself.
func formatAmountExpr(expr string, percent bool) string {
	if percent {
		return fmt.Sprintf("'+' + (%s) + '%%'", expr)
	}
	return expr
}

// staticShortClass returns a class attribute with cost-pill always set and
// is-short when any entry's need exceeds the matching stockpile value.
func staticShortClass(items []costItem, have map[string]int) Node {
	short := false
	for _, it := range items {
		if have[it.Resource] < it.need {
			short = true
			break
		}
	}
	return Classes{"cost-pill": true, "is-short": short}
}

// dynamicShortClass returns the pill's class attributes for the dynamic case:
// a static Class("cost-pill") so the base style is always present, plus a
// data-class attribute that toggles is-short based on whether any entry's
// cost expression exceeds its $<resource> signal (the OR of one comparison
// per entry). Attributes merge into the same Span cleanly.
func dynamicShortClass(items []costItem) Node {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		if it.expr == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("(%s > $%s)", it.expr, it.Resource))
	}
	// "is-short" is quoted because it is not a valid JS identifier (the hyphen
	// makes {is-short: …} a syntax error); datastar's toObject does not quote keys
	// for us, so callers must quote hyphenated class names themselves. This matches
	// the convention elsewhere in the codebase (e.g. ds.Class("'is-pending'", …)).
	return Group([]Node{Class("cost-pill"), ds.Class("'is-short'", strings.Join(parts, " || "))})
}
