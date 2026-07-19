package game

// ResourceValues holds an absolute quantity of each resource.
// Used for costs, stockpile deltas, and any context where values are actual amounts.
type ResourceValues struct {
	Wood      int
	Stone     int
	Food      int
	Mana      int
	Devotion  int
	Knowledge int
}

// ResourceOrder is the canonical display order of the six resources, matching
// the topbar pill order and the cost-pill rendering order. Callers iterating
// resource values for display should range over this slice.
var ResourceOrder = []string{"wood", "stone", "food", "mana", "devotion", "knowledge"}

// Amount returns the value for resKey ("wood"/"stone"/"food"/"mana"/
// "devotion"/"knowledge"), or 0 for an unknown key. It lets generic UI helpers
// range over ResourceOrder and read the matching field without re-implementing
// the field switch at every call site.
func (rv ResourceValues) Amount(resKey string) int {
	switch resKey {
	case "wood":
		return rv.Wood
	case "stone":
		return rv.Stone
	case "food":
		return rv.Food
	case "mana":
		return rv.Mana
	case "devotion":
		return rv.Devotion
	case "knowledge":
		return rv.Knowledge
	}
	return 0
}

// ProductionBonus holds production bonus percentages for each resource.
type ProductionBonus struct {
	Wood      int
	Stone     int
	Food      int
	Mana      int
	Devotion  int
	Knowledge int
}

func (b ProductionBonus) Add(other ProductionBonus) ProductionBonus {
	return ProductionBonus{
		Wood:      b.Wood + other.Wood,
		Stone:     b.Stone + other.Stone,
		Food:      b.Food + other.Food,
		Mana:      b.Mana + other.Mana,
		Devotion:  b.Devotion + other.Devotion,
		Knowledge: b.Knowledge + other.Knowledge,
	}
}

// HasAny reports whether any bonus field is non-zero.
func (b ProductionBonus) HasAny() bool {
	return b != (ProductionBonus{})
}
