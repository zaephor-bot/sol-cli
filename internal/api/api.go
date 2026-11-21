// Package api serves the sol-server HTTP API.
//
// Routes:
//
//	GET /v1/events       ?lat=&lon=&date=   full solar-event payload
//	GET /v1/golden-hour  ?lat=&lon=&date=   only the two golden-hour windows
//
// Both routes parse their inputs with events.ParseQuery and derive their
// output from events.Calculate, so they accept exactly the same requests and
// report exactly the same times; the golden-hour route is a projection of the
// events payload, never a second calculation.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/zaephor-bot/sol-cli/internal/events"
)

// Server implements http.Handler for the sol-server API.
type Server struct {
	mux *http.ServeMux
}

// New builds a Server with the API routes registered.
func New() *Server {
	s := &Server{mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /v1/events", s.handleEvents)
	s.mux.HandleFunc("GET /v1/golden-hour", s.handleGoldenHour)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// goldenHourResponse carries only the two golden-hour windows — nothing else
// from the events payload.
type goldenHourResponse struct {
	MorningGoldenHour *events.Window `json:"morning_golden_hour"`
	EveningGoldenHour *events.Window `json:"evening_golden_hour"`
}

// parseInputs is the shared request check: identical validation for both
// routes, with every error naming the offending parameter.
func parseInputs(r *http.Request) (events.Query, error) {
	q := r.URL.Query()
	return events.ParseQuery(q.Get("lat"), q.Get("lon"), q.Get("date"))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	q, err := parseInputs(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, events.Calculate(q))
}

func (s *Server) handleGoldenHour(w http.ResponseWriter, r *http.Request) {
	q, err := parseInputs(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	e := events.Calculate(q)
	writeJSON(w, http.StatusOK, goldenHourResponse{
		MorningGoldenHour: e.MorningGoldenHour,
		EveningGoldenHour: e.EveningGoldenHour,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
