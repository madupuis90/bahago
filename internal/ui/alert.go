package ui

import (
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

// AlertError renders error alert content from one or more errors.
// Returns nil when errs is empty, so callers can pass it directly to a feature alert wrapper.
func AlertError(errs ...error) Node {
	if len(errs) == 0 {
		return nil
	}
	return Div(Class("alert--error"),
		Map(errs, func(e error) Node { return P(Text(e.Error())) }),
	)
}

// AlertSuccess renders a success alert. Returns nil for an empty message.
func AlertSuccess(msg string) Node {
	if msg == "" {
		return nil
	}
	return Div(Class("alert--success"), P(Text(msg)))
}

// AlertContainer wraps inner content at a stable DOM ID for SSE patching.
// Pass nil to clear any displayed alert.
func AlertContainer(id string, inner Node) Node {
	return Div(ID(id), inner)
}
