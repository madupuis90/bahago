package ui

import (
	"fmt"
	"strconv"
	"strings"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/components"
	ds "maragu.dev/gomponents-datastar"
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
// red. Per-resource affordability was the previous buildings behaviour but
// was dropped to keep the component's look uniform across pages (Fork A).

// CostEntry pairs a resource key (wood/stone/food/mana/devotion/knowledge)
// with a static integer amount.
type CostEntry struct {
	Resource string
	Amount  int
}

// DynamicCostEntry pairs a resource key with a datastar expression that
// evaluates to the amount (e.g. "$cost_wood_militia").
type DynamicCostEntry struct {
	Resource string
	Expr     string
}

// CostOpt configures a cost pill.
type CostOpt func(*costConfig)

type costConfig struct {
	gemSize        int
	staticAvail    map[string]int
	staticAvailSet bool
	signalAvail    bool
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

func defaultConfig() costConfig { return costConfig{gemSize: 18} }

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
			items = append(items, costItem{Resource: res, need: n, amount: Text(strconv.Itoa(n))})
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
	return renderCostPill(items, classNode, cfg.gemSize)
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
		items = append(items, costItem{Resource: e.Resource, expr: e.Expr, amount: ds.Text(e.Expr)})
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
	return renderCostPill(items, classNode, cfg.gemSize)
}

// costItem is the internal, source-agnostic representation of one entry.
// For static pills, need + amount (Text) are set. For dynamic pills, expr +
// amount (ds.Text) are set. amount is always the node that renders the value;
// need/expr drive the affordability expression.
type costItem struct {
	Resource string
	amount   Node
	need     int    // static path: the fixed cost
	expr     string // dynamic path: expression yielding the cost
}

func renderCostPill(items []costItem, classNode Node, gemSize int) Node {
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
	return Span(children...)
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