package handler

import (
	"net/http"
	"strconv"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
)

type Search struct {
	svc *core.SearchService
}

func NewSearch(svc *core.SearchService) *Search {
	return &Search{svc: svc}
}

type searchResponse struct {
	Results []core.SearchResult `json:"results"`
}

func (h *Search) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		response.WriteJSON(w, http.StatusOK, searchResponse{Results: []core.SearchResult{}})
		return
	}

	limit := 5
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 20 {
			limit = parsed
		}
	}

	results, err := h.svc.Search(r.Context(), q, limit)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []core.SearchResult{}
	}

	response.WriteJSON(w, http.StatusOK, searchResponse{Results: results})
}
