// Command basic is a runnable demonstration of jsonflex as net/http
// middleware. Start it and send a camelCase request; the handler sees
// snake_case, and the snake_case response comes back as camelCase.
//
//	go run ./examples/basic
//
//	curl -s localhost:8080/users \
//	  -H 'Content-Type: application/json' \
//	  -d '{"userName":"amy","homeAddress":{"zipCode":"10001"}}'
//
//	# handler logs: {"user_name":"amy","home_address":{"zip_code":"10001"}}
//	# curl prints:  {"createdId":1,"userName":"amy","homeAddress":{"zipCode":"10001"}}
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/ikshantshukla123/jsonflex"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", createUser)

	// One line wires up bidirectional conversion for every route.
	handler := jsonflex.Middleware()(mux)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

// createUser works entirely in idiomatic Go snake_case-friendly structs; it
// never sees or emits camelCase.
func createUser(w http.ResponseWriter, r *http.Request) {
	// Body arrives already converted to snake_case, so plain json tags work.
	var in struct {
		UserName    string `json:"user_name"`
		HomeAddress struct {
			ZipCode string `json:"zip_code"`
		} `json:"home_address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("handler received: %+v", in)

	// Respond in snake_case; the middleware converts it back to camelCase.
	resp := map[string]any{
		"created_id":   1,
		"user_name":    in.UserName,
		"home_address": map[string]any{"zip_code": in.HomeAddress.ZipCode},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}
