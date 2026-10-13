package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/infrai-examples/game-phone-login/internal/game"
	"github.com/infrai-examples/game-phone-login/internal/infrai"
)

type server struct {
	signup *game.SignupService
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := infrai.New(infrai.DefaultBaseURL, key, nil)
	srv := server{signup: game.NewSignupService(client)}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup/start", srv.startSignup)
	mux.HandleFunc("POST /signup/complete", srv.completeSignup)
	log.Println("game login listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s server) startSignup(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Phone     string `json:"phone"`
		AttemptID string `json:"attempt_id"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Phone == "" || input.AttemptID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone and attempt_id are required"})
		return
	}
	if err := s.signup.Start(r.Context(), input.Phone, input.AttemptID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "code_sent", "attempt_id": input.AttemptID})
}

func (s server) completeSignup(w http.ResponseWriter, r *http.Request) {
	var input game.SignupInput
	if err := decodeJSON(r, &input); err != nil || input.AttemptID == "" || input.Code == "" || input.Player.Phone == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "attempt_id, code, and player.phone are required"})
		return
	}
	result, err := s.signup.Complete(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		status = apiErr.HTTPStatus
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
