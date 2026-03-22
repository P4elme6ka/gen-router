package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type InputStruct interface {
	EndpointPath() string
}

type OutputStruct interface{}

type Handler[I InputStruct, O OutputStruct] interface {
	Handle(input I) O
	I() I
	O() O
}

type Router struct {
	mux http.ServeMux
}

func Register[I InputStruct, O OutputStruct](r *Router, handler Handler[I, O]) {
	r.mux.Handle(handler.I().EndpointPath(), http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		inp, err := ParseInput(req, handler.I())
		if err != nil {
			http.Error(w, "invalid input", http.StatusBadRequest)
			return
		}
		output := handler.Handle(inp)

		err = WriteOutput(w, output)
		if err != nil {
			http.Error(w, "failed to write output", http.StatusInternalServerError)
			return
		}
	}))
}

func ParseInput[I InputStruct](req *http.Request, _ I) (I, error) {
	var input I
	err := json.NewDecoder(req.Body).Decode(&input)
	if err != nil {
		return input, err
	}
	return input, nil
}

func WriteOutput[O OutputStruct](w http.ResponseWriter, output O) error {
	jsonEncoded, err := json.Marshal(output)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonEncoded)

	return nil
}

func main() {
	router := &Router{}
	Register(router, &CustomerCreateHandler{})

	http.ListenAndServe(":8080", &router.mux)
}

type CustomerCreateHandler struct{}

func (h *CustomerCreateHandler) Handle(input CustomerCreateInput) CustomerCreateOutput {
	return CustomerCreateOutput{
		StatusOkResponse: struct {
			Message string `json:"message"`
		}{
			Message: fmt.Sprintf("Customer with name '%s' created successfully", input.Body.Name),
		},
	}
}

func (h *CustomerCreateHandler) I() CustomerCreateInput {
	return CustomerCreateInput{}
}

func (h *CustomerCreateHandler) O() CustomerCreateOutput {
	return CustomerCreateOutput{}
}

type CustomerCreateInput struct {
	Body struct {
		Name string `json:"name"`
	} `gen-router:"in:body;description:Customer creation payload;schema:CustomerCreateRequest"`
}

func (i CustomerCreateInput) EndpointPath() string {
	return "POST /customer/create"
}

type CustomerCreateOutput struct {
	StatusOkResponse struct {
		Message string `json:"message"`
	} `gen-router:"response:200;description:Customer created successfully;schema:CustomerCreateResponse;in:body"`
}
