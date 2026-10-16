package core

func (c Category) Valid() bool {
	switch c {
	case CategorySucculent, CategoryFoliage, CategoryFlowering, CategoryHerb:
		return true
	}
	return false
}

// DefaultRules returns the category's starting rules; zero Rules for an unknown category.
func DefaultRules(c Category) Rules {
	switch c {
	case CategorySucculent:
		return Rules{ThresholdPct: 10, AmountFraction: 0.10}
	case CategoryFoliage:
		return Rules{ThresholdPct: 30, AmountFraction: 0.20}
	case CategoryFlowering:
		return Rules{ThresholdPct: 35, AmountFraction: 0.20}
	case CategoryHerb:
		return Rules{ThresholdPct: 40, AmountFraction: 0.25}
	}
	return Rules{}
}
