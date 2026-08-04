package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/denysdovzhenko/url-shortener/internal/repository"
	"github.com/denysdovzhenko/url-shortener/internal/service"
)

type URLHandler struct {
	svc    *service.ShortenerService
	logger *slog.Logger
}

func NewURLHandler(svc *service.ShortenerService, logger *slog.Logger) *URLHandler {
	return &URLHandler{svc: svc, logger: logger}
}

func (h *URLHandler) Routes(r chi.Router) {
	r.Get("/health", h.Health)
	r.Post("/api/v1/shorten", h.Shorten)
	r.Get("/api/v1/links/{code}", h.Lookup)
	r.Get("/{code}", h.Redirect)
}

type shortenRequest struct {
	URL       string `json:"url"`
	TTLSecond *int64 `json:"ttl_seconds,omitempty"`
}

type shortenResponse struct {
	ShortCode string     `json:"short_code"`
	ShortURL  string     `json:"short_url"`
	LongURL   string     `json:"long_url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (h *URLHandler) Shorten(w http.ResponseWriter, r *http.Request) {
	var req shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var ttl *time.Duration
	if req.TTLSecond != nil {
		d := time.Duration(*req.TTLSecond) * time.Second
		ttl = &d
	}

	result, err := h.svc.Shorten(r.Context(), req.URL, ttl)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, shortenResponse{
		ShortCode: result.ShortCode,
		ShortURL:  result.ShortURL,
		LongURL:   result.LongURL,
		ExpiresAt: result.ExpiresAt,
	})
}

type linkResponse struct {
	ShortCode  string     `json:"short_code"`
	ShortURL   string     `json:"short_url"`
	LongURL    string     `json:"long_url"`
	ClickCount int64      `json:"click_count"`
	Expired    bool       `json:"expired"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Lookup reports where a short code points without following it, so a link
// can be inspected (or debugged with curl) without burning a click.
func (h *URLHandler) Lookup(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	details, err := h.svc.Lookup(r.Context(), code)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, linkResponse{
		ShortCode:  details.ShortCode,
		ShortURL:   details.ShortURL,
		LongURL:    details.LongURL,
		ClickCount: details.ClickCount,
		Expired:    details.Expired,
		ExpiresAt:  details.ExpiresAt,
		CreatedAt:  details.CreatedAt,
	})
}

func (h *URLHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	longURL, err := h.svc.Resolve(r.Context(), code)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
}

func (h *URLHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *URLHandler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidURL):
		writeError(w, http.StatusBadRequest, "url must be a valid absolute http(s) url")
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, http.StatusNotFound, "short link not found")
	case errors.Is(err, service.ErrLinkExpired):
		writeError(w, http.StatusGone, "short link has expired")
	default:
		h.logger.Error("unhandled service error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
