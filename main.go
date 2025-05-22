package main

import (
	"fmt"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	fmt.Println("Server is listening on localhost:8080")
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println("Server error", err)
	}
}
