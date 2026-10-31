package api

import (
	"errors"
	"myfit/backend/internal/domain"
	"myfit/backend/internal/store"
	"net/http"
	"time"
)

func (s *Server) onboarding(w http.ResponseWriter, r *http.Request) {
	v, e := s.DB.Onboarding(r.Context(), uid(r))
	if e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Server) onboardingProfile(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version int                      `json:"version"`
		Profile domain.OnboardingProfile `json:"profile"`
	}
	if !decode(w, r, &in) {
		return
	}
	now := time.Now().UTC()
	service := domain.RecommendationService{Config: domain.DefaultRecommendationConfig()}
	rec, p, e := service.Calculate(in.Profile, now)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	v, e := s.DB.SaveOnboarding(r.Context(), uid(r), in.Version, p, rec, now)
	if e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Server) onboardingGoals(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version          int              `json:"version"`
		RecommendationID int64            `json:"recommendation_id"`
		Mode             string           `json:"mode"`
		Goals            domain.Nutrition `json:"goals"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Mode != "recommendation" && in.Mode != "custom" && in.Mode != "keep" {
		fail(w, 400, "choose recommended, custom or keep current goals")
		return
	}
	if in.Mode == "custom" && (!in.Goals.Valid(15000, 2000) || in.Goals.Calories < 1) {
		fail(w, 400, "provide valid manual goals")
		return
	}
	u, e := s.DB.ChooseGoals(r.Context(), uid(r), in.Version, in.RecommendationID, in.Mode, in.Goals)
	if errors.Is(e, store.ErrManualRequired) || errors.Is(e, store.ErrOnboardingRequired) {
		fail(w, 422, e.Error())
		return
	}
	if e != nil {
		dbError(w, e)
		return
	}
	write(w, 200, u)
}
