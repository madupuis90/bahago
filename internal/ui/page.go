package ui

import (
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

// PageHeader renders the Comic Lab title plate (kicker + rotated red plate +
// sub). actions is optional; when present the text block is wrapped in
// .page-header-text and the header becomes a flex row (the actions sit right,
// aligned to the plate's bottom — see 41-kingdom-overview.css).
func PageHeader(kicker, title, sub string, actions Node) Node {
	textChildren := []Node{
		P(Class("page-header-kicker"), Text(kicker)),
		H1(Class("page-header-title"), Text(title)),
	}
	if sub != "" {
		textChildren = append(textChildren, P(Class("page-header-sub"), Text(sub)))
	}
	if actions == nil {
		return Div(Class("page-header"), Div(Class("page-header-text"), Group(textChildren)))
	}
	return Div(Class("page-header"),
		Div(Class("page-header-text"), Group(textChildren)),
		Div(Class("page-header-actions"), actions),
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
