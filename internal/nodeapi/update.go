package nodeapi

import (
	"context"
	"net/http"
	"time"
)

type UpdateStatus struct {
	Status    string `json:"state"`
	Version   string `json:"version"`
	Error     string `json:"error,omitempty"`
	Requested bool   `json:"requested"`
	Supported bool   `json:"supported"`
}

func (c *Client) UpdateStatus(ctx context.Context) (UpdateStatus, error) {
	var out UpdateStatus
	err := c.do(ctx, http.MethodGet, "/v1/update", nil, &out, 10*time.Second)
	return out, err
}
func (c *Client) RequestUpdate(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/v1/update", struct{}{}, nil, 10*time.Second)
}
