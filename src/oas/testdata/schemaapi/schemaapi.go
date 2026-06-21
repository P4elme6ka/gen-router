package schemaapi

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Handler struct{}

func (h *Handler) Handle(ctx context.Context, input Input) Output {
	_ = ctx
	_ = input
	return Output{}
}

func (h *Handler) I() Input {
	return Input{}
}

type Input struct {
	ItemID string        `gen-router:"in:path;name:id;description:Item identifier"`
	Body   createRequest `gen-router:"in:body;description:Payload for item creation"`
}

func (Input) EndpointPath() string {
	return "POST /items/{id}"
}

type createRequest struct {
	ID        uuid.UUID         `json:"id"`
	Name      string            `json:"name"`
	Count     *int              `json:"count,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
	Nested    child             `json:"nested"`
	CreatedAt time.Time         `json:"createdAt"`
}

type child struct {
	Enabled bool `json:"enabled"`
}

type Output struct {
	RequestID string          `gen-router:"in:header;name:X-Request-Id;description:Tracing request identifier"`
	OK        *createResponse `gen-router:"response:200;in:body;description:Created item response"`
}

type createResponse struct {
	Message string `json:"message"`
	Nested  *child `json:"nested,omitempty"`
}
