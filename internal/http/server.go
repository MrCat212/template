package httpserver

import (
	"context"
	"net/http"
	"time"

	api "github.com/MrCat212/template/internal/generated"
	"github.com/MrCat212/template/internal/http/response"
	createhandler "github.com/MrCat212/template/internal/http/trip/create"
	finishhandler "github.com/MrCat212/template/internal/http/trip/finish"
	gethandler "github.com/MrCat212/template/internal/http/trip/get"
)

type readinessChecker interface {
	Ping(ctx context.Context) error
}

type Server struct {
	create  *createhandler.Handler
	get     *gethandler.Handler
	finish  *finishhandler.Handler
	ready   readinessChecker
	timeout time.Duration
}

func New(create *createhandler.Handler, get *gethandler.Handler, finish *finishhandler.Handler, ready readinessChecker, timeout time.Duration) *Server {
	return &Server{create: create, get: get, finish: finish, ready: ready, timeout: timeout}
}

func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	s.create.Handle(w, r, params)
}

func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	s.get.Handle(w, r, id)
}

func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	s.finish.Handle(w, r, id)
}

func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	response.JSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	status := http.StatusOK
	body := api.HealthResponse{Status: api.Ok}
	if err := s.ready.Ping(ctx); err != nil {
		status = http.StatusServiceUnavailable
		body.Status = api.Unavailable
	}
	response.JSON(w, status, body)
}
