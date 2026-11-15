package routes

import (
	"net/http"

	"api-handler/handlers"
)

func Mount(mux *http.ServeMux) {
	mux.HandleFunc("/health", handlers.Health)
}
