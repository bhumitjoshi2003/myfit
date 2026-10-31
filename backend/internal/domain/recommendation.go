package domain

import (
	"errors"
	"fmt"
	"math"
	"time"
)

const RecommendationVersion = "mifflin-net-activity-v1"

type OnboardingProfile struct {
	DOB           string   `json:"dob"`
	Age           int      `json:"age"`
	AgeAsOf       string   `json:"age_as_of"`
	Sex           string   `json:"sex"`
	Height        float64  `json:"height"`
	HeightUnit    string   `json:"height_unit"`
	Weight        float64  `json:"weight"`
	WeightUnit    string   `json:"weight_unit"`
	Units         string   `json:"units"`
	Timezone      string   `json:"timezone"`
	Lifestyle     string   `json:"lifestyle"`
	Sessions      float64  `json:"sessions"`
	Minutes       float64  `json:"minutes"`
	Intensity     string   `json:"intensity"`
	Training      string   `json:"training"`
	Objective     string   `json:"objective"`
	RatePercent   float64  `json:"rate_percent"`
	TargetWeight  *float64 `json:"target_weight"`
	Diet          string   `json:"diet"`
	Restrictions  string   `json:"restrictions"`
	HealthContext string   `json:"health_context"`
}

func HeightCM(v float64, unit string) float64 {
	if unit == "in" {
		return v * 2.54
	}
	return v
}
func WeightKG(v float64, unit string) float64 {
	if unit == "lb" {
		return v * 0.45359237
	}
	return v
}
func member(v string, values ...string) bool {
	for _, s := range values {
		if v == s {
			return true
		}
	}
	return false
}
func bounded(v, lo, hi float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= lo && v <= hi
}

// Normalize validates transport units before canonical conversion. Age is derived in the
// user's time zone; an age-only input is anchored to its server-owned calculation date.
func (p OnboardingProfile) Normalize(now time.Time) (OnboardingProfile, error) {
	loc, e := time.LoadLocation(p.Timezone)
	if e != nil {
		return p, errors.New("choose a valid IANA time zone")
	}
	date := now.In(loc)
	p.AgeAsOf = date.Format("2006-01-02")
	if p.DOB != "" {
		dob, e := time.Parse("2006-01-02", p.DOB)
		if e != nil || p.DOB > p.AgeAsOf {
			return p, errors.New("invalid date of birth")
		}
		p.Age = date.Year() - dob.Year()
		if int(date.Month()) < int(dob.Month()) || (date.Month() == dob.Month() && date.Day() < dob.Day()) {
			p.Age--
		}
	}
	if p.Age < 1 || p.Age > 120 {
		return p, errors.New("provide age 1–120 or a valid date of birth")
	}
	if !member(p.HeightUnit, "cm", "in") || !member(p.WeightUnit, "kg", "lb") || !member(p.Units, "metric", "imperial") {
		return p, errors.New("invalid measurement units")
	}
	p.Height = HeightCM(p.Height, p.HeightUnit)
	p.Weight = WeightKG(p.Weight, p.WeightUnit)
	if p.TargetWeight != nil {
		v := WeightKG(*p.TargetWeight, p.WeightUnit)
		p.TargetWeight = &v
	}
	p.HeightUnit = "cm"
	p.WeightUnit = "kg"
	if !bounded(p.Height, 80, 250) || !bounded(p.Weight, 15, 350) {
		return p, errors.New("height must be 80–250 cm and weight 15–350 kg")
	}
	if p.TargetWeight != nil {
		if !bounded(*p.TargetWeight, 15, 350) {
			return p, errors.New("invalid target weight")
		}
		if (p.Objective == "gain" || p.Objective == "muscle") && *p.TargetWeight <= p.Weight {
			return p, errors.New("target weight must be above current weight for gain")
		}
		if p.Objective == "lose" && *p.TargetWeight >= p.Weight {
			return p, errors.New("target weight must be below current weight for loss")
		}
	}
	if !member(p.Sex, "male", "female", "unspecified") || !member(p.Lifestyle, "sedentary", "light", "moderate", "high") || !bounded(p.Sessions, 0, 7) || !bounded(p.Minutes, 0, 180) || !member(p.Intensity, "low", "moderate", "high") || !member(p.Training, "none", "resistance", "cardio", "mixed") {
		return p, errors.New("complete the formula, lifestyle and workout fields")
	}
	if p.Sessions > 0 && (p.Minutes < 10 || p.Training == "none") {
		return p, errors.New("workouts require a training type and duration of 10–180 minutes")
	}
	if p.Sessions == 0 {
		p.Minutes = 0
		p.Training = "none"
	}
	if !member(p.Objective, "gain", "muscle", "maintain", "lose") || !member(p.Diet, "vegetarian", "eggs", "nonvegetarian", "other") || !member(p.HealthContext, "healthy", "clinician") {
		return p, errors.New("complete objective, dietary preference and suitability fields")
	}
	if !bounded(p.RatePercent, 0, 1) {
		return p, errors.New("preferred rate must be 0–1% of body weight per week")
	}
	if p.Objective == "muscle" && p.RatePercent > 0.5 {
		return p, errors.New("choose at most 0.5% per week for controlled muscle gain")
	}
	if p.Objective == "maintain" {
		p.RatePercent = 0
	}
	if len(p.Restrictions) > 1000 {
		return p, errors.New("dietary notes are too long")
	}
	return p, nil
}

type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}
type Recommendation struct {
	Version          string    `json:"version"`
	Supported        bool      `json:"supported"`
	Age              int       `json:"age"`
	REE              float64   `json:"ree"`
	Maintenance      float64   `json:"maintenance"`
	LifestyleFactor  float64   `json:"lifestyle_factor"`
	ExerciseCalories float64   `json:"exercise_calories"`
	CalorieRange     Range     `json:"calorie_range"`
	ProteinRange     Range     `json:"protein_range"`
	FatRange         Range     `json:"fat_range"`
	Goals            Nutrition `json:"goals"`
	PaceKG           float64   `json:"pace_kg_per_week"`
	EffectiveRate    float64   `json:"effective_rate_percent"`
	Explanations     []string  `json:"explanations"`
	Limitations      []string  `json:"limitations"`
}

// Policy values are explicit and separate from user input. Lifestyle excludes workouts.
// These coarse activity/MET estimates are product assumptions, not measured expenditure.
type RecommendationConfig struct {
	Version                                                                                   string
	Lifestyle                                                                                 map[string]float64
	MET                                                                                       map[string]float64
	DefaultRate                                                                               map[string]float64
	MaxAdjustment                                                                             map[string]float64
	MinCalories, MaxCalories, FatShare, CarbMinShare, CarbMaxShare, FibrePer1000, EnergyPerKG float64
}

func DefaultRecommendationConfig() RecommendationConfig {
	return RecommendationConfig{
		Version: RecommendationVersion, Lifestyle: map[string]float64{"sedentary": 1.3, "light": 1.45, "moderate": 1.6, "high": 1.75}, MET: map[string]float64{"low": 3, "moderate": 5, "high": 7}, DefaultRate: map[string]float64{"gain": .25, "muscle": .25, "lose": .5}, MaxAdjustment: map[string]float64{"gain": .15, "muscle": .10, "lose": .20}, MinCalories: 1500, MaxCalories: 4500, FatShare: .30, CarbMinShare: .45, CarbMaxShare: .65, FibrePer1000: 14, EnergyPerKG: 7700}
}

type RecommendationService struct{ Config RecommendationConfig }

func (s RecommendationService) Calculate(input OnboardingProfile, asOf time.Time) (Recommendation, OnboardingProfile, error) {
	p, e := input.Normalize(asOf)
	if e != nil {
		return Recommendation{}, p, e
	}
	c := s.Config
	r := Recommendation{Version: c.Version, Age: p.Age, Explanations: []string{}, Limitations: []string{"Estimates are starting points, not precise measurements or medical advice. Review your trend and wellbeing after 2–4 weeks.", "Weight-change pace is highly uncertain; the static energy conversion does not model metabolic adaptation or predict muscle gain."}}
	bmi := p.Weight / math.Pow(p.Height/100, 2)
	if p.Age < 18 || p.Age > 65 || p.Sex == "unspecified" || p.HealthContext != "healthy" || bmi < 18.5 || bmi >= 40 {
		r.Limitations = append(r.Limitations, "Automatic targets are limited to apparently healthy adults aged 18–65, BMI 18.5–<40, with a selected male/female formula coefficient. Pregnancy, breastfeeding, eating-disorder history, relevant conditions/medications or other special needs require clinician-guided manual targets.")
		return r, p, nil
	}
	coefficient := 5.0
	if p.Sex == "female" {
		coefficient = -161
	}
	r.REE = 10*p.Weight + 6.25*p.Height - 5*float64(p.Age) + coefficient
	r.LifestyleFactor = c.Lifestyle[p.Lifestyle]
	// Subtract the already-covered non-exercise hourly baseline from gross workout METs.
	// This is more conservative than adding a second whole-day activity multiplier.
	r.ExerciseCalories = math.Max(0, c.MET[p.Intensity]-r.LifestyleFactor) * p.Weight * (p.Minutes / 60) * p.Sessions / 7
	r.Maintenance = r.REE*r.LifestyleFactor + r.ExerciseCalories
	rate := p.RatePercent
	if rate == 0 && p.Objective != "maintain" {
		rate = c.DefaultRate[p.Objective]
	}
	delta := math.Min(p.Weight*rate/100*c.EnergyPerKG/7, r.Maintenance*c.MaxAdjustment[p.Objective])
	if p.Objective == "lose" {
		delta = -delta
	}
	if p.Objective == "maintain" {
		delta = 0
	}
	target := math.Round((r.Maintenance+delta)/10) * 10
	if target < c.MinCalories || target > c.MaxCalories || target < r.REE {
		r.Limitations = append(r.Limitations, "This estimate falls outside the app's conservative automated calorie envelope. Use clinician-guided manual targets.")
		return r, p, nil
	}
	r.CalorieRange = Range{math.Max(math.Max(c.MinCalories, r.REE), math.Round(target*.95)), math.Min(c.MaxCalories, math.Round(target*1.05))}
	proteinMin, proteinMax, proteinFactor := .8, 1.2, 1.0
	if p.Sessions > 0 {
		proteinMin, proteinMax, proteinFactor = 1.4, 2, 1.6
		if p.Training == "resistance" || p.Training == "mixed" || p.Objective == "muscle" {
			proteinFactor = 1.8
		}
	}
	// Keep the weight-based protein range feasible within the dietary macro envelope.
	proteinMax = math.Min(proteinMax*p.Weight, target*(1-.20-c.CarbMinShare)/4)
	proteinMin = proteinMin * p.Weight
	if proteinMax < proteinMin {
		r.Limitations = append(r.Limitations, "Weight-based protein and balanced macro ranges do not overlap; please use clinician-guided targets.")
		return r, p, nil
	}
	protein := math.Max(proteinMin, math.Min(proteinMax, p.Weight*proteinFactor))
	fatShare := math.Max(.20, math.Min(.35, 1-protein*4/target-c.CarbMinShare))
	fatShare = math.Min(c.FatShare, fatShare)
	// A minimum 10% protein allocation and <=65% carbohydrate are respected too.
	protein = math.Max(protein, target*.10/4)
	if protein > proteinMax {
		r.Limitations = append(r.Limitations, "Protein requirements and the balanced macro envelope do not overlap; use manual targets.")
		return r, p, nil
	}
	fat := target * fatShare / 9
	carbs := (target - protein*4 - fat*9) / 4
	if carbs < target*c.CarbMinShare/4-1e-6 || carbs > target*c.CarbMaxShare/4+1e-6 || fat < 0 {
		r.Limitations = append(r.Limitations, "Balanced targets could not be allocated; use manual targets.")
		return r, p, nil
	}
	r.Supported = true
	r.ProteinRange = Range{math.Max(proteinMin, target*.1/4), proteinMax}
	r.FatRange = Range{target * .20 / 9, math.Min(target*.35/9, (target-protein*4-target*c.CarbMinShare)/9)}
	r.Goals = Nutrition{Calories: target, Protein: protein, Carbs: carbs, Fat: fat, Fibre: math.Round(target / 1000 * c.FibrePer1000)}
	r.PaceKG = (target - r.Maintenance) * 7 / c.EnergyPerKG
	r.EffectiveRate = math.Abs(r.PaceKG) / p.Weight * 100
	r.Explanations = append(r.Explanations, fmt.Sprintf("Mifflin–St Jeor resting estimate: 10 × %.2f kg + 6.25 × %.2f cm − 5 × %d years, with the selected %s coefficient.", p.Weight, p.Height, p.Age, p.Sex), fmt.Sprintf("Outside-workout lifestyle factor %.2f; %.0f kcal/day net workout allowance averaged over the week. Do not include these workouts in your lifestyle selection.", r.LifestyleFactor, r.ExerciseCalories), fmt.Sprintf("%s objective: preferred %.2f%% body weight/week; adjustment capped at %.0f%% of maintenance. Starting calories rounded to 10 kcal.", p.Objective, rate, c.MaxAdjustment[p.Objective]*100), "Protein uses body weight and training status; fat starts at 30% of calories and carbohydrate fills the remainder within 45–65%. Fibre uses 14 g per 1,000 kcal.", "Dietary preferences and allergies do not change energy expenditure. They are saved for future food suggestions.")
	return r, p, nil
}
