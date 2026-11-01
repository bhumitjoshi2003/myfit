export type Nutrition = {
  calories: number;
  protein: number;
  carbs: number;
  fat: number;
  fibre: number;
};
export type User = {
  profile: OnboardingProfile | null;
  onboarding_completed: boolean;
  onboarding_version: number;
  goals_source: string;
  id: number;
  name: string;
  email: string;
  timezone: string;
  goals: Nutrition;
};
export type Serving = { id: number; name: string; grams: number };
export type Food = Nutrition & {
  id: number;
  owner_id: number | null;
  name: string;
  brand: string;
  source: string;
  approximate: boolean;
  version: number;
  servings: Serving[];
  favourite: boolean;
};
export type Entry = {
  id: number;
  food_id: number;
  day: string;
  meal: string;
  quantity: number;
  serving_name: string;
  serving_grams: number;
  food_name: string;
  brand: string;
  reference: Nutrition;
  nutrition: Nutrition;
  version: number;
};
export type Weight = { id: number; day: string; kg: number };
export type Diary = {
  date: string;
  logs: Entry[];
  totals: Nutrition;
  meal_totals: Record<string, Nutrition>;
  remaining: Nutrition;
};
let csrf = "";
export function setCSRF(value: string) {
  csrf = value;
}
export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const r = await fetch("/api" + path, {
    method,
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await r.json();
  if (!r.ok) {
    if (r.status === 401 && !["/login", "/register", "/me"].includes(path))
      window.dispatchEvent(new Event("session-expired"));
    throw new Error(
      data.error?.message || "Something went wrong. Please try again.",
    );
  }
  return data;
}
export const emptyNutrition: Nutrition = {
  calories: 0,
  protein: 0,
  carbs: 0,
  fat: 0,
  fibre: 0,
};
export const keys = ["calories", "protein", "carbs", "fat", "fibre"] as const;
export const round = (n: number, d = 1) =>
  n.toLocaleString(undefined, { maximumFractionDigits: d });
export { today } from "./dates";

export type OnboardingProfile = {
  dob: string;
  age: number;
  age_as_of: string;
  sex: string;
  height: number;
  height_unit: string;
  weight: number;
  weight_unit: string;
  units: string;
  timezone: string;
  lifestyle: string;
  sessions: number;
  minutes: number;
  intensity: string;
  training: string;
  objective: string;
  rate_percent: number;
  target_weight: number | null;
  diet: string;
  restrictions: string;
  health_context: string;
};
export type Recommendation = {
  version: string;
  supported: boolean;
  age: number;
  ree: number;
  maintenance: number;
  lifestyle_factor: number;
  exercise_calories: number;
  calorie_range: { min: number; max: number };
  protein_range: { min: number; max: number };
  fat_range: { min: number; max: number };
  goals: Nutrition;
  pace_kg_per_week: number;
  effective_rate_percent: number;
  explanations: string[];
  limitations: string[];
};
export type OnboardingState = {
  user: User;
  recommendation: {
    id: number;
    profile_version: number;
    calculated_at: string;
    result: Recommendation;
    decision: string;
  } | null;
  latest_weight: Weight | null;
};
