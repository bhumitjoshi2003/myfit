package domain

import (
	"math"
	"testing"
	"time"
)

var asOf = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

func testProfile() OnboardingProfile {
	return OnboardingProfile{Age: 24, Sex: "male", Height: 167.64, HeightUnit: "cm", Weight: 54.5, WeightUnit: "kg", Units: "metric", Timezone: "Asia/Kolkata", Lifestyle: "sedentary", Sessions: 3.5, Minutes: 60, Intensity: "moderate", Training: "resistance", Objective: "muscle", Diet: "vegetarian", HealthContext: "healthy"}
}
func calculate(t *testing.T, p OnboardingProfile) Recommendation {
	t.Helper()
	r, _, e := (RecommendationService{DefaultRecommendationConfig()}).Calculate(p, asOf)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func near(t *testing.T, a, b float64) {
	t.Helper()
	if math.Abs(a-b) > 1e-6 {
		t.Fatalf("got %v want %v", a, b)
	}
}
func TestPersonalProfile(t *testing.T) {
	r := calculate(t, testProfile())
	if !r.Supported {
		t.Fatal(r.Limitations)
	}
	near(t, r.REE, 1477.75)
	near(t, r.Maintenance, 1477.75*1.3+(5-1.3)*54.5*3.5/7)
	near(t, r.Goals.Protein, 98.1)
	if r.Goals.Calories < 2000 || r.Goals.Calories > 2500 {
		t.Fatal(r)
	}
	r2 := calculate(t, testProfile())
	near(t, r2.Goals.Calories, r.Goals.Calories)
}
func TestFormulaAndMeasurements(t *testing.T) {
	p := testProfile()
	m := calculate(t, p)
	p.Sex = "female"
	f := calculate(t, p)
	near(t, m.REE-f.REE, 166)
	p.Sex = "male"
	p.Age = 44
	p.Height = 175
	p.Weight = 70
	r := calculate(t, p)
	near(t, r.REE, 10*70+6.25*175-5*44+5)
}
func TestUnitsAndDOB(t *testing.T) {
	p := testProfile()
	p.Height = 66
	p.HeightUnit = "in"
	p.Weight = 54.5 / .45359237
	p.WeightUnit = "lb"
	p.Units = "imperial"
	r := calculate(t, p)
	near(t, r.REE, calculate(t, testProfile()).REE)
	p.DOB = "2002-11-01"
	p.Age = 99
	service := RecommendationService{DefaultRecommendationConfig()}
	r, normalized, e := service.Calculate(p, asOf)
	if e != nil {
		t.Fatal(e)
	}
	if r.Age != 23 || normalized.Age != 23 || normalized.HeightUnit != "cm" || normalized.WeightUnit != "kg" {
		t.Fatal(normalized)
	}
	p.Timezone = "Pacific/Kiritimati"
	r, _, _ = service.Calculate(p, time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC))
	if r.Age != 24 {
		t.Fatal("birthday should follow user timezone")
	}
}
func TestActivityAndObjectives(t *testing.T) {
	p := testProfile()
	p.Sessions = 0
	p.Minutes = 0
	p.Training = "none"
	sed := calculate(t, p)
	near(t, sed.ExerciseCalories, 0)
	p.Lifestyle = "high"
	active := calculate(t, p)
	if active.Maintenance <= sed.Maintenance {
		t.Fatal("lifestyle ignored")
	}
	p = testProfile()
	few := calculate(t, p)
	p.Sessions = 7
	freq := calculate(t, p)
	near(t, freq.ExerciseCalories, 2*few.ExerciseCalories)
	p.Minutes = 120
	long := calculate(t, p)
	near(t, long.ExerciseCalories, 2*freq.ExerciseCalories)
	p = testProfile()
	p.Objective = "maintain"
	maint := calculate(t, p)
	p.Objective = "gain"
	gain := calculate(t, p)
	p.Objective = "lose"
	lose := calculate(t, p)
	if gain.Goals.Calories <= maint.Goals.Calories || lose.Goals.Calories >= maint.Goals.Calories {
		t.Fatal("objective ignored")
	}
	p = testProfile()
	p.Diet = "nonvegetarian"
	near(t, calculate(t, p).Maintenance, few.Maintenance)
}
func TestInvalidProfiles(t *testing.T) {
	cases := []func(*OnboardingProfile){func(p *OnboardingProfile) { p.Height = 0 }, func(p *OnboardingProfile) { p.Weight = -1 }, func(p *OnboardingProfile) { p.Weight = math.Inf(1) }, func(p *OnboardingProfile) { p.Age = 0 }, func(p *OnboardingProfile) { p.DOB = "2030-01-01" }, func(p *OnboardingProfile) { p.Lifestyle = "" }, func(p *OnboardingProfile) { p.HealthContext = "" }, func(p *OnboardingProfile) { p.Training = "none" }, func(p *OnboardingProfile) { p.Minutes = 0 }, func(p *OnboardingProfile) { p.Sessions = 8 }, func(p *OnboardingProfile) { p.RatePercent = .6 }, func(p *OnboardingProfile) { p.Timezone = "Invalid/Zone" }, func(p *OnboardingProfile) { v := 50.; p.TargetWeight = &v }, func(p *OnboardingProfile) { p.WeightUnit = "stone" }}
	for i, change := range cases {
		p := testProfile()
		change(&p)
		if _, _, e := (RecommendationService{DefaultRecommendationConfig()}).Calculate(p, asOf); e == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}
func TestOutsideSupportedPopulation(t *testing.T) {
	for _, change := range []func(*OnboardingProfile){func(p *OnboardingProfile) { p.Age = 17 }, func(p *OnboardingProfile) { p.Age = 70 }, func(p *OnboardingProfile) { p.HealthContext = "clinician" }, func(p *OnboardingProfile) { p.Sex = "unspecified" }, func(p *OnboardingProfile) { p.Weight = 40 }, func(p *OnboardingProfile) { p.Weight = 150 }} {
		p := testProfile()
		change(&p)
		r := calculate(t, p)
		if r.Supported || r.Goals.Calories != 0 {
			t.Fatal("unsafe population received goals")
		}
	}
}
func TestSafeMacroRanges(t *testing.T) {
	count := 0
	for _, sex := range []string{"male", "female"} {
		for _, age := range []int{18, 24, 45, 65} {
			for _, weight := range []float64{50, 65, 85, 110} {
				for _, life := range []string{"sedentary", "light", "moderate", "high"} {
					for _, goal := range []string{"gain", "muscle", "maintain", "lose"} {
						p := testProfile()
						p.Sex = sex
						p.Age = age
						p.Weight = weight
						p.Lifestyle = life
						p.Objective = goal
						r := calculate(t, p)
						if !r.Supported {
							continue
						}
						count++
						n := r.Goals
						near(t, n.Protein*4+n.Carbs*4+n.Fat*9, n.Calories)
						if !n.Valid(4500, 2000) || n.Calories < 1500 || n.Carbs*4/n.Calories < .45-1e-9 || n.Carbs*4/n.Calories > .65+1e-9 || n.Fat*9/n.Calories < .20-1e-9 || n.Fat*9/n.Calories > .35+1e-9 || n.Protein < r.ProteinRange.Min || n.Protein > r.ProteinRange.Max || n.Fat < r.FatRange.Min || n.Fat > r.FatRange.Max+1e-8 {
							t.Fatal("unsafe macro allocation", r)
						}
					}
				}
			}
		}
	}
	if count < 300 {
		t.Fatalf("too few supported cases: %d", count)
	}
}
func TestConfigIsIndependent(t *testing.T) {
	p := testProfile()
	c := DefaultRecommendationConfig()
	c.Version = "test-policy"
	c.DefaultRate["muscle"] = .1
	r, _, e := (RecommendationService{c}).Calculate(p, asOf)
	if e != nil || r.Version != "test-policy" {
		t.Fatal(e)
	}
	if r.Goals.Calories >= calculate(t, p).Goals.Calories {
		t.Fatal("configuration not applied")
	}
}
