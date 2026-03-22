# gen-router

A small generic HTTP router for Go that binds typed request structs from `net/http` requests and renders tagged response variants.

## What this library does

- register generic handlers with typed input/output structs
- parse request data from:
  - JSON body
  - path params
  - query params
  - headers
  - cookies
- render response variants from tagged output structs
- apply shared output fields to any selected response variant
- use the standard library `http.ServeMux`

## Core model

A handler looks like this:

```go
type Handler[I router.Input, O router.Output] interface {
	Handle(input I) O
	I() I
}
```

The input type defines the route:

```go
func (i MyInput) EndpointPath() string {
	return "POST /customers/{id}"
}
```

The rest of the runtime behavior comes from `gen-router` struct tags.

---

## Input tags

Input tags tell the router where each field should come from.

### Supported sources

#### Request body

```go
gen-router:"in:body"
```

Example:

```go
type CreateCustomerInput struct {
	Body CreateCustomerBody `gen-router:"in:body"`
}
```

Rules:
- only one `in:body` field is allowed per input struct
- body is decoded as JSON
- unknown JSON fields are rejected
- non-pointer body fields are required
- pointer body fields are optional

---

#### Path param

```go
gen-router:"in:path;name:id"
```

Example:

```go
type GetCustomerInput struct {
	CustomerID string `gen-router:"in:path;name:id"`
}
```

For route:

```go
func (i GetCustomerInput) EndpointPath() string {
	return "GET /customers/{id}"
}
```

---

#### Query param

```go
gen-router:"in:query;name:verbose"
```

Example:

```go
type ListCustomersInput struct {
	Verbose *bool `gen-router:"in:query;name:verbose"`
}
```

---

#### Header

```go
gen-router:"in:header;name:X-Auth-Token"
```

Example:

```go
type AuthenticatedInput struct {
	Token string `gen-router:"in:header;name:X-Auth-Token"`
}
```

---

#### Cookie

```go
gen-router:"in:cookie;name:session"
```

Example:

```go
type SessionInput struct {
	SessionID string `gen-router:"in:cookie;name:session"`
}
```

---

### Input field rules

#### Required vs optional

For `path`, `query`, `header`, and `cookie`:
- non-pointer fields are required
- pointer fields are optional

Examples:

```go
UserID string  `gen-router:"in:path;name:id"`       // required
Verbose *bool  `gen-router:"in:query;name:verbose"` // optional
Token   string `gen-router:"in:header;name:X-Token"`// required
```

#### Repeated query/header values into slices

Slice fields are supported for repeated values.

Example:

```go
Tags []string `gen-router:"in:query;name:tag"`
```

For:

```text
/items?tag=one&tag=two
```

The field receives:

```go
[]string{"one", "two"}
```

#### Supported scalar types

The binder currently supports these scalar field types:
- `string`
- `bool`
- `int`, `int8`, `int16`, `int32`, `int64`
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- `float32`, `float64`
- `uuid.UUID`

Pointer versions of those types are also supported.

#### UUID support

`github.com/google/uuid` values are supported in path and query fields.

Example:

```go
import "github.com/google/uuid"

type ResourceInput struct {
	ResourceID uuid.UUID  `gen-router:"in:path;name:id"`
	RequestID  *uuid.UUID `gen-router:"in:query;name:request_id"`
}
```

---

## Output tags

Output tags tell the router how to build the HTTP response.

There are two kinds of output fields:
- variant-specific fields
- shared/root fields

### Variant-specific fields

These fields have `response:XXX` and belong to one status code.

#### Response body variant

```go
gen-router:"response:200;in:body"
```

Example:

```go
Success *CustomerResponse `gen-router:"response:200;in:body"`
BadReq  *ErrorResponse    `gen-router:"response:400;in:body"`
Denied  *ErrorResponse    `gen-router:"response:403;in:body"`
```

#### Response header variant

```go
gen-router:"response:200;in:header;name:X-Request-Id"
```

Example:

```go
RequestID string `gen-router:"response:200;in:header;name:X-Request-Id"`
Success   *Body  `gen-router:"response:200;in:body"`
```

This header is only written if the selected response variant is `200`.

---

### Shared/root output fields

If an output field has an output source like `in:header` but does **not** have `response:XXX`, it is treated as a shared field.

Shared fields are applied to **any selected response variant**.

Example:

```go
type CreateCustomerOutput struct {
	TraceID      string           `gen-router:"in:header;name:X-Trace-Id"`
	RequestID    string           `gen-router:"in:header;name:X-Request-Id"`
	Success      *SuccessBody     `gen-router:"response:200;in:body"`
	BadRequest   *ErrorBody       `gen-router:"response:400;in:body"`
	Unauthorized *ErrorBody       `gen-router:"response:403;in:body"`
}
```

Behavior:
- if `Success` is selected, shared headers are written too
- if `BadRequest` is selected, shared headers are written too
- if only shared fields are set and no response variant is selected, rendering fails

Important:
- shared fields do **not** participate in choosing the response variant
- shared `in:body` fields are not valid, because a body must belong to a concrete response status

---

## How response selection works

The router checks response variants in struct field order.

The first response variant with at least one non-zero variant field wins.

Example:

```go
type Output struct {
	OK        *SuccessBody `gen-router:"response:200;in:body"`
	BadInput  *ErrorBody   `gen-router:"response:400;in:body"`
	Forbidden *ErrorBody   `gen-router:"response:403;in:body"`
}
```

Selection rules:
- if `OK != nil`, status `200` is selected
- otherwise if `BadInput != nil`, status `400` is selected
- otherwise if `Forbidden != nil`, status `403` is selected
- otherwise rendering fails with "no suitable response variant found"

Recommended pattern:
- use pointer body fields for optional variants
- set exactly one response body variant in normal handler code

---

## Full example

```go
package main

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"gen-router/router"
)

type CreateCustomerHandler struct{}

func (h *CreateCustomerHandler) Handle(input CreateCustomerInput) CreateCustomerOutput {
	if strings.TrimSpace(input.Token) == "" {
		return CreateCustomerOutput{
			Unauthorized: &ErrorResponse{Error: "missing token"},
		}
	}
	if strings.TrimSpace(input.Body.Name) == "" {
		return CreateCustomerOutput{
			BadRequest: &ErrorResponse{Error: "name is required"},
		}
	}
	return CreateCustomerOutput{
		TraceID:   "trace-123",
		RequestID: fmt.Sprintf("req-%s", input.CustomerID.String()),
		Success: &CreateCustomerOK{
			Message: "created",
		},
	}
}

func (h *CreateCustomerHandler) I() CreateCustomerInput {
	return CreateCustomerInput{}
}

type CreateCustomerInput struct {
	CustomerID uuid.UUID          `gen-router:"in:path;name:id"`
	Token      string             `gen-router:"in:header;name:X-Auth-Token"`
	Verbose    *bool              `gen-router:"in:query;name:verbose"`
	Body       CreateCustomerBody `gen-router:"in:body"`
}

func (i CreateCustomerInput) EndpointPath() string {
	return "POST /customers/{id}"
}

type CreateCustomerBody struct {
	Name string `json:"name"`
}

type CreateCustomerOutput struct {
	TraceID      string            `gen-router:"in:header;name:X-Trace-Id"`
	RequestID    string            `gen-router:"in:header;name:X-Request-Id"`
	Success      *CreateCustomerOK `gen-router:"response:200;in:body"`
	BadRequest   *ErrorResponse    `gen-router:"response:400;in:body"`
	Unauthorized *ErrorResponse    `gen-router:"response:403;in:body"`
}

type CreateCustomerOK struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	r := router.New()
	_ = router.Register(r, &CreateCustomerHandler{})
}
```

---

## Preserved but currently unused tag keys

These keys are parsed and preserved in tags for future work like OpenAPI generation, but runtime binding/rendering does not currently use them:
- `description:...`
- `schema:...`

Example:

```go
gen-router:"response:200;in:body;description:Customer created;schema:CustomerResponse"
```

---

## Development

```bash
go build ./...
go test ./...
make bench
make bench-save
```

## Notes

This version is intentionally small and reflection-based at compile/setup time so it can evolve toward OpenAPI generation later, or even to handler generation.
