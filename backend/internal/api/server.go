package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"myfit/backend/internal/domain"
	"myfit/backend/internal/store"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	DB         *store.Store
	Origin     string
	Secure     bool
	CookieName string
	mu         sync.Mutex
	attempts   map[string]bucket
}
type bucket struct {
	Start time.Time
	Count int
}
type authKey struct{}
type identity struct {
	ID   int64
	CSRF string
}

func New(db *store.Store, origin string, secure bool) http.Handler {
	s := &Server{DB: db, Origin: origin, Secure: secure, CookieName: SessionCookieName(secure), attempts: make(map[string]bucket)}
	m := http.NewServeMux()
	m.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "UP"}) })
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if e := db.Pool.Ping(ctx); e != nil {
			fail(w, 503, "database unavailable")
			return
		}
		write(w, 200, map[string]string{"status": "UP", "service": "myfit-api"})
	})
	m.HandleFunc("POST /api/register", s.register)
	m.HandleFunc("POST /api/login", s.login)
	m.Handle("GET /api/onboarding", s.auth(http.HandlerFunc(s.onboarding)))
	m.Handle("PUT /api/onboarding/profile", s.auth(http.HandlerFunc(s.onboardingProfile)))
	m.Handle("POST /api/onboarding/goals", s.auth(http.HandlerFunc(s.onboardingGoals)))
	m.Handle("GET /api/me", s.auth(http.HandlerFunc(s.me)))
	m.Handle("PUT /api/profile", s.auth(http.HandlerFunc(s.profile)))
	m.Handle("POST /api/logout", s.auth(http.HandlerFunc(s.logout)))
	m.Handle("GET /api/foods", s.auth(http.HandlerFunc(s.foods)))
	m.Handle("POST /api/foods", s.auth(http.HandlerFunc(s.saveFood)))
	m.Handle("PUT /api/foods/{id}", s.auth(http.HandlerFunc(s.saveFood)))
	m.Handle("PUT /api/foods/{id}/favourite", s.auth(http.HandlerFunc(s.favourite)))
	m.Handle("GET /api/diary", s.auth(http.HandlerFunc(s.diary)))
	m.Handle("POST /api/logs", s.auth(http.HandlerFunc(s.saveLog)))
	m.Handle("PUT /api/logs/{id}", s.auth(http.HandlerFunc(s.saveLog)))
	m.Handle("DELETE /api/logs/{id}", s.auth(http.HandlerFunc(s.deleteLog)))
	m.Handle("GET /api/weights", s.auth(http.HandlerFunc(s.weights)))
	m.Handle("POST /api/weights", s.auth(http.HandlerFunc(s.weight)))
	m.Handle("DELETE /api/weights/{id}", s.auth(http.HandlerFunc(s.deleteWeight)))
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "endpoint not found") })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		defer func() {
			if x := recover(); x != nil {
				log.Printf("panic: %v", x)
				fail(w, 500, "internal server error")
			}
		}()
		if r.Method != "GET" && r.Method != "HEAD" {
			if r.Header.Get("Origin") != s.Origin {
				fail(w, 403, "untrusted request origin")
				return
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				fail(w, 415, "application/json required")
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		m.ServeHTTP(w, r.WithContext(ctx))
	})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	write(w, status, map[string]any{"error": map[string]string{"message": msg}})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32768)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		fail(w, 400, "invalid request body")
		return false
	}
	if e := d.Decode(&struct{}{}); e != io.EOF {
		fail(w, 400, "invalid request body")
		return false
	}
	return true
}
func dbError(w http.ResponseWriter, e error) {
	var pg *pgconn.PgError
	switch {
	case errors.Is(e, pgx.ErrNoRows):
		fail(w, 404, "record not found")
	case errors.Is(e, store.ErrConflict):
		fail(w, 409, e.Error())
	case errors.As(e, &pg) && pg.Code == "23505":
		fail(w, 409, "record already exists")
	default:
		log.Printf("database: %v", e)
		fail(w, 500, "could not complete request")
	}
}
func uid(r *http.Request) int64 { return r.Context().Value(authKey{}).(identity).ID }
func idPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || id <= 0 {
		fail(w, 400, "invalid record id")
		return 0, false
	}
	return id, true
}
func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie(s.CookieName)
		if e != nil {
			fail(w, 401, "please sign in")
			return
		}
		id, csrf, e := s.DB.Session(r.Context(), hash(c.Value))
		if errors.Is(e, pgx.ErrNoRows) {
			fail(w, 401, "session expired; please sign in")
			return
		}
		if e != nil {
			dbError(w, e)
			return
		}
		if r.Method != "GET" && subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			fail(w, 403, "invalid CSRF token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authKey{}, identity{id, csrf})))
	})
}
func (s *Server) session(w http.ResponseWriter, r *http.Request, id int64) {
	t, csrf := token(), token()
	if e := s.DB.NewSession(r.Context(), hash(t), id, csrf); e != nil {
		dbError(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.CookieName, Value: t, Path: "/", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 3600})
	u, e := s.DB.User(r.Context(), id)
	if e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, map[string]any{"user": u, "csrf": csrf})
}
func (s *Server) rate(w http.ResponseWriter, r *http.Request) bool {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, b := range s.attempts {
		if now.Sub(b.Start) > 15*time.Minute {
			delete(s.attempts, k)
		}
	}
	b := s.attempts[ip]
	if b.Start.IsZero() {
		b.Start = now
	}
	b.Count++
	s.attempts[ip] = b
	if b.Count > 30 {
		w.Header().Set("Retry-After", "900")
		fail(w, 429, "too many attempts; try again in 15 minutes")
		return false
	}
	return true
}
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !s.rate(w, r) {
		return
	}
	var in struct{ Email, Password, Name, Timezone string }
	if !decode(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	if in.Timezone == "" {
		in.Timezone = "Asia/Kolkata"
	}
	a, e := mail.ParseAddress(in.Email)
	_, tzerr := time.LoadLocation(in.Timezone)
	if e != nil || a.Address != in.Email || len(in.Email) > 254 || len(in.Password) < 10 || len(in.Password) > 72 || len(in.Name) < 1 || len(in.Name) > 100 || tzerr != nil {
		fail(w, 400, "provide a name, valid email, timezone and password of 10–72 bytes")
		return
	}
	b, e := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if e != nil {
		dbError(w, e)
		return
	}
	u, e := s.DB.CreateUser(r.Context(), in.Email, string(b), in.Name, in.Timezone)
	if e != nil {
		dbError(w, e)
		return
	}
	s.session(w, r, u.ID)
}

// Public dummy credential, precomputed at cost 12. Avoid expensive package init
// in the healthcheck/memory-stats processes while retaining login timing parity.
var dummyHash = []byte("$2a$12$vnLEbIJpkuMkgkNqKAT91uZsBvevxOdh//vvj9O8o/sjUsOlg/OG.")

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.rate(w, r) {
		return
	}
	var in struct{ Email, Password string }
	if !decode(w, r, &in) {
		return
	}
	id, h, e := s.DB.Credentials(r.Context(), strings.ToLower(strings.TrimSpace(in.Email)))
	if e != nil {
		h = string(dummyHash)
	}
	valid := bcrypt.CompareHashAndPassword([]byte(h), []byte(in.Password)) == nil
	if e != nil || !valid {
		fail(w, 401, "email or password is incorrect")
		return
	}
	if c, err := r.Cookie(s.CookieName); err == nil {
		if err = s.DB.DeleteSession(r.Context(), hash(c.Value)); err != nil {
			dbError(w, err)
			return
		}
	}
	s.session(w, r, id)
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, e := s.DB.User(r.Context(), uid(r))
	if e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, map[string]any{"user": u, "csrf": r.Context().Value(authKey{}).(identity).CSRF})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie(s.CookieName)
	if e := s.DB.DeleteSession(r.Context(), hash(c.Value)); e != nil {
		dbError(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.CookieName, Value: "", Path: "/", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	write(w, 200, map[string]bool{"ok": true})
}
func (s *Server) profile(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string           `json:"name"`
		Timezone string           `json:"timezone"`
		Goals    domain.Nutrition `json:"goals"`
		Version  int              `json:"version"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	_, e := time.LoadLocation(in.Timezone)
	if e != nil || len(in.Name) < 1 || len(in.Name) > 100 || !in.Goals.Valid(15000, 2000) || in.Goals.Calories < 1 {
		fail(w, 400, "invalid name, timezone or goals")
		return
	}
	u, e := s.DB.Profile(r.Context(), uid(r), in.Name, in.Timezone, in.Goals, in.Version)
	if e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, u)
}
func (s *Server) foods(w http.ResponseWriter, r *http.Request) {
	f, e := s.DB.Foods(r.Context(), uid(r), r.URL.Query().Get("q"), r.URL.Query().Get("filter"))
	if e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, f)
}
func (s *Server) saveFood(w http.ResponseWriter, r *http.Request) {
	var f domain.Food
	if !decode(w, r, &f) {
		return
	}
	f.Name = strings.TrimSpace(f.Name)
	if len(f.Name) < 1 || len(f.Name) > 160 || len(f.Brand) > 160 || len(f.Source) > 1000 || !f.Nutrition.Valid(1000, 100) || len(f.Servings) < 1 || len(f.Servings) > 20 {
		fail(w, 400, "invalid food: provide a name, nutrition per 100 g and 1–20 servings")
		return
	}
	for _, v := range f.Servings {
		if len(strings.TrimSpace(v.Name)) == 0 || len(v.Name) > 100 || !finiteRange(v.Grams, .001, 10000) {
			fail(w, 400, "invalid serving")
			return
		}
	}
	f.ID = 0
	if r.Method == "PUT" {
		id, ok := idPath(w, r)
		if !ok {
			return
		}
		existing, e := s.DB.Food(r.Context(), uid(r), id)
		if e != nil {
			dbError(w, e)
			return
		}
		if existing.OwnerID == nil || *existing.OwnerID != uid(r) {
			fail(w, 403, "copy this shared food to make a private version")
			return
		}
		f.ID = id
	}
	if f.Source == "" {
		f.Source = "User entered / packaging label"
	}
	v, e := s.DB.SaveFood(r.Context(), uid(r), f)
	if e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Server) favourite(w http.ResponseWriter, r *http.Request) {
	id, ok := idPath(w, r)
	if !ok {
		return
	}
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if !decode(w, r, &in) {
		return
	}
	if e := s.DB.Favourite(r.Context(), uid(r), id, in.Enabled); e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func validDay(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Year() >= 1900 && t.Year() <= 2200
}

// localDay is the calendar day (YYYY-MM-DD) at instant now in the IANA zone tz.
func localDay(tz string, now time.Time) (string, error) {
	loc, e := time.LoadLocation(tz)
	if e != nil {
		return "", e
	}
	return now.In(loc).Format("2006-01-02"), nil
}
func (s *Server) day(r *http.Request) string {
	d := r.URL.Query().Get("date")
	if d != "" {
		return d
	}
	u, e := s.DB.User(r.Context(), uid(r))
	if e != nil {
		return ""
	}
	d, e = localDay(u.Timezone, time.Now())
	if e != nil {
		return ""
	}
	return d
}
func (s *Server) diary(w http.ResponseWriter, r *http.Request) {
	d := s.day(r)
	if !validDay(d) {
		fail(w, 400, "invalid date (YYYY-MM-DD)")
		return
	}
	logs, tot, e := s.DB.Logs(r.Context(), uid(r), d)
	if e != nil {
		dbError(w, e)
		return
	}
	u, e := s.DB.User(r.Context(), uid(r))
	if e != nil {
		dbError(w, e)
		return
	}
	mealTotals := map[string]domain.Nutrition{}
	for _, l := range logs {
		n := mealTotals[l.Meal]
		n.Add(l.Nutrition)
		mealTotals[l.Meal] = n
	}
	remaining := domain.Nutrition{Calories: u.Goals.Calories - tot.Calories, Protein: u.Goals.Protein - tot.Protein, Carbs: u.Goals.Carbs - tot.Carbs, Fat: u.Goals.Fat - tot.Fat, Fibre: u.Goals.Fibre - tot.Fibre}
	write(w, 200, map[string]any{"date": d, "logs": logs, "totals": tot, "meal_totals": mealTotals, "remaining": remaining})
}
func finiteRange(v, lo, hi float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= lo && v <= hi
}
func (s *Server) saveLog(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FoodID    int64   `json:"food_id"`
		ServingID int64   `json:"serving_id"`
		Quantity  float64 `json:"quantity"`
		Day       string  `json:"day"`
		Meal      string  `json:"meal"`
		Version   int     `json:"version"`
		ReuseID   int64   `json:"reuse_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !validDay(in.Day) || !finiteRange(in.Quantity, .001, 10000) || (in.Meal != "breakfast" && in.Meal != "lunch" && in.Meal != "dinner" && in.Meal != "snacks") {
		fail(w, 400, "invalid date, meal or quantity")
		return
	}
	var l domain.Log
	if r.Method == "PUT" {
		id, ok := idPath(w, r)
		if !ok {
			return
		}
		v, e := s.DB.Log(r.Context(), uid(r), id)
		if e != nil {
			dbError(w, e)
			return
		}
		l = v
		l.Version = in.Version
	} else if in.ReuseID > 0 {
		v, e := s.DB.Log(r.Context(), uid(r), in.ReuseID)
		if e != nil {
			dbError(w, e)
			return
		}
		l = v
		l.ID = 0
		l.Version = 0
	} else {
		f, e := s.DB.Food(r.Context(), uid(r), in.FoodID)
		if e != nil {
			dbError(w, e)
			return
		}
		var serving *domain.Serving
		for i := range f.Servings {
			if f.Servings[i].ID == in.ServingID {
				serving = &f.Servings[i]
				break
			}
		}
		if serving == nil {
			fail(w, 400, "choose a serving belonging to this food")
			return
		}
		l = domain.Log{FoodID: f.ID, ServingName: serving.Name, ServingGrams: serving.Grams, FoodName: f.Name, Brand: f.Brand, Reference: f.Nutrition}
	}
	l.Day = in.Day
	l.Meal = in.Meal
	l.Quantity = in.Quantity
	l.Nutrition = l.Reference.Scale(l.ServingGrams, l.Quantity)
	v, e := s.DB.SaveLog(r.Context(), uid(r), l)
	if e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Server) deleteLog(w http.ResponseWriter, r *http.Request) {
	id, ok := idPath(w, r)
	if !ok {
		return
	}
	var in struct {
		Version int `json:"version"`
	}
	if !decode(w, r, &in) {
		return
	}
	if _, e := s.DB.Log(r.Context(), uid(r), id); e != nil {
		dbError(w, e)
		return
	}
	if e := s.DB.DeleteLog(r.Context(), uid(r), id, in.Version); e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (s *Server) weights(w http.ResponseWriter, r *http.Request) {
	v, e := s.DB.Weights(r.Context(), uid(r))
	if e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Server) weight(w http.ResponseWriter, r *http.Request) {
	var in domain.Weight
	if !decode(w, r, &in) {
		return
	}
	if !validDay(in.Day) || !finiteRange(in.KG, 15, 500) {
		fail(w, 400, "provide a date and weight between 15 and 500 kg")
		return
	}
	v, e := s.DB.SaveWeight(r.Context(), uid(r), in)
	if e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Server) deleteWeight(w http.ResponseWriter, r *http.Request) {
	id, ok := idPath(w, r)
	if !ok {
		return
	}
	if e := s.DB.DeleteWeight(r.Context(), uid(r), id); e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}

// The __Host- prefix forbids Domain cookies and sibling-subdomain cookie injection.
func SessionCookieName(secure bool) string {
	if secure {
		return "__Host-myfit_session"
	}
	return "myfit_session"
}
