package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"golinkshrink.com/linkshrink/internal/domain/link"
	"golinkshrink.com/linkshrink/internal/inbound/rest/httperr"
)

const defaultPageSize = 20

type LinksHandler struct {
	service link.Service
}

func NewLinksHandler(service link.Service) *LinksHandler {
	return &LinksHandler{service: service}
}

func (h *LinksHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/links", h.create)
	mux.HandleFunc("GET /v1/links", h.list)
	mux.HandleFunc("GET /v1/links/{code}", h.stats)
	mux.HandleFunc("GET /{code}", h.redirect)
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

type listResponse struct {
	Items      []linkResponse `json:"items"`
	NextCursor *string        `json:"next_cursor"`
}

func toResponse(l link.Link) linkResponse {
	return linkResponse{
		ShortCode:  l.ShortCode,
		LongURL:    l.LongURL,
		CreatedAt:  l.CreatedAt,
		ClickCount: l.ClickCount,
	}
}

func (h *LinksHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	l, err := h.service.Create(r.Context(), req.URL)
	if err != nil {
		if errors.Is(err, link.ErrInvalidURL) {
			writeError(w, http.StatusUnprocessableEntity, httperr.CodeInvalidURL,
				fmt.Sprintf("url must be a valid http or https URL of at most %d characters", link.MaxURLLength))
			return
		}
		// Not a client mistake: writeInternal picks between 503, 499 and 500.
		writeInternal(w, r, "create link failed", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toResponse(l))
}

func (h *LinksHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := defaultPageSize
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, httperr.CodeInvalidLimit,
				fmt.Sprintf("limit must be an integer between 1 and %d", link.MaxPageSize))
			return
		}
		limit = n
	}

	var after *link.Cursor
	if v := q.Get("cursor"); v != "" {
		c, err := decodeCursor(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, httperr.CodeInvalidCursor, "cursor is not valid")
			return
		}
		after = c
	}

	page, err := h.service.List(r.Context(), limit, after)
	if err != nil {
		if errors.Is(err, link.ErrInvalidLimit) {
			writeError(w, http.StatusBadRequest, httperr.CodeInvalidLimit,
				fmt.Sprintf("limit must be an integer between 1 and %d", link.MaxPageSize))
			return
		}
		writeInternal(w, r, "list links failed", err)
		return
	}

	resp := listResponse{Items: make([]linkResponse, 0, len(page.Links))}
	for _, l := range page.Links {
		resp.Items = append(resp.Items, toResponse(l))
	}
	if page.Next != nil {
		next := encodeCursor(*page.Next)
		resp.NextCursor = &next
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *LinksHandler) stats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	l, err := h.service.Stats(r.Context(), code)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			writeError(w, http.StatusNotFound, httperr.CodeNotFound, "link not found")
			return
		}
		writeInternal(w, r, "link stats failed", err, "code", code)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toResponse(l))
}

func (h *LinksHandler) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	l, err := h.service.Resolve(r.Context(), code)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			writeError(w, http.StatusNotFound, httperr.CodeNotFound, "link not found")
			return
		}
		writeInternal(w, r, "resolve link failed", err, "code", code)
		return
	}

	http.Redirect(w, r, l.LongURL, http.StatusFound)
}
