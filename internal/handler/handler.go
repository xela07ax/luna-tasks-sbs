package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/xela07ax/luna-tasks-sbs/internal/config"
	"github.com/xela07ax/luna-tasks-sbs/internal/docs"
	"github.com/xela07ax/luna-tasks-sbs/internal/domain"
	"github.com/xela07ax/luna-tasks-sbs/internal/infra"
	"github.com/xela07ax/luna-tasks-sbs/internal/service"
	"go.uber.org/zap"
)

type Handler struct {
	svc    *service.Service
	logger *zap.Logger
	cfg    *config.Config
}

func NewHandler(svc *service.Service, logger *zap.Logger, cfg *config.Config) *Handler {
	return &Handler{svc: svc, logger: logger, cfg: cfg}
}

func (h *Handler) InitRoutes() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	if h.cfg.Metrics.Enabled {
		r.Use(infra.MetricsMiddleware)
	}

	r.Use(infra.CORSMiddleware(h.cfg.Server.CorsAllowedOrigins))

	r.Route("/api/v1", func(r chi.Router) {
		// Публичные эндпоинты
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
		r.Get("/docs", h.GetDocs) // Zero Dependency Docs

		// Защищенные эндпоинты
		r.Group(func(r chi.Router) {
			r.Use(infra.AuthMiddleware(h.cfg.Auth.PublicKey))
			r.Use(infra.RateLimitMiddleware(h.cfg.RateLimit.RequestsPerMinute, h.cfg.RateLimit.Burst))

			r.Route("/teams", func(r chi.Router) {
				r.Post("/", h.CreateTeam)
				r.Get("/", h.GetTeams)
				r.Post("/{id}/invite", h.InviteToTeam)
			})

			r.Route("/tasks", func(r chi.Router) {
				r.Post("/", h.CreateTask)
				r.Get("/", h.GetTasks)
				r.Put("/{id}", h.UpdateTask)
				r.Get("/{id}/history", h.GetTaskHistory)
				r.Post("/{id}/comments", h.AddComment)
				r.Get("/{id}/comments", h.GetTaskComments)
			})

			r.Get("/analytics", h.GetAnalytics)
		})
	})

	return r
}

func (h *Handler) GetDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(docs.GetAPIDocs())
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.svc.Register(r.Context(), req.Email, req.Username, req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tokens, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	json.NewEncoder(w).Encode(tokens)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	tokens, err := h.svc.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	json.NewEncoder(w).Encode(tokens)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	_ = h.svc.Logout(r.Context(), req.RefreshToken)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	team, err := h.svc.CreateTeam(r.Context(), req.Name, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(team)
}

func (h *Handler) GetTeams(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)
	teams, err := h.svc.GetUserTeams(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(teams)
}

func (h *Handler) InviteToTeam(w http.ResponseWriter, r *http.Request) {
	inviterID := r.Context().Value("user_id").(int64)
	teamID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var req struct {
		UserID int64 `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.svc.InviteToTeam(r.Context(), teamID, inviterID, req.UserID); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)
	var task domain.Task
	json.NewDecoder(r.Body).Decode(&task)

	if err := h.svc.CreateTask(r.Context(), &task, userID); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	json.NewEncoder(w).Encode(task)
}

func (h *Handler) GetTasks(w http.ResponseWriter, r *http.Request) {
	teamID, _ := strconv.ParseInt(r.URL.Query().Get("team_id"), 10, 64)
	status := r.URL.Query().Get("status")
	var assigneeID *int64
	if a := r.URL.Query().Get("assignee_id"); a != "" {
		id, _ := strconv.ParseInt(a, 10, 64)
		assigneeID = &id
	}

	limit := 10
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	tasks, err := h.svc.GetTasks(r.Context(), teamID, status, assigneeID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(tasks)
}

func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)
	taskID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var req struct {
		TeamID int64  `json:"team_id"`
		Status string `json:"status"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.svc.UpdateTask(r.Context(), taskID, req.TeamID, req.Status, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetTaskHistory(w http.ResponseWriter, r *http.Request) {
	taskID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	history, err := h.svc.GetTaskHistory(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(history)
}

func (h *Handler) AddComment(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)
	taskID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	comment, err := h.svc.AddComment(r.Context(), taskID, userID, req.Content)
	if err != nil {
		if err.Error() == "task not found" || err.Error() == "user is not a member of the team" {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(comment)
}

func (h *Handler) GetTaskComments(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)
	taskID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	comments, err := h.svc.GetTaskComments(r.Context(), taskID, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	json.NewEncoder(w).Encode(comments)
}

func (h *Handler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetAnalytics(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(stats)
}
