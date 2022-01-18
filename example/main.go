package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/buddyspike/gohybrid"
)

func main() {
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("hello\n"))
	})

	http.HandleFunc("/header", func(res http.ResponseWriter, req *http.Request) {
		res.Write([]byte(strings.Join(req.Header.Values("X-Custom"), ",")))
	})

	http.HandleFunc("/json", func(res http.ResponseWriter, req *http.Request) {
		json.NewEncoder(res).Encode(map[string]interface{}{
			"name": "alice cooper",
			"age":  25,
		})
	})

	http.HandleFunc("/error", func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusInternalServerError)
	})

	http.HandleFunc("/panic", func(res http.ResponseWriter, req *http.Request) {
		panic("panic route")
	})

	gohybrid.ListenAndServe(":8080", nil, gohybrid.WithBasePath("api"))
}
