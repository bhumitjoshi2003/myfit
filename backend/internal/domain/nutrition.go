package domain

import "math"

type Nutrition struct {
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein"`
	Carbs    float64 `json:"carbs"`
	Fat      float64 `json:"fat"`
	Fibre    float64 `json:"fibre"`
}

func (n Nutrition) Valid(maxCalories, maxMacro float64) bool {
	for i, v := range []float64{n.Calories, n.Protein, n.Carbs, n.Fat, n.Fibre} {
		max := maxMacro
		if i == 0 {
			max = maxCalories
		}
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > max {
			return false
		}
	}
	return true
}

// Scale retains precision; presentation rounding happens only in the client.
func (n Nutrition) Scale(grams, quantity float64) Nutrition {
	f := grams * quantity / 100
	return Nutrition{n.Calories * f, n.Protein * f, n.Carbs * f, n.Fat * f, n.Fibre * f}
}
func (n *Nutrition) Add(b Nutrition) {
	n.Calories += b.Calories
	n.Protein += b.Protein
	n.Carbs += b.Carbs
	n.Fat += b.Fat
	n.Fibre += b.Fibre
}

type Serving struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Grams float64 `json:"grams"`
}
type Food struct {
	ID      int64  `json:"id"`
	OwnerID *int64 `json:"owner_id"`
	Name    string `json:"name"`
	Brand   string `json:"brand"`
	Nutrition
	Source      string    `json:"source"`
	Approximate bool      `json:"approximate"`
	Version     int       `json:"version"`
	Servings    []Serving `json:"servings"`
	Favourite   bool      `json:"favourite"`
}
type User struct {
	Profile             *OnboardingProfile `json:"profile"`
	OnboardingCompleted bool               `json:"onboarding_completed"`
	OnboardingVersion   int                `json:"onboarding_version"`
	GoalsSource         string             `json:"goals_source"`
	ID                  int64              `json:"id"`
	Email               string             `json:"email"`
	Name                string             `json:"name"`
	Timezone            string             `json:"timezone"`
	Goals               Nutrition          `json:"goals"`
}
type Log struct {
	ID           int64     `json:"id"`
	FoodID       int64     `json:"food_id"`
	Day          string    `json:"day"`
	Meal         string    `json:"meal"`
	Quantity     float64   `json:"quantity"`
	ServingName  string    `json:"serving_name"`
	ServingGrams float64   `json:"serving_grams"`
	FoodName     string    `json:"food_name"`
	Brand        string    `json:"brand"`
	Reference    Nutrition `json:"reference"`
	Nutrition    Nutrition `json:"nutrition"`
	Version      int       `json:"version"`
}
type Weight struct {
	ID  int64   `json:"id"`
	Day string  `json:"day"`
	KG  float64 `json:"kg"`
}
