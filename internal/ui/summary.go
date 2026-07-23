package ui

import (
	"strconv"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

// SummaryStatTone selects the visual treatment of a SummaryStat's number.
type SummaryStatTone int

const (
	// StatDefault is the standard ink number.
	StatDefault SummaryStatTone = iota
	// StatNeg renders the number in red — use for resource drains or costs.
	StatNeg
	// StatMuted renders the number in a dimmed ink — use when the value is
	// currently zero and you want to de-emphasise it (e.g. no power afield).
	StatMuted
)

// SummaryStat is one entry in a SummaryStrip: a labelled number with an
// optional trailing sub-label ("units", "per tick", "/ 3"). Tone controls the
// number's colour treatment.
type SummaryStat struct {
	Label string
	Sub   string
	Num   int
	Tone  SummaryStatTone
}

// SummaryStrip renders the framed top-of-page stat bar shared by the Units
// and Campaign pages. Stats lay out vertically: label above, number (and
// optional sub) below. The strip is a single framed parchment surface; each
// stat is divided from its neighbour by a hairline border.
func SummaryStrip(stats ...SummaryStat) Node {
	return Div(Class("summary-strip"),
		Group(Map(stats, func(s SummaryStat) Node { return summaryStatNode(s) })),
	)
}

func summaryStatNode(s SummaryStat) Node {
	numClass := "summary-num"
	switch s.Tone {
	case StatNeg:
		numClass += " is-neg"
	case StatMuted:
		numClass += " is-muted"
	}
	val := Div(Class("summary-val"),
		Span(Class(numClass), Text(strconv.Itoa(s.Num))),
		If(s.Sub != "", Span(Class("summary-sub"), Text(s.Sub))),
	)
	return Div(Class("summary-stat"),
		Span(Class("summary-label"), Text(s.Label)),
		val,
	)
}