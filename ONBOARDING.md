# Personalized onboarding and nutrition estimates

The feature uses migration **005_personalized_onboarding.sql**. It adds nullable profile data, a completion flag, an optimistic version and goal provenance to `users`, plus an append-only recommendation calculation table. Existing goals, food logs, food snapshots, weights, sessions and the Docker volume are preserved. New users start with zero/unset goals and must explicitly select targets in the wizard. Existing users with missing profile data see the same wizard after login; a recorded weight is a clearly labelled prefill, never an invented measurement. Setup does not add or modify a weight-history record.

The six screens cover personal measurements, lifestyle and workouts, objective/rate, diet and suitability, review, and results. Both cm/kg and feet+inches/lb inputs are supported. Measurements are canonicalized **on the backend**; a pound is 0.45359237 kg and an inch is 2.54 cm. Preferred display units are retained for the wizard. The existing diary and weight screens continue to display grams and kilograms.

Date of birth takes precedence over an entered age. Age is calculated using the selected IANA time zone on the server. An age-only input is anchored to the stored `age_as_of` date; users should verify it on subsequent recalculations. Birth date is preferable for automatic age derivation. Other measurements are user-declared current values, not automatically changed by weight check-ins.

## Versioned calculation rules

The independently testable service is `backend/internal/domain/recommendation.go`. The default policy is `mifflin-net-activity-v1`; calculation receives an explicit timestamp so the same inputs, policy and timestamp give the same result. Policy constants are in `RecommendationConfig`, separate from user inputs. Changing the production policy requires a new calculation version. The HTTP API uses the server's default policy and time, never a client-supplied algorithm or calculation date.

1. **Resting expenditure:** `10 × weight_kg + 6.25 × height_cm − 5 × age + coefficient`, where the selected male coefficient is +5 and female coefficient −161. This implements the simplified [Mifflin–St Jeor equation](https://pubmed.ncbi.nlm.nih.gov/2305711/).
2. **Non-workout activity:** resting expenditure multiplied by 1.30 for sedentary, 1.45 light, 1.60 moderate or 1.75 high. These are deliberately coarse **MyFit policy assumptions**, not validated individual measurements. Users must exclude listed workouts from this choice.
3. **Exercise allowance:** gross intensity proxies of 3, 5 or 7 MET for low, moderate or high effort. Net average kcal/day is `max(0, MET − lifestyle_factor) × kg × duration_hours × sessions_per_week / 7`. Subtracting the already represented non-workout baseline avoids adding a second full-day activity multiplier. This is a conservative approximation; it cannot account precisely for rest intervals, activity compensation or individual efficiency. A MET's approximate kcal/kg/hour interpretation comes from the [Physical Activity Compendium](https://pacompendium.com/); MyFit's broad intensity categories are product proxies, not claimed Compendium activity codes. Training type influences protein selection rather than adding another energy factor.
4. **Maintenance:** non-workout expenditure plus the net weekly-averaged exercise allowance.
5. **Objective:** default preferred weekly weight change is 0.25% of body weight for gain or controlled muscle gain, 0.5% for loss and zero for maintenance. A user may select 0.1–1% (at most 0.5% for controlled muscle gain); zero in an API weight-change request uses the policy default. The static planning conversion is 7,700 kcal/kg: `kg × weekly_percent / 100 × 7700 / 7`. Gain adjustments are capped at 15% of maintenance, muscle gain at 10%, loss at 20%. Starting calories round to 10 kcal. The displayed *effective* pace uses the actual rounded adjustment, not the requested rate. These caps and defaults are conservative application choices informed by [research on small versus large energy surpluses](https://pmc.ncbi.nlm.nih.gov/articles/PMC10620361/). That trial concerns resistance-trained participants, not every app user. The static conversion is not a physiological prediction and cannot predict muscle gain; see [NIDDK's dynamic modelling context](https://www.niddk.nih.gov/health-information/weight-management/body-weight-planner).
6. **Protein:** without workouts, a starting policy of 1.0 g/kg within 0.8–1.2 g/kg; with workouts, 1.6 g/kg within 1.4–2.0 g/kg. Resistance or mixed training, or a muscle objective with workouts, selects 1.8 g/kg. The exercising range is grounded in the [ISSN position statement](https://pmc.ncbi.nlm.nih.gov/articles/PMC2117006/). Protein range is intersected with the macro allocation envelope; if no feasible intersection exists, use manual goals. The final target must also supply at least 10% of energy. These are estimates for the supported population, not a prescription for kidney disease or other special needs.
7. **Fat and carbohydrate:** fat starts at 30% of calories, within 20–35%; protein uses its weight-based allocation and carbohydrate fills remaining energy, within 45–65%. Allocation reduces fat if needed to retain at least 45% carbohydrate. Protein is constrained to at most 35% of energy. All supported calculations exactly satisfy `protein × 4 + carbohydrate × 4 + fat × 9 = calories`, before display rounding. These broad ranges come from the [dietary reference macro ranges](https://www.canada.ca/en/health-canada/services/food-nutrition/healthy-eating/dietary-reference-intakes/tables/reference-values-macronutrients.html). If constraints cannot all be met, no automatic target is offered.
8. **Fibre:** 14 g per 1,000 kcal, rounded to a whole gram, using the same dietary reference source.
9. **Ranges:** calorie suggestions show ±5% around the starting target, constrained by the app's automated envelope and resting estimate. This is a practical adjustment band, **not a statistical confidence interval**. Protein and fat ranges show feasible daily ranges, not precision claims. Dietary preferences and allergy notes do not change expenditure or targets; they are stored for future food suggestions, not used for allergy detection.

For the requested 24-year-old male, 167.64 cm, 54.5 kg, sedentary outside the gym, 3.5 moderate-intensity 60-minute resistance sessions/week, and controlled gain at 0.25%/week:

| Result | Calculation output |
| --- | --- |
| Resting expenditure | 1,477.75 kcal/day |
| Non-workout baseline | 1,921.075 kcal/day |
| Net exercise allowance | 100.825 kcal/day |
| Estimated maintenance | 2,021.9 kcal/day |
| Starting calories | 2,170 kcal/day |
| Protein | 98.1 g/day (range 76.3–109 g) |
| Fat | 72.333… g/day |
| Carbohydrate | 281.65 g/day |
| Fibre | 30 g/day |
| Uncertain starting pace | about +0.135 kg/week |

The supplied approximate resting value of 1,469 differs from the standard formula by 8.75 kcal (about 0.6%). The comparison values 2,350 kcal and 100 g protein are not hard-coded. A different session duration, intensity or outside-gym lifestyle changes the output. These values are test fixtures, not defaults for new users.

## Supported scope and limitations

Automated goals are limited by app policy to apparently healthy adults aged **18–65**, BMI **18.5–<40**, with a selected formula coefficient. BMI is an applicability check, not a diagnosis. Users may enter valid measurements outside this scope and complete setup with manual goals. Pregnancy, breastfeeding, eating-disorder history, relevant medical conditions/medications or uncertainty should use clinician-guided targets. Choosing not to select a sex coefficient also uses manual goals. The app does not diagnose or screen comprehensively for medical conditions.

Targets below 1,500 kcal/day, below estimated resting expenditure, or above 4,500 kcal/day are not automatically offered. This is an intentionally conservative product envelope, not a universal medical cutoff. The app never replaces an out-of-envelope estimate with a misleading clamped recommendation. Clinician-guided custom targets retain the existing numeric validation bounds (positive calories up to 15,000 and each macro 0–2,000 g). Manual targets need not satisfy the automated macro equation; they are labelled custom and are not certified as medically suitable.

Calorie expenditure, intensity, absorption, water changes, adaptive metabolism and food references all introduce uncertainty. Users should review trends after 2–4 weeks. There is no automatic diet adjustment, target-weight deadline or promise of a specific rate of muscle gain.

## Persistence and decisions

- `GET /api/onboarding`: only the authenticated user's profile, latest recommendation and latest recorded weight.
- `PUT /api/onboarding/profile`: `{version, profile}`. Validates, normalizes, calculates and transactionally saves a profile snapshot, calculation version, timestamp and result. It increments the optimistic version; it does **not** change active goals. Previous pending suggestions become rejected/superseded.
- `POST /api/onboarding/goals`: `{version, recommendation_id, mode, goals?}`. Mode is `recommendation`, `custom` or `keep`. Recommendation mode reads its values from the server-stored result, ignoring any submitted nutrient values. Custom mode validates the user's chosen goals. Keep is available only after initial onboarding and explicitly rejects the latest suggestion while preserving active goals. The current recommendation must belong to the session user and be their latest record. Stale requests return 409.
- `PUT /api/profile` now also requires `version`. Updating goal values marks provenance custom; an unchanged goal set preserves its source. This protects a newer selection from another browser tab's stale form.
- Every new calculation has an immutable input/result snapshot and decision state. Saving profile information never silently overwrites active custom or accepted goals. Weight logging never changes the onboarding profile, active goals or calculation history. Food snapshots and weight history are not rewritten by onboarding.

The frontend gates the dashboard on `onboarding_completed`; low-level existing diary APIs remain available for compatibility and history access. Onboarding endpoints retain existing session authentication, CSRF and exact-origin validation. There are no user IDs accepted in onboarding requests. All DB reads/writes use the session owner. New profile and goal writes use optimistic versions; multi-row updates use transactions.

## Verification and operation

Run the app exactly as before:

```sh
./scripts/start.sh
# http://localhost:5173
./scripts/stop.sh
```

An already-running old API must be restarted to load the new migration and routes. PostgreSQL is not reset. Go's existing project toolchain is reused.

```sh
TEST_DATABASE_URL='postgres://myfit:myfit_local_dev@localhost:5433/myfit?sslmode=disable' go -C backend test -race ./...
go -C backend vet ./...
npm run build --prefix frontend
# With the app running:
npm run test:e2e --prefix frontend
```

Unit tests cover both formula coefficients, age/height/weight changes, birthday time zones, imperial conversion, activity/workout/objective effects, invalid/incomplete profiles, excluded populations, policy configuration, and calorie/macro consistency across a broad input matrix. PostgreSQL tests exercise an actual upgrade from migration 004 to 005 with a legacy account, diary and weight records, explicit decisions, custom-goal preservation, forged recommendation values, stale writes, CSRF and owner isolation. Browser tests cover metric and imperial onboarding, review/back navigation, reload recovery, accepting results, manual-only setup, recalculation/rejection with custom goals preserved, and the existing daily diary journey at desktop/mobile sizes. Chromium and mobile emulation are covered; Safari/Firefox and clinical validation are not.
