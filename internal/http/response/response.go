package response

import (
	"encoding/json"
	"net/http"

	api "github.com/MrCat212/template/internal/generated"
)

func JSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}

func Problem(w http.ResponseWriter, status int, value api.Problem) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}
