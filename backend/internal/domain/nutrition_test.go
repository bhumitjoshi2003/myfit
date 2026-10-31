package domain

import (
	"math"
	"testing"
)

func TestBanana(t *testing.T) {
	n := Nutrition{89, 1.09, 22.84, .33, 2.6}
	two := n.Scale(118, 2)
	one := n.Scale(118, 1)
	want := Nutrition{210.04, 2.5724, 53.9024, .7788, 6.136}
	for i, v := range []float64{two.Calories - want.Calories, two.Protein - want.Protein, two.Carbs - want.Carbs, two.Fat - want.Fat, two.Fibre - want.Fibre, two.Calories - 2*one.Calories} {
		if math.Abs(v) > 1e-9 {
			t.Fatalf("nutrient %d difference %v", i, v)
		}
	}
	var total Nutrition
	total.Add(one)
	total.Add(one)
	if total != two {
		t.Fatal("aggregation differs")
	}
}
func TestValidation(t *testing.T) {
	for _, v := range []float64{-1, math.NaN(), math.Inf(1), 1001} {
		if (Nutrition{Calories: v}).Valid(1000, 100) {
			t.Fatalf("accepted %v", v)
		}
	}
}
