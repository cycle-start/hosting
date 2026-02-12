package handler

import (
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type Node struct {
	svc *core.NodeService
}

func NewNode(svc *core.NodeService) *Node {
	return &Node{svc: svc}
}

func (h *Node) ListByCluster(w http.ResponseWriter, r *http.Request) {
	clusterID, err := request.RequireID(chi.URLParam(r, "clusterID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	nodes, hasMore, err := h.svc.ListByCluster(r.Context(), clusterID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(nodes) > 0 {
		nextCursor = nodes[len(nodes)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, nodes, nextCursor, hasMore)
}

func (h *Node) Create(w http.ResponseWriter, r *http.Request) {
	clusterID, err := request.RequireID(chi.URLParam(r, "clusterID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req struct {
		ID         string   `json:"id"`
		Hostname   string   `json:"hostname" validate:"required"`
		IPAddress  *string  `json:"ip_address"`
		IP6Address *string  `json:"ip6_address"`
		ShardID    *string  `json:"shard_id"`
		Roles      []string `json:"roles" validate:"required,min=1"`
	}
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	nodeID := req.ID
	if nodeID == "" {
		nodeID = platform.NewID()
	}

	now := time.Now()
	node := &model.Node{
		ID:         nodeID,
		ClusterID:  clusterID,
		ShardID:    req.ShardID,
		Hostname:   req.Hostname,
		IPAddress:  req.IPAddress,
		IP6Address: req.IP6Address,
		Roles:      req.Roles,
		Status:     model.StatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.svc.Create(r.Context(), node); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusCreated, node)
}

func (h *Node) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	node, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, node)
}

func (h *Node) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req struct {
		Hostname   string   `json:"hostname"`
		IPAddress  *string  `json:"ip_address"`
		IP6Address *string  `json:"ip6_address"`
		ShardID    *string  `json:"shard_id"`
		Roles      []string `json:"roles"`
		Status     string   `json:"status"`
	}
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	node, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Hostname != "" {
		node.Hostname = req.Hostname
	}
	if req.IPAddress != nil {
		node.IPAddress = req.IPAddress
	}
	if req.IP6Address != nil {
		node.IP6Address = req.IP6Address
	}
	if req.ShardID != nil {
		node.ShardID = req.ShardID
	}
	if req.Roles != nil {
		node.Roles = req.Roles
	}
	if req.Status != "" {
		node.Status = req.Status
	}

	if err := h.svc.Update(r.Context(), node); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, node)
}

func (h *Node) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

