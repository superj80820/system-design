package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
)

type HelloService interface {
	Hello(name string) string
}

type helloService struct{}

func (h helloService) Hello(name string) string {
	return "Hello: " + name
}

type helloRequest struct {
	Name string `json:"name"`
}
type helloResponse struct {
	Reply string `json:"reply"`
	Error string `json:"error,omitempty"`
}

func makeHelloEndpoint(svc HelloService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(helloRequest)
		return helloResponse{Reply: svc.Hello(req.Name)}, nil
	}
}

func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	if err == nil {
		panic("encodeError with nil error")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

func main() {
	var helloSVC helloService
	helloHandler := httptransport.NewServer(
		makeHelloEndpoint(helloSVC),
		decodeHelloRequests,
		encodeHelloResponse,
		httptransport.ServerErrorEncoder(encodeError),
	)

	http.Handle("/hello", helloHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func decodeHelloRequests(ctx context.Context, r *http.Request) (interface{}, error) { // TODO: york can use interface for r?
	var request helloRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, errors.New("decode body error: " + err.Error())
	}
	return request, nil
}

func encodeHelloResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}
