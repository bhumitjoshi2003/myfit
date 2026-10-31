import { useEffect, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import { Leaf, ArrowRight, ArrowLeft, Check, LogOut } from "lucide-react";
import { api, keys, round } from "./api";
import type {
  User,
  OnboardingProfile,
  OnboardingState,
  Nutrition,
} from "./api";
const steps = [
  "About you",
  "Movement",
  "Your goal",
  "Preferences",
  "Review",
  "Your targets",
];
const lifestyleOptions = [
  ["sedentary", "Sedentary / desk job"],
  ["light", "Light activity"],
  ["moderate", "Moderate activity"],
  ["high", "High daily physical activity"],
];
const objectiveOptions = [
  ["gain", "Gain weight"],
  ["muscle", "Gain muscle with controlled weight gain"],
  ["maintain", "Maintain weight"],
  ["lose", "Lose weight"],
];
const titles = [
  "Let’s start with you.",
  "What does your day look like?",
  "What are you working towards?",
  "Make room for your needs.",
  "A quick check before we calculate.",
  "Your starting point, made personal.",
];
const names = {
  calories: "Calories (kcal)",
  protein: "Protein (g)",
  carbs: "Carbohydrates (g)",
  fat: "Fat (g)",
  fibre: "Fibre (g)",
};
function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
    </label>
  );
}
function Select({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: string[][];
}) {
  return (
    <Field label={label}>
      <select required value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="" disabled>
          Choose an option
        </option>
        {options.map(([v, l]) => (
          <option key={v} value={v}>
            {l}
          </option>
        ))}
      </select>
    </Field>
  );
}
function blank(user: User): OnboardingProfile {
  return {
    dob: "",
    age: 0,
    age_as_of: "",
    sex: "",
    height: 0,
    height_unit: "cm",
    weight: 0,
    weight_unit: "kg",
    units: "metric",
    timezone: user.timezone,
    lifestyle: "",
    sessions: 0,
    minutes: 60,
    intensity: "moderate",
    training: "none",
    objective: "",
    rate_percent: 0,
    target_weight: null,
    diet: "",
    restrictions: "",
    health_context: "",
  };
}
export default function Onboarding({
  user,
  onDone,
  onCancel,
  onLogout,
}: {
  user: User;
  onDone: (u: User) => void;
  onCancel?: () => void;
  onLogout: () => void;
}) {
  const [state, setState] = useState<OnboardingState | null>(null),
    [p, setP] = useState<OnboardingProfile>(blank(user)),
    [step, setStep] = useState(0),
    [loading, setLoading] = useState(true),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [custom, setCustom] = useState(false),
    [manual, setManual] = useState<Nutrition>(user.goals),
    [reload, setReload] = useState(0);
  useEffect(() => {
    let active = true;
    setLoading(true);
    api<OnboardingState>("/onboarding")
      .then((s) => {
        if (!active) return;
        setState(s);
        const data = s.user.profile
          ? { ...s.user.profile }
          : { ...blank(s.user), weight: s.latest_weight?.kg || 0 };
        data.timezone = s.user.timezone;
        if (data.units === "imperial") {
          data.height /= 2.54;
          data.height_unit = "in";
          data.weight /= 0.45359237;
          data.weight_unit = "lb";
          if (data.target_weight !== null) data.target_weight /= 0.45359237;
        }
        setP(data);
        if (!s.user.onboarding_completed && s.recommendation) {
          setStep(5);
          setCustom(!s.recommendation.result.supported);
          setManual(
            s.recommendation.result.supported
              ? s.recommendation.result.goals
              : s.user.goals,
          );
        }
        setError("");
      })
      .catch((e) => {
        if (active) setError(e.message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [user.id, reload]);
  function set<K extends keyof OnboardingProfile>(
    key: K,
    value: OnboardingProfile[K],
  ) {
    setP((v) => ({ ...v, [key]: value }));
  }
  function units(value: string) {
    if (p.units === value) return;
    const imperial = value === "imperial";
    const factor = imperial ? 1 / 0.45359237 : 0.45359237;
    setP((v) => ({
      ...v,
      units: value,
      height: v.height * (imperial ? 1 / 2.54 : 2.54),
      height_unit: imperial ? "in" : "cm",
      weight: v.weight * factor,
      weight_unit: imperial ? "lb" : "kg",
      target_weight: v.target_weight === null ? null : v.target_weight * factor,
    }));
  }
  async function next(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    if (step < 4) {
      setStep(step + 1);
      return;
    }
    setBusy(true);
    try {
      const s = await api<OnboardingState>("/onboarding/profile", "PUT", {
        version: state!.user.onboarding_version,
        profile: p,
      });
      setState(s);
      setStep(5);
      setCustom(!s.recommendation!.result.supported);
      setManual(
        s.recommendation!.result.supported
          ? s.recommendation!.result.goals
          : s.user.goals,
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function choose(mode: string) {
    setBusy(true);
    setError("");
    try {
      const u = await api<User>("/onboarding/goals", "POST", {
        version: state!.user.onboarding_version,
        recommendation_id: state!.recommendation!.id,
        mode,
        ...(mode === "custom" ? { goals: manual } : {}),
      });
      onDone(u);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  const rec = state?.recommendation?.result;
  const fmtWeight = (n: number) =>
    round(p.units === "imperial" ? n / 0.45359237 : n, 2) +
    (p.units === "imperial" ? " lb" : " kg");
  return (
    <div className="onboarding-page">
      <header className="onboarding-header">
        <a href="/" className="brand">
          <span className="brand-symbol">
            <Leaf />
          </span>
          myfit<span className="brand-dot">.</span>
        </a>
        <span className="muted small">Your goals. Your pace.</span>
        <button
          className="text-btn"
          onClick={async () => {
            try {
              await api("/logout", "POST", {});
              onLogout();
            } catch (e) {
              setError((e as Error).message);
            }
          }}
        >
          <LogOut size={16} />
          Sign out
        </button>
      </header>
      <main className="onboarding-main">
        <div className="onboarding-intro">
          <span className="eyebrow">
            {user.onboarding_completed
              ? "REVIEW YOUR PERSONAL PLAN"
              : "A LITTLE ABOUT YOU"}
          </span>
          <h1>{titles[step]}</h1>
          <p>
            {step === 5
              ? "A practical estimate to begin with. You decide what becomes your daily goal."
              : "A few details help us build a useful starting point. Your food and weight history stay with you."}
          </p>
        </div>
        <ol className="wizard-progress" aria-label="Onboarding progress">
          {steps.map((title, i) => (
            <li
              key={title}
              className={i === step ? "current" : i < step ? "complete" : ""}
              aria-current={i === step ? "step" : undefined}
            >
              <span>{i < step ? <Check size={13} /> : i + 1}</span>
              <small>{title}</small>
            </li>
          ))}
        </ol>
        {error && (
          <div className="error" role="alert">
            {error}
            <button
              type="button"
              className="text-btn"
              onClick={() => setReload((v) => v + 1)}
            >
              Reload saved profile
            </button>
          </div>
        )}
        {loading ? (
          <div className="card empty">Loading your profile…</div>
        ) : !state ? (
          <button className="btn" onClick={() => setReload((v) => v + 1)}>
            Try again
          </button>
        ) : step < 5 ? (
          <form className="card wizard-card" onSubmit={next}>
            {step === 0 && (
              <>
                <h2>The basics</h2>
                <Select
                  label="Preferred measurement units"
                  value={p.units}
                  onChange={units}
                  options={[
                    ["metric", "Metric · cm and kg"],
                    ["imperial", "Imperial · feet, inches and pounds"],
                  ]}
                />
                <div className="two-fields">
                  <Field label="Date of birth (optional)">
                    <input
                      type="date"
                      value={p.dob}
                      max={new Date().toISOString().slice(0, 10)}
                      onChange={(e) => set("dob", e.target.value)}
                    />
                  </Field>
                  <Field
                    label={
                      p.dob
                        ? "Age (calculated from date of birth)"
                        : "Age in years"
                    }
                  >
                    <input
                      type="number"
                      disabled={!!p.dob}
                      required={!p.dob}
                      min="1"
                      max="120"
                      value={p.dob ? "" : p.age || ""}
                      onChange={(e) => set("age", Number(e.target.value))}
                    />
                  </Field>
                </div>
                <Select
                  label="Sex coefficient for metabolic estimation"
                  value={p.sex}
                  onChange={(v) => set("sex", v)}
                  options={[
                    ["male", "Male formula"],
                    ["female", "Female formula"],
                    ["unspecified", "Prefer not to select · manual goals"],
                  ]}
                />
                <p className="small muted">
                  This is a formula input, not a statement about gender. Choose
                  manual targets if these coefficients are not appropriate for
                  you.
                </p>
                <div className="two-fields">
                  {p.units === "metric" ? (
                    <Field label="Height (cm)">
                      <input
                        type="number"
                        min="80"
                        max="250"
                        step="any"
                        required
                        value={p.height || ""}
                        onChange={(e) => set("height", Number(e.target.value))}
                      />
                    </Field>
                  ) : (
                    <div className="two-fields">
                      <Field label="Height (feet)">
                        <input
                          type="number"
                          min="2"
                          max="8"
                          step="1"
                          required
                          value={Math.floor(p.height / 12) || ""}
                          onChange={(e) =>
                            set(
                              "height",
                              Number(e.target.value) * 12 + (p.height % 12),
                            )
                          }
                        />
                      </Field>
                      <Field label="Height (inches)">
                        <input
                          type="number"
                          min="0"
                          max="11.999"
                          step="any"
                          required
                          value={Number((p.height % 12).toFixed(3))}
                          onChange={(e) =>
                            set(
                              "height",
                              Math.floor(p.height / 12) * 12 +
                                Number(e.target.value),
                            )
                          }
                        />
                      </Field>
                    </div>
                  )}
                  <Field
                    label={
                      p.units === "metric"
                        ? "Current weight (kg)"
                        : "Current weight (lb)"
                    }
                  >
                    <input
                      type="number"
                      min={p.units === "metric" ? 15 : 33.07}
                      max={p.units === "metric" ? 350 : 771.62}
                      step="any"
                      required
                      value={p.weight ? Number(p.weight.toFixed(4)) : ""}
                      onChange={(e) => set("weight", Number(e.target.value))}
                    />
                  </Field>
                </div>
                {!user.profile && state.latest_weight && (
                  <p className="small muted">
                    Prefilled from your recorded weight on{" "}
                    {state.latest_weight.day}. Please check it is still current.
                  </p>
                )}
                <Field label="IANA time zone">
                  <input
                    required
                    value={p.timezone}
                    onChange={(e) => set("timezone", e.target.value)}
                    placeholder="Asia/Kolkata"
                  />
                </Field>
              </>
            )}
            {step === 1 && (
              <>
                <h2>Outside the gym</h2>
                <p className="muted small">
                  Describe your usual work, chores and travel. Exclude the
                  workouts below to avoid counting them twice.
                </p>
                <Select
                  label="Everyday lifestyle"
                  value={p.lifestyle}
                  onChange={(v) => set("lifestyle", v)}
                  options={lifestyleOptions}
                />
                <h2 className="wizard-subheading">Your workouts</h2>
                <div className="two-fields">
                  <Field label="Workout sessions per week">
                    <input
                      type="number"
                      min="0"
                      max="7"
                      step="0.5"
                      required
                      value={p.sessions}
                      onChange={(e) => {
                        const n = Number(e.target.value);
                        setP((v) => ({
                          ...v,
                          sessions: n,
                          training:
                            n > 0 && v.training === "none"
                              ? "mixed"
                              : n === 0
                                ? "none"
                                : v.training,
                          minutes: n === 0 ? 0 : v.minutes || 60,
                        }));
                      }}
                    />
                  </Field>
                  <Field label="Average minutes per session">
                    <input
                      type="number"
                      min={p.sessions > 0 ? 10 : 0}
                      max="180"
                      required
                      disabled={p.sessions === 0}
                      value={p.minutes}
                      onChange={(e) => set("minutes", Number(e.target.value))}
                    />
                  </Field>
                </div>
                {p.sessions > 0 && (
                  <>
                    <Select
                      label="Workout intensity"
                      value={p.intensity}
                      onChange={(v) => set("intensity", v)}
                      options={[
                        ["low", "Low · easy effort"],
                        ["moderate", "Moderate · sustained effort"],
                        ["high", "High · vigorous effort"],
                      ]}
                    />
                    <Select
                      label="Primary training type"
                      value={p.training}
                      onChange={(v) => set("training", v)}
                      options={[
                        ["resistance", "Resistance training"],
                        ["cardio", "Cardio"],
                        ["mixed", "Mixed training"],
                      ]}
                    />
                  </>
                )}
                <p className="source-note">
                  Alternate-day training can be entered as 3.5 sessions/week. We
                  estimate net exercise energy and average it over seven days;
                  frequency alone does not determine your activity level.
                </p>
              </>
            )}
            {step === 2 && (
              <>
                <h2>Your direction</h2>
                <Select
                  label="Fitness objective"
                  value={p.objective}
                  onChange={(v) =>
                    setP((p) => ({
                      ...p,
                      objective: v,
                      rate_percent:
                        v === "maintain" ? 0 : v === "lose" ? 0.5 : 0.25,
                      target_weight: null,
                    }))
                  }
                  options={objectiveOptions}
                />
                {p.objective && p.objective !== "maintain" && (
                  <>
                    <Field label="Preferred change (% of body weight per week)">
                      <input
                        type="number"
                        min="0.1"
                        max={p.objective === "muscle" ? 0.5 : 1}
                        step="0.05"
                        required
                        value={p.rate_percent || ""}
                        onChange={(e) =>
                          set("rate_percent", Number(e.target.value))
                        }
                      />
                    </Field>
                    <p className="small muted">
                      A conservative starting preference is 0.25% for gain and
                      0.5% for loss. Calorie adjustments are capped; the
                      estimated pace may be slower and is uncertain.
                    </p>
                    <Field
                      label={`Target weight (${p.units === "imperial" ? "lb" : "kg"}, optional)`}
                    >
                      <input
                        type="number"
                        step="any"
                        min={p.units === "imperial" ? 33.07 : 15}
                        max={p.units === "imperial" ? 771.62 : 350}
                        value={p.target_weight ?? ""}
                        onChange={(e) =>
                          set(
                            "target_weight",
                            e.target.value === ""
                              ? null
                              : Number(e.target.value),
                          )
                        }
                      />
                    </Field>
                  </>
                )}
                <p className="source-note">
                  A target weight is a direction, not a deadline. Muscle gain
                  cannot be predicted from a calorie surplus.
                </p>
              </>
            )}
            {step === 3 && (
              <>
                <h2>Food that fits your life</h2>
                <Select
                  label="Dietary preference"
                  value={p.diet}
                  onChange={(v) => set("diet", v)}
                  options={[
                    ["vegetarian", "Vegetarian"],
                    ["eggs", "Vegetarian with eggs"],
                    ["nonvegetarian", "Non-vegetarian"],
                    ["other", "Other"],
                  ]}
                />
                <Field label="Allergies, restrictions or dietary notes (optional)">
                  <textarea
                    rows={3}
                    maxLength={1000}
                    value={p.restrictions}
                    onChange={(e) => set("restrictions", e.target.value)}
                  />
                </Field>
                <p className="small muted">
                  Preferences are saved for future food suggestions. They do not
                  change calorie expenditure. The food catalogue is not an
                  allergy checker.
                </p>
                <h2 className="wizard-subheading">
                  Is a standard estimate suitable?
                </h2>
                <Select
                  label="Health and suitability"
                  value={p.health_context}
                  onChange={(v) => set("health_context", v)}
                  options={[
                    [
                      "healthy",
                      "Apparently healthy adult; no special nutrition needs",
                    ],
                    [
                      "clinician",
                      "Special needs, relevant condition or medication, or unsure",
                    ],
                  ]}
                />
                <p className="source-note">
                  Choose special needs for pregnancy, breastfeeding,
                  eating-disorder history, or any condition or medication
                  affecting nutrition. Standard estimates are limited to ages
                  18–65 and a supported body-size range. You can complete setup
                  with clinician-guided manual goals.
                </p>
              </>
            )}
            {step === 4 && (
              <>
                <h2>Ready for a useful starting point</h2>
                <dl className="review-grid">
                  <dt>Age / birth date</dt>
                  <dd>{p.dob || `${p.age} years`}</dd>
                  <dt>Formula</dt>
                  <dd>{p.sex}</dd>
                  <dt>Measurements</dt>
                  <dd>
                    {round(p.height, 2)} {p.height_unit} · {round(p.weight, 2)}{" "}
                    {p.weight_unit}
                  </dd>
                  <dt>Time zone</dt>
                  <dd>{p.timezone}</dd>
                  <dt>Daily activity</dt>
                  <dd>
                    {lifestyleOptions.find(([v]) => v === p.lifestyle)?.[1]}
                  </dd>
                  <dt>Workouts</dt>
                  <dd>
                    {p.sessions} / week · {p.minutes} min · {p.intensity} ·{" "}
                    {p.training}
                  </dd>
                  <dt>Objective</dt>
                  <dd>
                    {objectiveOptions.find(([v]) => v === p.objective)?.[1]}
                    {p.objective !== "maintain"
                      ? ` · ${p.rate_percent}% / week`
                      : ""}
                  </dd>
                  <dt>Target weight</dt>
                  <dd>
                    {p.target_weight === null
                      ? "Not set"
                      : `${round(p.target_weight, 2)} ${p.weight_unit}`}
                  </dd>
                  <dt>Diet</dt>
                  <dd>{p.diet}</dd>
                  <dt>Dietary notes</dt>
                  <dd>{p.restrictions || "None entered"}</dd>
                  <dt>Suitability</dt>
                  <dd>
                    {p.health_context === "healthy"
                      ? "Standard adult estimate requested"
                      : "Clinician-guided manual targets"}
                  </dd>
                </dl>
                <div className="reference-notice">
                  <p>
                    Saving calculates recommendations on the server. Your active
                    goals, food diary and weight history are preserved until you
                    choose targets.
                  </p>
                </div>
              </>
            )}
            <div className="wizard-actions">
              {step > 0 ? (
                <button
                  type="button"
                  className="btn"
                  disabled={busy}
                  onClick={() => {
                    setError("");
                    setStep(step - 1);
                  }}
                >
                  <ArrowLeft size={15} />
                  Previous
                </button>
              ) : onCancel ? (
                <button type="button" className="btn" onClick={onCancel}>
                  Back to profile
                </button>
              ) : (
                <span />
              )}
              <button className="btn primary" disabled={busy}>
                {busy
                  ? "Calculating…"
                  : step === 4
                    ? "Save profile & calculate"
                    : "Next"}
                <ArrowRight size={16} />
              </button>
            </div>
          </form>
        ) : (
          rec && (
            <section className="card wizard-card recommendation-results">
              <div className="card-heading">
                <h2>
                  {rec.supported
                    ? "Your recommended daily targets"
                    : "Let’s use guided manual targets"}
                </h2>
                <span className="badge">
                  {state.user.goals_source === "custom"
                    ? "CUSTOM GOALS PRESERVED"
                    : "YOUR CHOICE"}
                </span>
              </div>
              {rec.supported && (
                <>
                  <div className="recommendation-stats">
                    <div>
                      <span>Resting expenditure</span>
                      <strong>
                        {round(rec.ree, 0)}
                        <small> kcal/day</small>
                      </strong>
                    </div>
                    <div>
                      <span>Estimated maintenance</span>
                      <strong>
                        {round(rec.maintenance, 0)}
                        <small> kcal/day</small>
                      </strong>
                    </div>
                    <div>
                      <span>Suggested calorie range</span>
                      <strong>
                        {round(rec.calorie_range.min, 0)}–
                        {round(rec.calorie_range.max, 0)}
                      </strong>
                    </div>
                    <div>
                      <span>Starting calorie target</span>
                      <strong>
                        {round(rec.goals.calories, 0)}
                        <small> kcal/day</small>
                      </strong>
                    </div>
                  </div>
                  <div className="target-grid">
                    {(["protein", "carbs", "fat", "fibre"] as const).map(
                      (k) => (
                        <div key={k}>
                          <span>{names[k].replace(" (g)", "")}</span>
                          <strong>{round(rec.goals[k])} g</strong>
                          {k === "protein" && (
                            <small>
                              Range {round(rec.protein_range.min)}–
                              {round(rec.protein_range.max)} g
                            </small>
                          )}
                          {k === "fat" && (
                            <small>
                              Range {round(rec.fat_range.min)}–
                              {round(rec.fat_range.max)} g
                            </small>
                          )}
                        </div>
                      ),
                    )}
                  </div>
                  <p className="pace-note">
                    Uncertain estimated pace:{" "}
                    {rec.pace_kg_per_week > 0
                      ? "gain"
                      : rec.pace_kg_per_week < 0
                        ? "loss"
                        : "maintenance"}{" "}
                    {fmtWeight(Math.abs(rec.pace_kg_per_week))}/week. This is
                    not a promised outcome.
                  </p>
                  <details className="calculation-details">
                    <summary>Why these targets?</summary>
                    {rec.explanations.map((x) => (
                      <p key={x}>{x}</p>
                    ))}
                    <p>
                      Calculation {rec.version} ·{" "}
                      {new Date(
                        state.recommendation!.calculated_at,
                      ).toLocaleString()}
                    </p>
                    <p>
                      <a
                        href="https://pubmed.ncbi.nlm.nih.gov/2305711/"
                        target="_blank"
                        rel="noreferrer"
                      >
                        Mifflin–St Jeor
                      </a>{" "}
                      ·{" "}
                      <a
                        href="https://pmc.ncbi.nlm.nih.gov/articles/PMC2117006/"
                        target="_blank"
                        rel="noreferrer"
                      >
                        ISSN protein reference
                      </a>{" "}
                      ·{" "}
                      <a
                        href="https://www.canada.ca/en/health-canada/services/food-nutrition/healthy-eating/dietary-reference-intakes/tables/reference-values-macronutrients.html"
                        target="_blank"
                        rel="noreferrer"
                      >
                        Dietary reference ranges
                      </a>
                    </p>
                  </details>
                </>
              )}
              {state.user.onboarding_completed && (
                <div className="goal-comparison">
                  <h3>Review the change</h3>
                  <p>
                    Active goals ({state.user.goals_source}) → proposed targets.
                    Your active goals have not changed.
                  </p>
                  {rec.supported && (
                    <dl>
                      {keys.map((k) => (
                        <div key={k}>
                          <dt>{names[k]}</dt>
                          <dd>
                            {round(state.user.goals[k])} → {round(rec.goals[k])}
                          </dd>
                        </div>
                      ))}
                    </dl>
                  )}
                </div>
              )}
              <div className="limitation-note">
                {rec.limitations.map((x) => (
                  <p key={x}>{x}</p>
                ))}
              </div>
              {custom ? (
                <form
                  onSubmit={(e) => {
                    e.preventDefault();
                    void choose("custom");
                  }}
                >
                  <h3>
                    {rec.supported
                      ? "Choose your own daily goals"
                      : "Enter clinician-guided daily goals"}
                  </h3>
                  <div className="goals-fields">
                    {keys.map((k) => (
                      <Field key={k} label={names[k]}>
                        <input
                          type="number"
                          required
                          step="any"
                          min={k === "calories" ? 1 : 0}
                          max={k === "calories" ? 15000 : 2000}
                          value={manual[k]}
                          onChange={(e) =>
                            setManual((v) => ({
                              ...v,
                              [k]: Number(e.target.value),
                            }))
                          }
                        />
                      </Field>
                    ))}
                  </div>
                  <p className="small muted">
                    Manual goals are your selected values; the app does not
                    certify their medical suitability.
                  </p>
                  <button className="btn primary wide" disabled={busy}>
                    {busy ? "Saving…" : "Save custom goals"}
                  </button>
                  {rec.supported && (
                    <button
                      type="button"
                      className="text-btn"
                      onClick={() => setCustom(false)}
                    >
                      Back to recommended targets
                    </button>
                  )}
                </form>
              ) : (
                <div className="result-actions">
                  <button
                    className="btn primary"
                    disabled={busy}
                    onClick={() => choose("recommendation")}
                  >
                    Accept recommended goals
                    <Check size={16} />
                  </button>
                  <button
                    className="btn"
                    onClick={() => {
                      setManual(
                        state.user.goals_source === "custom"
                          ? state.user.goals
                          : rec.goals,
                      );
                      setCustom(true);
                    }}
                  >
                    Customize goals manually
                  </button>
                </div>
              )}
              <div className="wizard-actions">
                <button
                  className="text-btn"
                  disabled={busy}
                  onClick={() => {
                    setStep(0);
                    setError("");
                  }}
                >
                  <ArrowLeft size={15} />
                  Edit details
                </button>
                {state.user.onboarding_completed && (
                  <button
                    className="text-btn"
                    disabled={busy}
                    onClick={() => choose("keep")}
                  >
                    Keep current goals
                  </button>
                )}
              </div>
            </section>
          )
        )}
      </main>
    </div>
  );
}
