package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"golinkshrink.com/linkshrink/internal/domain/link"
)

type LinksHandler struct {
	service link.Service
}

func NewLinksHandler(service link.Service) *LinksHandler {
	return &LinksHandler{service: service}
}

func (h *LinksHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /links", h.create)
	mux.HandleFunc("GET /{code}", h.redirect)
	mux.HandleFunc("GET /links/{code}", h.stats)
}

type createRequest struct {
	URL string `json:"url"`
}

type linkResponse struct {
	ShortCode  string    `json:"short_code"`
	LongURL    string    `json:"long_url"`
	CreatedAt  time.Time `json:"created_at"`
	ClickCount int64     `json:"click_count"`
}

func (h *LinksHandler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // cap the body at 1 MB

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, codeBodyTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, codeInvalidJSON, "request body must be valid JSON")
		return
	}

	l, err := h.service.Create(r.Context(), req.URL)
	if err != nil {
		if errors.Is(err, link.ErrInvalidURL) {
			writeError(w, http.StatusUnprocessableEntity, codeInvalidURL, "url must be a valid http or https URL")
			return
		}
		slog.Error("create link failed", "request_id", requestID(w), "error", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(linkResponse{
		ShortCode: l.ShortCode,
		LongURL:   l.LongURL,
		CreatedAt: l.CreatedAt,
	})
}

func (h *LinksHandler) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	l, err := h.service.Resolve(r.Context(), code)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, "link not found")
			return
		}
		slog.Error("resolve link failed", "request_id", requestID(w), "code", code, "error", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "internal server error")
		return
	}

	http.Redirect(w, r, l.LongURL, http.StatusFound)
}

func (h *LinksHandler) stats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	l, err := h.service.Stats(r.Context(), code)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			writeError(w, http.StatusNotFound, codeNotFound, "link not found")
			return
		}
		slog.Error("link stats failed", "request_id", requestID(w), "code", code, "error", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(linkResponse{
		ShortCode:  l.ShortCode,
		LongURL:    l.LongURL,
		CreatedAt:  l.CreatedAt,
		ClickCount: l.ClickCount,
	})
}
