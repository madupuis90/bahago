package ui

import (
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

// PageHeader renders the page title plate (a single rotated red plate). All
// pages share this one loud title treatment (Design Reference §3). actions is
// optional; when present the title is wrapped in .page-header-text and the
// header becomes a flex row (the actions sit right, aligned to the plate's
// bottom — see 41-kingdom-overview.css).
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

// Breadcrumb renders a back-link chip. label should include the arrow.
func Breadcrumb(label, href string) Node {
	return Div(Class("breadcrumb"),
		A(Href(href), Class("crumb-back"), Text(label)),
	)
}
