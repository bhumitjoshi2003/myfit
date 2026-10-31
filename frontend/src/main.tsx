import Onboarding from "./Onboarding";
import React, { useState, useEffect, useCallback } from "react";
import { createRoot } from "react-dom/client";
import {
  LayoutDashboard,
  BookOpen,
  Library,
  ChartNoAxesCombined,
  SlidersHorizontal,
  Plus,
  ArrowUpRight,
  ChevronLeft,
  ChevronRight,
  Search,
  Star,
  X,
  Trash2,
  Pencil,
  LogOut,
  Leaf,
  Flame,
  Utensils,
  Check,
  Copy,
  Menu,
  Scale,
} from "lucide-react";
import { api, setCSRF, emptyNutrition, keys, round, today } from "./api";
import type { User, Food, Entry, Weight, Diary } from "./api";
import "./style.css";

const meals = ["breakfast", "lunch", "dinner", "snacks"];
const labels = {
  calories: "Calories",
  protein: "Protein",
  carbs: "Carbohydrates",
  fat: "Fat",
  fibre: "Fibre",
};
const nav = [
  { id: "dashboard", label: "Dashboard", icon: LayoutDashboard },
  { id: "diary", label: "Food Diary", icon: BookOpen },
  { id: "library", label: "Food Library", icon: Library },
  { id: "weight", label: "Weight Progress", icon: ChartNoAxesCombined },
  { id: "profile", label: "Profile / Goals", icon: SlidersHorizontal },
];
function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
    </label>
  );
}
function ErrorBox({ error }: { error: string }) {
  return error ? (
    <div role="alert" className="error">
      {error}
    </div>
  ) : null;
}
function Modal({
  title,
  close,
  children,
}: {
  title: string;
  close: () => void;
  children: React.ReactNode;
}) {
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if (e.key === "Escape") close();
    };
    document.addEventListener("keydown", h);
    const prev = document.activeElement as HTMLElement;
    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    dialog?.focus();
    return () => {
      document.removeEventListener("keydown", h);
      prev?.focus();
    };
  }, [close]);
  return (
    <div
      className="modal-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget) close();
      }}
    >
      <section
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-label={title}
        tabIndex={-1}
        onKeyDown={(e) => {
          if (e.key === "Tab") {
            const els = Array.from(
              e.currentTarget.querySelectorAll<HTMLElement>(
                'button,input,select,textarea,[tabindex="0"]',
              ),
            ).filter((x) => !x.hasAttribute("disabled"));
            const first = els[0],
              last = els[els.length - 1];
            if (e.shiftKey && document.activeElement === first) {
              e.preventDefault();
              last?.focus();
            } else if (!e.shiftKey && document.activeElement === last) {
              e.preventDefault();
              first?.focus();
            }
          }
        }}
      >
        <header>
          <h2>{title}</h2>
          <button
            className="icon-btn"
            aria-label="Close dialog"
            onClick={close}
          >
            <X size={20} />
          </button>
        </header>
        {children}
      </section>
    </div>
  );
}
function Auth({ signedIn }: { signedIn: (u: User) => void }) {
  const [register, setRegister] = useState(false),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const f = new FormData(e.currentTarget);
    const body = {
      email: f.get("email"),
      password: f.get("password"),
      ...(register
        ? {
            name: f.get("name"),
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          }
        : {}),
    };
    try {
      const r = await api<{ user: User; csrf: string }>(
        register ? "/register" : "/login",
        "POST",
        body,
      );
      setCSRF(r.csrf);
      signedIn(r.user);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="auth-page">
      <div className="auth-story">
        <a className="brand" href="/">
          {" "}
          <span className="brand-symbol">
            <Leaf />
          </span>
          myfit<span className="brand-dot">.</span>
        </a>
        <div>
          <span className="eyebrow">SMALL STEPS. LASTING CHANGE.</span>
          <h1>
            A little more
            <br />
            in tune with you.
          </h1>
          <p>
            Good nutrition starts with awareness.
            <br />
            Make space for a healthier everyday.
          </p>
          <div className="auth-art">
            <div className="art-ring">
              <Leaf size={64} />
            </div>
            <span className="art-pill">
              A fresh start, every day <span>↗</span>
            </span>
          </div>
        </div>
        <p className="muted">Your food. Your goals. Your pace.</p>
      </div>
      <main className="auth-form">
        <div className="auth-card">
          <span className="eyebrow">WELCOME TO MYFIT</span>
          <h2>
            {register ? "Start your next chapter." : "Good to see you again."}
          </h2>
          <p className="muted">
            {register
              ? "Build healthy habits, one meal at a time."
              : "Your everyday wellbeing, all in one place."}
          </p>
          <form onSubmit={submit}>
            {register && (
              <Field label="Your name">
                <input
                  name="name"
                  autoComplete="name"
                  required
                  maxLength={100}
                  placeholder="What should we call you?"
                />
              </Field>
            )}
            <Field label="Email address">
              <input
                name="email"
                type="email"
                required
                autoComplete="email"
                placeholder="you@example.com"
              />
            </Field>
            <Field label="Password">
              <input
                name="password"
                type="password"
                required
                minLength={register ? 10 : 1}
                maxLength={72}
                autoComplete={register ? "new-password" : "current-password"}
                placeholder={
                  register ? "At least 10 characters" : "Enter your password"
                }
              />
            </Field>
            <ErrorBox error={error} />
            <button className="btn primary wide" disabled={busy}>
              {busy ? "Please wait…" : register ? "Create account" : "Sign in"}
              <ArrowUpRight size={17} />
            </button>
          </form>
          <p className="auth-switch">
            {register ? "Already have an account?" : "New here?"}{" "}
            <button
              onClick={() => {
                setRegister(!register);
                setError("");
              }}
            >
              {register ? "Sign in" : "Create an account"}
            </button>
          </p>
          <p className="small muted">
            Your diary is private. Your goals are always yours to set.
          </p>
        </div>
      </main>
    </div>
  );
}
function App() {
  const [editOnboarding, setEditOnboarding] = useState(false);
  const [initialFood, setInitialFood] = useState<Food | null>(null);
  const [user, setUser] = useState<User | null>(null),
    [boot, setBoot] = useState(true),
    [page, setPage] = useState("dashboard"),
    [date, setDate] = useState(""),
    [diary, setDiary] = useState<Diary>({
      date: "",
      logs: [],
      totals: emptyNutrition,
      meal_totals: {},
      remaining: emptyNutrition,
    }),
    [weights, setWeights] = useState<Weight[]>([]),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(false),
    [adding, setAdding] = useState<string | null>(null),
    [editing, setEditing] = useState<Entry | null>(null),
    [weightModal, setWeightModal] = useState(false),
    [toast, setToast] = useState(""),
    [mobile, setMobile] = useState(false);
  useEffect(() => {
    api<{ user: User; csrf: string }>("/me")
      .then((r) => {
        setCSRF(r.csrf);
        setUser(r.user);
        setDate(today(r.user.timezone));
      })
      .catch(() => {})
      .finally(() => setBoot(false));
    const expire = () => {
      setUser(null);
      setCSRF("");
    };
    window.addEventListener("session-expired", expire);
    return () => window.removeEventListener("session-expired", expire);
  }, []);
  const refresh = useCallback(async () => {
    if (!user || !user.onboarding_completed || !date) return;
    setLoading(true);
    setError("");
    try {
      const [d, w] = await Promise.all([
        api<Diary>("/diary?date=" + date),
        api<Weight[]>("/weights"),
      ]);
      setDiary(d);
      setWeights(w);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [user, date]);
  useEffect(() => {
    let active = true;
    if (!user || !user.onboarding_completed || !date) return;
    setLoading(true);
    setError("");
    Promise.all([api<Diary>("/diary?date=" + date), api<Weight[]>("/weights")])
      .then(([d, w]) => {
        if (active) {
          setDiary(d);
          setWeights(w);
        }
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
  }, [user, date]);
  useEffect(() => {
    if (toast) {
      const t = setTimeout(() => setToast(""), 3500);
      return () => clearTimeout(t);
    }
  }, [toast]);
  function signedIn(u: User) {
    setEditOnboarding(false);
    setMobile(false);
    setToast("");
    setUser(u);
    setDate(today(u.timezone));
    setPage("dashboard");
  }
  function changeDate(delta: number) {
    const d = new Date(date + "T12:00:00Z");
    d.setUTCDate(d.getUTCDate() + delta);
    setDate(d.toISOString().slice(0, 10));
  }
  async function remove(l: Entry) {
    try {
      await api("/logs/" + l.id, "DELETE", { version: l.version });
      await refresh();
      setToast("Entry deleted");
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function reuse(l: Entry) {
    try {
      await api("/logs", "POST", {
        reuse_id: l.id,
        day: date,
        meal: l.meal,
        quantity: l.quantity,
      });
      await refresh();
      setToast("Meal entry added again");
    } catch (e) {
      setError((e as Error).message);
    }
  }
  if (boot)
    return (
      <div className="boot">
        <Leaf size={36} />
        <p>Getting your day ready…</p>
      </div>
    );
  if (!user) return <Auth signedIn={signedIn} />;
  if (!user.onboarding_completed || editOnboarding)
    return (
      <Onboarding
        user={user}
        onDone={(u) => {
          setUser(u);
          setEditOnboarding(false);
          setDate(today(u.timezone));
          setToast("Your goals are saved");
        }}
        onCancel={
          user.onboarding_completed ? () => setEditOnboarding(false) : undefined
        }
        onLogout={() => {
          setUser(null);
          setCSRF("");
          setEditOnboarding(false);
        }}
      />
    );
  const isToday = date === today(user.timezone),
    total = diary.totals,
    goal = user.goals,
    latest = weights.at(-1),
    progress = Math.min(100, (total.calories / goal.calories) * 100);
  return (
    <div className="app-shell">
      <aside className={"sidebar " + (mobile ? "open" : "")}>
        <a
          href="#dashboard"
          className="brand"
          onClick={() => setPage("dashboard")}
        >
          <span className="brand-symbol">
            <Leaf size={23} />
          </span>
          myfit<span className="brand-dot">.</span>
        </a>
        <div className="nav-caption">YOUR EVERYDAY</div>
        <nav>
          {nav.map((n) => (
            <button
              key={n.id}
              className={page === n.id ? "active" : ""}
              onClick={() => {
                setPage(n.id);
                setMobile(false);
              }}
            >
              <n.icon size={19} />
              {n.label}
              {page === n.id && <span className="nav-indicator" />}
            </button>
          ))}
        </nav>
        <div className="sidebar-note">
          <span className="note-leaf">
            <Leaf size={24} />
          </span>
          <h3>Progress, at your pace.</h3>
          <p>
            Every mindful choice is
            <br />a step in the right direction.
          </p>
        </div>
        <div className="sidebar-user">
          <div className="avatar">{user.name.slice(0, 1).toUpperCase()}</div>
          <div>
            <strong>{user.name}</strong>
            <small>Personal workspace</small>
          </div>
          <button
            className="icon-btn"
            title="Sign out"
            aria-label="Sign out"
            onClick={async () => {
              try {
                await api("/logout", "POST", {});
                setUser(null);
                setCSRF("");
              } catch (e) {
                setError((e as Error).message);
              }
            }}
          >
            <LogOut size={18} />
          </button>
        </div>
      </aside>
      <div className="main-shell">
        <header className="topbar">
          <button
            className="icon-btn mobile-menu"
            aria-label="Toggle menu"
            onClick={() => setMobile(!mobile)}
          >
            <Menu />
          </button>
          <div className="breadcrumb">
            My workspace <span>/</span>{" "}
            <strong>{nav.find((n) => n.id === page)?.label}</strong>
          </div>
          <div className="topbar-right">
            <span className="status-dot" /> A little better, every day{" "}
            <div className="avatar small-avatar">
              {user.name.slice(0, 1).toUpperCase()}
            </div>
          </div>
        </header>
        <main className="main-content">
          <div className="page-heading">
            <div>
              <span className="eyebrow">
                {page === "dashboard"
                  ? "YOUR DAILY CHECK-IN"
                  : "YOUR WELLBEING, SIMPLIFIED"}
              </span>
              <h1>
                {page === "dashboard"
                  ? `Looking good, ${user.name.split(" ")[0]}.`
                  : page === "diary"
                    ? "Your food diary."
                    : page === "library"
                      ? "Good food, at your fingertips."
                      : page === "weight"
                        ? "See the bigger picture."
                        : "Make it personal."}{" "}
                {page === "dashboard" && <span className="heading-sun">☀</span>}
              </h1>
              <p>
                {page === "dashboard"
                  ? "A little awareness today. A healthier you tomorrow."
                  : page === "diary"
                    ? "Every meal is a chance to check in with yourself."
                    : page === "library"
                      ? "Find a favourite, discover the basics, or add your own."
                      : page === "weight"
                        ? "Small changes add up. Follow your journey over time."
                        : "Set goals that fit your life and your needs."}
              </p>
            </div>
            {["dashboard", "diary"].includes(page) ? (
              <button
                className="btn primary"
                onClick={() => setAdding("breakfast")}
              >
                <Plus size={18} /> Log food
              </button>
            ) : page === "weight" ? (
              <button
                className="btn primary"
                onClick={() => setWeightModal(true)}
              >
                <Plus size={18} /> Record weight
              </button>
            ) : null}
          </div>
          <ErrorBox error={error} />
          {["dashboard", "diary"].includes(page) && (
            <div className="date-row">
              <div className="date-picker">
                <button
                  className="icon-btn"
                  aria-label="Previous day"
                  onClick={() => changeDate(-1)}
                >
                  <ChevronLeft size={17} />
                </button>
                <label>
                  <span>
                    {isToday
                      ? "Today"
                      : new Date(date + "T12:00:00").toLocaleDateString(
                          undefined,
                          { weekday: "short" },
                        )}
                  </span>
                  <input
                    type="date"
                    aria-label="Diary date"
                    value={date}
                    min="1900-01-01"
                    max="2200-12-31"
                    onChange={(e) => {
                      if (e.target.value) setDate(e.target.value);
                    }}
                  />
                </label>
                <button
                  className="icon-btn"
                  aria-label="Next day"
                  onClick={() => changeDate(1)}
                >
                  <ChevronRight size={17} />
                </button>
              </div>
              {!isToday && (
                <button
                  className="text-btn"
                  onClick={() => setDate(today(user.timezone))}
                >
                  Back to today
                </button>
              )}
              <span className="date-note">
                {loading
                  ? "Updating your diary…"
                  : "A fresh perspective, one day at a time."}
              </span>
            </div>
          )}
          {loading && ["dashboard", "diary"].includes(page) ? (
            <div className="loading-card">Loading your nutrition diary…</div>
          ) : (
            <>
              {page === "dashboard" && (
                <>
                  <div className="dashboard-grid">
                    <section className="card calories-card">
                      <div className="card-heading">
                        <h2>Daily nutrition</h2>
                        <span className="badge">
                          {isToday ? "TODAY" : "DAILY TOTAL"}
                        </span>
                      </div>
                      <div className="calories-body">
                        <div
                          className="calorie-ring"
                          style={{
                            background: `conic-gradient(var(--green) ${progress}%, #e7ece5 0)`,
                          }}
                        >
                          <div>
                            <Flame size={20} />
                            <strong>{round(total.calories, 0)}</strong>
                            <span>of {round(goal.calories, 0)} kcal</span>
                          </div>
                        </div>
                        <div className="calorie-detail">
                          <span className="eyebrow">
                            {total.calories > goal.calories
                              ? "ABOVE YOUR GOAL"
                              : "LEFT FOR THE DAY"}
                          </span>
                          <h3>
                            {round(Math.abs(diary.remaining.calories), 0)}{" "}
                            <small>kcal</small>
                          </h3>
                          <p>
                            {total.calories === 0
                              ? "Your next nourishing meal awaits."
                              : total.calories > goal.calories
                                ? "A single day is one part of your journey."
                                : "Making room for what feels good."}
                          </p>
                          <div className="legend">
                            <span />
                            Consumed <span />
                            Remaining
                          </div>
                        </div>
                      </div>
                      <div className="calories-footer">
                        <span>
                          <Utensils size={15} /> {diary.logs.length}{" "}
                          {diary.logs.length === 1 ? "entry" : "entries"} logged
                        </span>
                        <button
                          className="text-btn"
                          onClick={() => setPage("profile")}
                        >
                          Edit goals <ArrowUpRight size={14} />
                        </button>
                      </div>
                    </section>
                    <section className="card macros-card">
                      <div className="card-heading">
                        <h2>Your macros</h2>
                        <span className="muted small">Daily balance</span>
                      </div>
                      {(["protein", "carbs", "fat", "fibre"] as const).map(
                        (k, i) => (
                          <div className={"macro macro-" + i} key={k}>
                            <div>
                              <span>
                                <i />
                                {labels[k]}
                              </span>
                              <span>
                                <strong>{round(total[k])}</strong>
                                <small> / {round(goal[k])} g</small>
                              </span>
                            </div>
                            <div className="progress-track">
                              <div
                                style={{
                                  width: `${goal[k] > 0 ? Math.min(100, (total[k] / goal[k]) * 100) : 0}%`,
                                }}
                              />
                            </div>
                          </div>
                        ),
                      )}
                    </section>
                  </div>
                  <div className="section-heading">
                    <div>
                      <h2>On your plate</h2>
                      <p>A little of everything, a day that adds up.</p>
                    </div>
                    <button
                      className="text-btn"
                      onClick={() => setPage("diary")}
                    >
                      View diary <ArrowUpRight size={17} />
                    </button>
                  </div>
                  <div className="meal-cards">
                    {meals.map((m, i) => {
                      const entries = diary.logs.filter((l) => l.meal === m),
                        cals = diary.meal_totals[m]?.calories || 0;
                      return (
                        <section className="card meal-card" key={m}>
                          <div className={"meal-emoji meal-" + i}>
                            {["☀", "◒", "☾", "♧"][i]}
                          </div>
                          <h3>{m}</h3>
                          <p className="meal-cal">
                            {round(cals, 0)} <span>kcal</span>
                          </p>
                          <p className="meal-description">
                            {entries.length
                              ? entries
                                  .map((l) => l.food_name.split(",")[0])
                                  .join(", ")
                              : [
                                  "Start your morning right",
                                  "Fuel your afternoon",
                                  "Wind down, nourish up",
                                  "A little something extra",
                                ][i]}
                          </p>
                          <button
                            className="meal-add"
                            onClick={() => setAdding(m)}
                          >
                            <Plus size={15} />
                            {entries.length ? "Add more" : "Add food"}
                          </button>
                        </section>
                      );
                    })}
                  </div>
                  <div className="bottom-grid">
                    <section className="card weight-summary">
                      <div>
                        <span className="eyebrow">ONE STEP AT A TIME</span>
                        <h2>Your weight journey</h2>
                        <p>
                          {latest
                            ? `Last recorded ${new Date(latest.day + "T12:00:00").toLocaleDateString(undefined, { month: "short", day: "numeric" })}`
                            : "The first check-in is a great place to start."}
                        </p>
                        <button
                          className="text-btn"
                          onClick={() => setWeightModal(true)}
                        >
                          {latest
                            ? "Record a check-in"
                            : "Record your first weight"}{" "}
                          <ArrowUpRight size={15} />
                        </button>
                      </div>
                      <div className="weight-number">
                        <Scale size={20} />
                        <strong>{latest ? round(latest.kg) : "—"}</strong>
                        <span>kilograms</span>
                      </div>
                    </section>
                    <section className="daily-note">
                      <Leaf size={27} />
                      <div>
                        <span className="eyebrow">A GENTLE REMINDER</span>
                        <h3>Consistency looks different every day.</h3>
                        <p>
                          Focus on showing up for yourself. Small habits build a
                          healthier everyday.
                        </p>
                      </div>
                    </section>
                  </div>
                </>
              )}
              {page === "diary" && (
                <>
                  <div className="diary-summary card">
                    {keys.map((k) => (
                      <div key={k}>
                        <span>{labels[k]}</span>
                        <strong>
                          {round(total[k], k === "calories" ? 0 : 1)}
                          <small>{k === "calories" ? " kcal" : " g"}</small>
                        </strong>
                      </div>
                    ))}
                  </div>
                  <div className="diary-meals">
                    {meals.map((m, i) => {
                      const entries = diary.logs.filter((l) => l.meal === m);
                      return (
                        <section className="card diary-meal" key={m}>
                          <header>
                            <h2>
                              <span className={"meal-emoji meal-" + i}>
                                {["☀", "◒", "☾", "♧"][i]}
                              </span>
                              {m}
                            </h2>
                            <button
                              className="text-btn"
                              onClick={() => setAdding(m)}
                            >
                              <Plus size={16} />
                              Add food
                            </button>
                          </header>
                          {entries.length === 0 ? (
                            <div className="empty-meal">
                              Nothing here yet. Make room for something
                              nourishing.
                            </div>
                          ) : (
                            entries.map((l) => (
                              <div className="entry-row" key={l.id}>
                                <div className="entry-description">
                                  <strong>{l.food_name}</strong>
                                  <small>
                                    {round(l.quantity, 3)} × {l.serving_name} ·{" "}
                                    {round(l.quantity * l.serving_grams)} g
                                    {l.brand ? " · " + l.brand : ""}
                                  </small>
                                  <small>
                                    P {round(l.nutrition.protein)} · C{" "}
                                    {round(l.nutrition.carbs)} · F{" "}
                                    {round(l.nutrition.fat)} · Fibre{" "}
                                    {round(l.nutrition.fibre)} g
                                  </small>
                                </div>
                                <strong className="entry-cal">
                                  {round(l.nutrition.calories, 0)}
                                  <small> kcal</small>
                                </strong>
                                <div className="entry-actions">
                                  <button
                                    className="icon-btn"
                                    aria-label={"Repeat " + l.food_name}
                                    title="Add again"
                                    onClick={() => reuse(l)}
                                  >
                                    <Copy size={16} />
                                  </button>
                                  <button
                                    className="icon-btn"
                                    aria-label={"Edit " + l.food_name}
                                    onClick={() => setEditing(l)}
                                  >
                                    <Pencil size={16} />
                                  </button>
                                  <button
                                    className="icon-btn danger"
                                    aria-label={"Delete " + l.food_name}
                                    onClick={() => remove(l)}
                                  >
                                    <Trash2 size={16} />
                                  </button>
                                </div>
                              </div>
                            ))
                          )}
                        </section>
                      );
                    })}
                  </div>
                </>
              )}
            </>
          )}
          {page === "library" && (
            <FoodLibrary
              onAdd={(f) => {
                setInitialFood(f);
                setAdding("breakfast");
              }}
              notify={setToast}
            />
          )}
          {page === "weight" && (
            <WeightPage
              weights={weights}
              add={() => setWeightModal(true)}
              remove={async (w) => {
                try {
                  await api("/weights/" + w.id, "DELETE", {});
                  await refresh();
                  setToast("Weight entry deleted");
                } catch (e) {
                  setError((e as Error).message);
                }
              }}
            />
          )}
          {page === "profile" && (
            <Profile
              key={user.onboarding_version}
              recalculate={() => setEditOnboarding(true)}
              user={user}
              save={(u) => {
                setUser(u);
                setToast("Your profile and goals are saved");
              }}
            />
          )}
          <footer className="page-footer">
            <span className="footer-logo">
              <Leaf size={14} /> myfit
            </span>
            <span>Built around you. One day at a time.</span>
          </footer>
        </main>
      </div>
      {adding && (
        <FoodLogger
          initialFood={initialFood}
          meal={adding}
          date={date}
          close={() => {
            setAdding(null);
            setInitialFood(null);
          }}
          saved={async () => {
            setAdding(null);
            setInitialFood(null);
            await refresh();
            setToast("Food added to your diary");
          }}
        />
      )}
      {editing && (
        <EditEntry
          entry={editing}
          close={() => setEditing(null)}
          saved={async () => {
            setEditing(null);
            await refresh();
            setToast("Entry updated");
          }}
        />
      )}
      {weightModal && (
        <WeightForm
          date={date}
          close={() => setWeightModal(false)}
          saved={async () => {
            setWeightModal(false);
            await refresh();
            setToast("Weight recorded");
          }}
        />
      )}
      {toast && (
        <div className="toast" role="status">
          <Check size={17} />
          {toast}
        </div>
      )}
    </div>
  );
}
function FoodLogger({
  initialFood,
  meal,
  date,
  close,
  saved,
}: {
  initialFood: Food | null;
  meal: string;
  date: string;
  close: () => void;
  saved: () => void;
}) {
  const [q, setQ] = useState(""),
    [filter, setFilter] = useState(""),
    [foods, setFoods] = useState<Food[]>([]),
    [food, setFood] = useState<Food | null>(initialFood),
    [serving, setServing] = useState(
      initialFood
        ? (
            initialFood.servings.find(
              (s) => s.grams !== 1 && s.grams !== 100,
            ) || initialFood.servings[0]
          ).id
        : 0,
    ),
    [quantity, setQuantity] = useState("1"),
    [selectedMeal, setMeal] = useState(meal),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(true),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    let active = true;
    setLoading(true);
    const t = setTimeout(() => {
      api<Food[]>("/foods?q=" + encodeURIComponent(q) + "&filter=" + filter)
        .then((f) => {
          if (active) {
            setFoods(f);
            setError("");
          }
        })
        .catch((e) => {
          if (active) setError(e.message);
        })
        .finally(() => {
          if (active) setLoading(false);
        });
    }, 180);
    return () => {
      active = false;
      clearTimeout(t);
    };
  }, [q, filter]);
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/logs", "POST", {
        food_id: food?.id,
        serving_id: serving,
        quantity: Number(quantity),
        day: date,
        meal: selectedMeal,
      });
      saved();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      title={food ? "A little more detail" : "What’s on your plate?"}
      close={close}
    >
      <ErrorBox error={error} />
      {!food ? (
        <>
          <div className="search-box">
            <Search size={18} />
            <input
              autoFocus
              aria-label="Search foods"
              placeholder="Search banana, rice, paneer…"
              value={q}
              onChange={(e) => setQ(e.target.value)}
            />
          </div>
          <div className="tabs">
            {[
              ["", "All foods"],
              ["recent", "Recent"],
              ["favourites", "Favourites"],
              ["private", "My foods"],
            ].map(([k, l]) => (
              <button
                className={filter === k ? "selected" : ""}
                key={k}
                onClick={() => setFilter(k)}
              >
                {l}
              </button>
            ))}
          </div>
          <div className="food-results">
            {loading ? (
              <p className="empty">Finding something good…</p>
            ) : foods.length === 0 ? (
              <p className="empty">
                No foods here yet. Add your own in the Food Library.
              </p>
            ) : (
              foods.map((f) => (
                <button
                  className="food-result"
                  key={f.id}
                  onClick={() => {
                    setFood(f);
                    setServing(
                      (
                        f.servings.find(
                          (s) => s.grams !== 1 && s.grams !== 100,
                        ) || f.servings[0]
                      ).id,
                    );
                  }}
                >
                  <span className="food-icon">
                    <Leaf size={18} />
                  </span>
                  <span>
                    <strong>{f.name}</strong>
                    <small>
                      {f.brand ||
                        (f.owner_id
                          ? "Your private food"
                          : "Shared catalogue")}{" "}
                      · {round(f.calories, 0)} kcal / 100 g
                    </small>
                  </span>
                  <Plus size={18} />
                </button>
              ))
            )}
          </div>
        </>
      ) : (
        <form onSubmit={submit}>
          <button
            type="button"
            className="text-btn"
            onClick={() => setFood(null)}
          >
            <ChevronLeft size={16} />
            Back to search
          </button>
          <div className="selected-food">
            <h3>{food.name}</h3>
            <p>
              {food.brand || "Generic food"} · {round(food.calories, 0)} kcal
              per 100 g
            </p>
          </div>
          <div className="two-fields">
            <Field label="Serving size">
              <select
                aria-label="Serving size"
                value={serving}
                onChange={(e) => setServing(Number(e.target.value))}
              >
                {food.servings.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                    {s.grams !== 1 ? ` · ${s.grams} g` : ""}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Quantity">
              <input
                aria-label="Quantity"
                type="number"
                required
                min="0.001"
                max="10000"
                step="any"
                value={quantity}
                onChange={(e) => setQuantity(e.target.value)}
              />
            </Field>
          </div>
          <Field label="Meal">
            <select
              value={selectedMeal}
              onChange={(e) => setMeal(e.target.value)}
            >
              {meals.map((m) => (
                <option key={m}>{m}</option>
              ))}
            </select>
          </Field>
          <p className="source-note">
            {food.approximate ? "Approximate reference. " : ""}
            {food.source}. Values vary by preparation; create a private copy in
            Food Library to use your label.
          </p>
          <button className="btn primary wide" disabled={busy}>
            {busy ? "Adding…" : `Add to ${selectedMeal}`}
            <Plus size={18} />
          </button>
          <p className="small muted">
            Nutrition is calculated and saved by the server.
          </p>
        </form>
      )}
    </Modal>
  );
}
function EditEntry({
  entry,
  close,
  saved,
}: {
  entry: Entry;
  close: () => void;
  saved: () => void;
}) {
  const [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const f = new FormData(e.currentTarget);
    try {
      await api("/logs/" + entry.id, "PUT", {
        quantity: Number(f.get("quantity")),
        day: f.get("day"),
        meal: f.get("meal"),
        version: entry.version,
      });
      saved();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal title="Edit diary entry" close={close}>
      <form onSubmit={submit}>
        <h3>{entry.food_name}</h3>
        <p className="muted">
          {entry.serving_name} · {entry.serving_grams} g per serving
        </p>
        <Field label="Quantity">
          <input
            name="quantity"
            aria-label="Quantity"
            type="number"
            required
            min="0.001"
            max="10000"
            step="any"
            defaultValue={entry.quantity}
          />
        </Field>
        <div className="two-fields">
          <Field label="Date">
            <input name="day" type="date" required defaultValue={entry.day} />
          </Field>
          <Field label="Meal">
            <select name="meal" defaultValue={entry.meal}>
              {meals.map((m) => (
                <option key={m}>{m}</option>
              ))}
            </select>
          </Field>
        </div>
        <p className="small muted">
          Your original nutrition reference is preserved. Totals recalculate
          when you save.
        </p>
        <ErrorBox error={error} />
        <button className="btn primary wide" disabled={busy}>
          {busy ? "Saving…" : "Save changes"}
        </button>
      </form>
    </Modal>
  );
}
function FoodLibrary({
  onAdd,
  notify,
}: {
  onAdd: (f: Food) => void;
  notify: (s: string) => void;
}) {
  const [foods, setFoods] = useState<Food[]>([]),
    [q, setQ] = useState(""),
    [filter, setFilter] = useState(""),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(true),
    [edit, setEdit] = useState<Food | null | undefined>(undefined),
    [refresh, setRefresh] = useState(0);
  useEffect(() => {
    let active = true;
    setLoading(true);
    const t = setTimeout(
      () =>
        api<Food[]>("/foods?q=" + encodeURIComponent(q) + "&filter=" + filter)
          .then((f) => {
            if (active) {
              setFoods(f);
              setError("");
            }
          })
          .catch((e) => {
            if (active) setError(e.message);
          })
          .finally(() => {
            if (active) setLoading(false);
          }),
      180,
    );
    return () => {
      active = false;
      clearTimeout(t);
    };
  }, [q, filter, refresh]);
  async function favourite(f: Food) {
    try {
      await api("/foods/" + f.id + "/favourite", "PUT", {
        enabled: !f.favourite,
      });
      setRefresh((x) => x + 1);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <>
      <div className="library-tools">
        <div className="search-box">
          <Search size={18} />
          <input
            aria-label="Search library"
            placeholder="Find a food or ingredient…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
        </div>
        <button className="btn primary" onClick={() => setEdit(null)}>
          <Plus size={17} />
          Create food
        </button>
      </div>
      <div className="tabs">
        {[
          ["", "All foods"],
          ["favourites", "Favourites"],
          ["recent", "Recently eaten"],
          ["private", "My foods"],
        ].map(([k, l]) => (
          <button
            className={filter === k ? "selected" : ""}
            key={k}
            onClick={() => setFilter(k)}
          >
            {l}
          </button>
        ))}
      </div>
      <ErrorBox error={error} />
      <div className="reference-notice">
        <Leaf size={18} />
        <p>
          Shared values are approximate generic references per 100 g, based on
          USDA SR Legacy and ICMR-NIN. Servings and recipes vary. Make a private
          copy to use packaging values.
        </p>
      </div>
      <div className="library-list card">
        {loading ? (
          <p className="empty">Loading food library…</p>
        ) : foods.length === 0 ? (
          <p className="empty">
            No foods found. Try another search or create your own.
          </p>
        ) : (
          foods.map((f) => (
            <article className="library-row" key={f.id}>
              <button
                className={
                  "icon-btn star " + (f.favourite ? "is-favourite" : "")
                }
                aria-label={
                  (f.favourite ? "Unfavourite " : "Favourite ") + f.name
                }
                onClick={() => favourite(f)}
              >
                <Star size={19} fill={f.favourite ? "currentColor" : "none"} />
              </button>
              <div className="library-food">
                <strong>{f.name}</strong>
                <small>
                  {f.owner_id ? "Private · " : ""}
                  {f.brand || "Generic reference"} ·{" "}
                  {f.approximate ? "Approximate" : "Label values"}
                </small>
                <details>
                  <summary>Nutrition reference & servings</summary>
                  <p>{f.source}</p>
                  <p>
                    {f.servings
                      .map((s) => `${s.name} (${s.grams} g)`)
                      .join(" · ")}
                  </p>
                </details>
              </div>
              <div className="library-macros">
                <strong>
                  {round(f.calories, 0)} <small>kcal</small>
                </strong>
                <small>
                  P {round(f.protein)} · C {round(f.carbs)} · F {round(f.fat)} ·
                  Fibre {round(f.fibre)} g
                </small>
                <small>per 100 g</small>
              </div>
              <div className="library-actions">
                <button
                  className="icon-btn"
                  aria-label={(f.owner_id ? "Edit " : "Copy ") + f.name}
                  title={f.owner_id ? "Edit food" : "Create private copy"}
                  onClick={() =>
                    setEdit(
                      f.owner_id
                        ? f
                        : {
                            ...f,
                            id: 0,
                            owner_id: null,
                            name: f.name + " (my version)",
                            source: "Packaging label / user correction",
                            approximate: false,
                          },
                    )
                  }
                >
                  {f.owner_id ? <Pencil size={17} /> : <Copy size={17} />}
                </button>
                <button
                  className="icon-btn"
                  aria-label={"Log " + f.name}
                  onClick={() => onAdd(f)}
                >
                  <Plus size={19} />
                </button>
              </div>
            </article>
          ))
        )}
      </div>
      {edit !== undefined && (
        <FoodForm
          food={edit}
          close={() => setEdit(undefined)}
          saved={() => {
            setEdit(undefined);
            setRefresh((x) => x + 1);
            notify("Private food saved");
          }}
        />
      )}
    </>
  );
}
function FoodForm({
  food,
  close,
  saved,
}: {
  food: Food | null;
  close: () => void;
  saved: () => void;
}) {
  const [servings, setServings] = useState(
      food?.servings.map((s) => ({ name: s.name, grams: s.grams })) || [
        { name: "Grams", grams: 1 },
        { name: "Portion", grams: 100 },
      ],
    ),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const f = new FormData(e.currentTarget);
    const nutrition = Object.fromEntries(
      keys.map((k) => [k, Number(f.get(k))]),
    );
    try {
      await api(
        food?.id ? "/foods/" + food.id : "/foods",
        food?.id ? "PUT" : "POST",
        {
          name: f.get("name"),
          brand: f.get("brand"),
          source: f.get("source"),
          approximate: f.get("approximate") === "on",
          version: food?.version || 1,
          ...nutrition,
          servings,
        },
      );
      saved();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      title={food?.id ? "Edit your food" : "Create a private food"}
      close={close}
    >
      <form onSubmit={submit}>
        <p className="muted small">
          Only you can see this food. Enter values per 100 grams.
        </p>
        <div className="two-fields">
          <Field label="Food name">
            <input
              name="name"
              required
              maxLength={160}
              defaultValue={food?.name}
            />
          </Field>
          <Field label="Brand (optional)">
            <input name="brand" maxLength={160} defaultValue={food?.brand} />
          </Field>
        </div>
        <div className="nutrition-fields">
          {keys.map((k) => (
            <Field
              key={k}
              label={labels[k] + (k === "calories" ? " (kcal)" : " (g)")}
            >
              <input
                name={k}
                type="number"
                step="any"
                min="0"
                max={k === "calories" ? 1000 : 100}
                required
                defaultValue={food?.[k] ?? 0}
              />
            </Field>
          ))}
        </div>
        <Field label="Source / label notes">
          <input
            name="source"
            maxLength={1000}
            defaultValue={food?.source || "Packaging label"}
          />
        </Field>
        <label className="checkbox-field">
          <input
            type="checkbox"
            name="approximate"
            defaultChecked={food?.approximate ?? false}
          />
          Values are approximate
        </label>
        <h3 className="servings-title">
          Serving sizes{" "}
          <button
            type="button"
            className="text-btn"
            disabled={servings.length >= 20}
            onClick={() => setServings([...servings, { name: "", grams: 100 }])}
          >
            <Plus size={14} />
            Add serving
          </button>
        </h3>
        {servings.map((s, i) => (
          <div className="serving-row" key={i}>
            <input
              aria-label={"Serving " + (i + 1) + " name"}
              placeholder="e.g. One slice"
              required
              maxLength={100}
              value={s.name}
              onChange={(e) =>
                setServings(
                  servings.map((v, j) =>
                    i === j ? { ...v, name: e.target.value } : v,
                  ),
                )
              }
            />
            <input
              aria-label={"Serving " + (i + 1) + " grams"}
              type="number"
              required
              step="any"
              min="0.001"
              max="10000"
              value={s.grams}
              onChange={(e) =>
                setServings(
                  servings.map((v, j) =>
                    i === j ? { ...v, grams: Number(e.target.value) } : v,
                  ),
                )
              }
            />
            <span>g</span>
            <button
              type="button"
              className="icon-btn"
              disabled={servings.length === 1}
              aria-label={"Remove serving " + (i + 1)}
              onClick={() => setServings(servings.filter((_, j) => i !== j))}
            >
              <X size={16} />
            </button>
          </div>
        ))}
        <ErrorBox error={error} />
        <button className="btn primary wide" disabled={busy}>
          {busy ? "Saving…" : "Save private food"}
        </button>
      </form>
    </Modal>
  );
}
function WeightForm({
  date,
  close,
  saved,
}: {
  date: string;
  close: () => void;
  saved: () => void;
}) {
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const f = new FormData(e.currentTarget);
    try {
      await api("/weights", "POST", {
        day: f.get("day"),
        kg: Number(f.get("kg")),
      });
      saved();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal title="A moment to check in" close={close}>
      <p className="muted">Just one part of your bigger picture.</p>
      <form onSubmit={submit}>
        <Field label="Date">
          <input type="date" name="day" required defaultValue={date} />
        </Field>
        <Field label="Body weight (kg)">
          <input
            aria-label="Body weight (kg)"
            name="kg"
            type="number"
            required
            step="0.01"
            min="15"
            max="500"
            placeholder="e.g. 54.5"
            autoFocus
          />
        </Field>
        <p className="small muted">
          Recording the same date updates that day’s weight.
        </p>
        <ErrorBox error={error} />
        <button className="btn primary wide" disabled={busy}>
          {busy ? "Saving…" : "Save weight"}
        </button>
      </form>
    </Modal>
  );
}
function WeightPage({
  weights,
  add,
  remove,
}: {
  weights: Weight[];
  add: () => void;
  remove: (w: Weight) => void;
}) {
  const last = weights.at(-1),
    first = weights[0],
    change = last && first ? last.kg - first.kg : 0;
  const min = Math.min(...weights.map((w) => w.kg)) - 1,
    max = Math.max(...weights.map((w) => w.kg)) + 1;
  const firstDay = first ? Date.parse(first.day) : 0,
    range = last ? Date.parse(last.day) - firstDay : 1;
  const points = weights.map((w) => ({
    x: 40 + (range ? ((Date.parse(w.day) - firstDay) / range) * 720 : 360),
    y: 190 - ((w.kg - min) / (max - min)) * 160,
    w,
  }));
  return (
    <>
      <div className="weight-stats">
        {[
          ["Latest weight", last ? round(last.kg) + " kg" : "—"],
          ["First check-in", first ? round(first.kg) + " kg" : "—"],
          [
            "Change over time",
            last ? (change > 0 ? "+" : "") + round(change) + " kg" : "—",
          ],
        ].map(([l, v]) => (
          <section className="card" key={l}>
            <span className="muted">{l}</span>
            <h2>{v}</h2>
          </section>
        ))}
      </div>
      <section className="card chart-card">
        <div className="card-heading">
          <h2>Your weight over time</h2>
          <span className="badge">KILOGRAMS</span>
        </div>
        {weights.length ? (
          <>
            <svg
              className="weight-chart"
              viewBox="0 0 800 240"
              role="img"
              aria-label={`Weight trend, ${weights.length} entries from ${first.day} to ${last!.day}`}
            >
              <defs>
                <linearGradient id="chartFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#56886a" stopOpacity=".18" />
                  <stop offset="100%" stopColor="#56886a" stopOpacity="0" />
                </linearGradient>
              </defs>
              {[0, 1, 2, 3].map((i) => (
                <g key={i}>
                  <line
                    x1="40"
                    x2="760"
                    y1={30 + i * 53}
                    y2={30 + i * 53}
                    stroke="#e8ece5"
                    strokeDasharray="4 5"
                  />
                  <text x="2" y={34 + i * 53} fontSize="11" fill="#7e877c">
                    {round(max - (i * (max - min)) / 3)}
                  </text>
                </g>
              ))}
              {points.length > 1 && (
                <path
                  d={`M ${points[0].x} 200 L ${points.map((p) => `${p.x} ${p.y}`).join(" L ")} L ${points.at(-1)!.x} 200 Z`}
                  fill="url(#chartFill)"
                />
              )}
              <polyline
                points={points.map((p) => `${p.x},${p.y}`).join(" ")}
                fill="none"
                stroke="#347252"
                strokeWidth="3"
                strokeLinejoin="round"
              />
              {points.map((p) => (
                <circle
                  key={p.w.id}
                  cx={p.x}
                  cy={p.y}
                  r="5"
                  fill="#347252"
                  stroke="white"
                  strokeWidth="2"
                >
                  <title>
                    {p.w.day}: {p.w.kg} kg
                  </title>
                </circle>
              ))}
              <text x="40" y="225" fontSize="12" fill="#7e877c">
                {first.day}
              </text>
              <text
                x="760"
                y="225"
                textAnchor="end"
                fontSize="12"
                fill="#7e877c"
              >
                {last!.day}
              </text>
            </svg>
            <p className="small muted">
              Day-to-day changes are normal. Look at the trend over time.
            </p>
          </>
        ) : (
          <div className="empty">
            <Scale size={32} />
            <h3>Your journey starts here.</h3>
            <p>Record a weight to see your progress over time.</p>
            <button className="btn primary" onClick={add}>
              <Plus size={17} />
              Record weight
            </button>
          </div>
        )}
      </section>
      {weights.length > 0 && (
        <section className="card weight-history">
          <h2>Previous check-ins</h2>
          {[...weights].reverse().map((w) => (
            <div className="weight-row" key={w.id}>
              <span>
                {new Date(w.day + "T12:00:00").toLocaleDateString(undefined, {
                  year: "numeric",
                  month: "short",
                  day: "numeric",
                })}
              </span>
              <strong>{round(w.kg, 2)} kg</strong>
              <button
                className="icon-btn danger"
                aria-label={"Delete weight " + w.day}
                onClick={() => remove(w)}
              >
                <Trash2 size={16} />
              </button>
            </div>
          ))}
        </section>
      )}
    </>
  );
}
function Profile({
  user,
  save,
  recalculate,
}: {
  user: User;
  save: (u: User) => void;
  recalculate: () => void;
}) {
  const [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const f = new FormData(e.currentTarget);
    try {
      const u = await api<User>("/profile", "PUT", {
        name: f.get("name"),
        timezone: f.get("timezone"),
        version: user.onboarding_version,
        goals: Object.fromEntries(keys.map((k) => [k, Number(f.get(k))])),
      });
      save(u);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <form className="profile-form" onSubmit={submit}>
      <section className="card">
        <div className="card-heading">
          <h2>Your personal plan</h2>
          <span className="badge">{user.goals_source}</span>
        </div>
        <p className="muted">
          Update measurements, lifestyle, training or your objective. New
          suggestions are reviewed before your active goals change. Weight
          check-ins do not automatically recalculate goals.
        </p>
        <button type="button" className="btn primary" onClick={recalculate}>
          Update profile & review recommendations
        </button>
      </section>
      <section className="card">
        <h2>The basics</h2>
        <p className="muted">Your daily diary follows your time zone.</p>
        <div className="two-fields">
          <Field label="Your name">
            <input
              name="name"
              required
              maxLength={100}
              defaultValue={user.name}
            />
          </Field>
          <Field label="Email">
            <input value={user.email} readOnly />
          </Field>
        </div>
        <Field label="Time zone">
          <input
            name="timezone"
            list="timezones"
            required
            defaultValue={user.timezone}
          />
          <datalist id="timezones">
            {[
              "Asia/Kolkata",
              "Asia/Dubai",
              "Europe/London",
              "America/New_York",
              "America/Los_Angeles",
              "Australia/Sydney",
              "UTC",
            ].map((z) => (
              <option key={z}>{z}</option>
            ))}
          </datalist>
        </Field>
      </section>
      <section className="card">
        <div className="card-heading">
          <h2>Your daily goals</h2>
          <span className="badge">PERSONAL TO YOU</span>
        </div>
        <p className="muted">
          Active goals: {user.goals_source}. You can edit these manually or
          review a new calculation before changing them.
        </p>
        <div className="goals-fields">
          {keys.map((k) => (
            <Field
              key={k}
              label={labels[k] + (k === "calories" ? " (kcal)" : " (g)")}
            >
              <input
                name={k}
                type="number"
                required
                step="any"
                min={k === "calories" ? 1 : 0}
                max={k === "calories" ? 15000 : 2000}
                defaultValue={user.goals[k]}
              />
            </Field>
          ))}
        </div>
      </section>
      <ErrorBox error={error} />
      <button className="btn primary" disabled={busy}>
        {busy ? "Saving…" : "Save profile & goals"}
        <Check size={17} />
      </button>
    </form>
  );
}
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
