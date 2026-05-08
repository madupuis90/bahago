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
