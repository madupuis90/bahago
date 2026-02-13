package main

import (
	"fmt"
	"net/http"

	"github.com/mad/bahago/internal/server"
)

func main() {

	app := server.New()

	// start server
	fmt.Println("Server running at http://localhost:8080")
	http.ListenAndServe(":8080", app)
}
