package ui

import (
	"fmt"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

// PageHeader renders the engraved page title: a formal symmetrical
// Cinzel caps title engraved in gold-brown on the parchment, followed by a
// gold filigree divider. All pages share this one title treatment (Design
// Reference §3). actions is optional; when present the title is wrapped in
// .page-header-text and the header becomes a flex row (the actions sit right,
// aligned to the title's bottom — see 41-kingdom-overview.css).
func PageHeader(title string, actions ...Node) Node {
	titleNode := H1(Class("page-header-title"), Text(title))
	if len(actions) == 0 {
		return Div(Class("page-header"), titleNode)
	}
	return Div(Class("page-header"),
		Div(Class("page-header-text"), titleNode),
		Div(Class("page-header-actions"), Group(actions)),
	)
}

// SectionHeader renders the shared .section-header bar. meta is optional.
func SectionHeader(title, meta string) Node {
	children := []Node{
		Div(Class("section-title"), Text(title)),
		Div(Class("section-rule")),
	}
	if meta != "" {
		children = append(children, Span(Class("section-meta"), Text(meta)))
	}
	return Div(Class("section-header"), Group(children))
}

// Breadcrumb renders a back-link chip; the arrow is part of the component.
func Breadcrumb(label, href string) Node {
	return Div(Class("breadcrumb"),
		A(Href(href), Class("crumb-back"), Text("← "+label)),
	)
}

// TickMeter renders a tick-progress meter: a name and eta line above a fill track
// with one notch per tick. The fill is the elapsed share, (total-remaining)/total.
func TickMeter(name Node, eta string, remaining, total int) Node {
	pct := 0.0
	if total > 0 {
		pct = float64(total-remaining) / float64(total) * 100
	}
	notches := make([]Node, 0, total)
	for range total {
		notches = append(notches, Span(Class("meter-notch")))
	}
	return Div(Class("meter"),
		Div(Class("meter-top"),
			Span(Class("meter-name"), name),
			Span(Class("meter-eta"), Text(eta)),
		),
		Div(Class("meter-track"),
			Div(Class("meter-fill"), Style(fmt.Sprintf("width:%.1f%%", pct))),
			Div(Class("meter-notches"), Group(notches)),
		),
	)
}
