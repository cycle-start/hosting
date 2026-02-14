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

// Search godoc
//
//	@Summary		Search across all resources
//	@Description	Performs a parallel substring search across all resource types (tenants, databases, zones, FQDNs, webroots, etc.). Returns matching resources with their type, ID, and display name. Useful for finding resources when you only have a partial name or identifier.
//	@Tags			Search
//	@Security		ApiKeyAuth
//	@Param			q query string true "Search query (substring match across names and IDs)"
//	@Param			limit query int false "Max results per resource type (1-20)" default(5)
//	@Success		200 {object} searchResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/search [get]
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
