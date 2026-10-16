package core

import "testing"

func TestDefaultRules(t *testing.T) {
	want := map[Category]Rules{
		CategorySucculent: {10, 0.10}, CategoryFoliage: {30, 0.20},
		CategoryFlowering: {35, 0.20}, CategoryHerb: {40, 0.25},
	}
	for c, r := range want {
		if got := DefaultRules(c); got != r {
			t.Errorf("%s: %+v", c, got)
		}
	}
	if Category("cactus").Valid() {
		t.Error("unknown category valid")
	}
}
