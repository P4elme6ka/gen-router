package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"gen-router/router"
)

func main() {
	r := router.New()
	if err := router.Register(r, &CustomerCreateHandler{}); err != nil {
		log.Fatal(err)
	}

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

type CustomerCreateHandler struct{}

func (h *CustomerCreateHandler) Handle(input CustomerCreateInput) CustomerCreateOutput {
	if strings.TrimSpace(input.AuthToken) == "" {
		return CustomerCreateOutput{
			Unauthorized: &ErrorResponse{Error: "missing X-Auth-Token header"},
		}
	}
	if strings.TrimSpace(input.CustomerID) == "" {
		return CustomerCreateOutput{
			BadRequest: &ErrorResponse{Error: "missing customer id"},
		}
	}
	name := strings.TrimSpace(input.Body.Name)
	if name == "" {
		return CustomerCreateOutput{
			BadRequest: &ErrorResponse{Error: "name is required"},
		}
	}

	message := fmt.Sprintf("customer %s with name '%s' created successfully", input.CustomerID, name)
	if input.Verbose != nil && *input.Verbose {
		message = message + fmt.Sprintf(" from %s", input.AuthToken)
	}

	return CustomerCreateOutput{
		StatusOK:  &CustomerCreateSuccess{Message: message},
		TraceID:   input.TraceID,
		RequestID: fmt.Sprintf("req-%s", input.CustomerID),
	}
}

func (h *CustomerCreateHandler) I() CustomerCreateInput {
	return CustomerCreateInput{}
}

type CustomerCreateInput struct {
	CustomerID string             `gen-router:"in:path;name:id"`
	AuthToken  string             `gen-router:"in:header;name:X-Auth-Token"`
	TraceID    string             `gen-router:"in:header;name:X-Trace-Id"`
	Verbose    *bool              `gen-router:"in:query;name:verbose"`
	Body       CustomerCreateBody `gen-router:"in:body;description:Customer creation payload;schema:CustomerCreateRequest"`
}

func (i CustomerCreateInput) EndpointPath() string {
	return "POST /customers/{id}"
}

type CustomerCreateBody struct {
	Name string `json:"name"`
}

type CustomerCreateOutput struct {
	RequestID    string                 `gen-router:"in:header;name:X-Request-Id"`
	TraceID      string                 `gen-router:"in:header;name:X-Trace-Id"`
	StatusOK     *CustomerCreateSuccess `gen-router:"response:200;description:Customer created successfully;schema:CustomerCreateResponse;in:body"`
	BadRequest   *ErrorResponse         `gen-router:"response:400;description:Invalid request;schema:ErrorResponse;in:body"`
	Unauthorized *ErrorResponse         `gen-router:"response:403;description:Unauthorized;schema:ErrorResponse;in:body"`
}

type CustomerCreateSuccess struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
