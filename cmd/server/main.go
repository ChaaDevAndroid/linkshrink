package main

import (
	"fmt"
	"net/http"

	"golinkshrink.com/linkshrink/internal/inbound/rest/server"
)

func main() {
	handler := server.New()
	fmt.Println("listening on :8080")
	http.ListenAndServe(":8080", handler)
}
