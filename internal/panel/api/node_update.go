package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"mirai/internal/nodeapi"
	"net/http"
	"strconv"
)

type nodeUpdater interface {
	UpdateStatus(context.Context, int64) (nodeapi.UpdateStatus, error)
	RequestUpdate(context.Context, int64) error
}
type nodeUpdateOutput struct{ Body nodeapi.UpdateStatus }

func (h *handlers) nodeUpdateStatus(ctx context.Context, in *nodeIDInput) (*nodeUpdateOutput, error) {
	if _, err := h.d.Store.Q.GetNode(ctx, in.ID); err != nil {
		return nil, mapDomainErr(err)
	}
	runtime, ok := h.d.Nodes.(nodeUpdater)
	if !ok {
		return nil, huma.Error503ServiceUnavailable("Node updater unavailable")
	}
	status, err := runtime.UpdateStatus(ctx, in.ID)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("Node updater unavailable", err)
	}
	return &nodeUpdateOutput{Body: status}, nil
}
func (h *handlers) requestNodeUpdate(ctx context.Context, in *nodeIDInput) (*struct{}, error) {
	if _, err := h.d.Store.Q.GetNode(ctx, in.ID); err != nil {
		return nil, mapDomainErr(err)
	}
	runtime, ok := h.d.Nodes.(nodeUpdater)
	if !ok {
		return nil, huma.Error503ServiceUnavailable("Node updater unavailable")
	}
	if err := runtime.RequestUpdate(ctx, in.ID); err != nil {
		return nil, huma.Error503ServiceUnavailable("Node update request failed", err)
	}
	h.audit(ctx, sessionOf(ctx).AdminID, "node.request_update", "node", strconv.FormatInt(in.ID, 10), nil)
	return nil, nil
}
func (h *handlers) registerNodeUpdates() {
	huma.Register(h.api, huma.Operation{OperationID: "node-update-status", Method: http.MethodGet, Path: "/api/v1/nodes/{id}/update", Summary: "Статус обновления ноды", Tags: []string{"node"}}, h.nodeUpdateStatus)
	huma.Register(h.api, huma.Operation{OperationID: "request-node-update", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodPost, Path: "/api/v1/nodes/{id}/update", Summary: "Обновить ноду", Tags: []string{"node"}, DefaultStatus: http.StatusAccepted}, h.requestNodeUpdate)
}
