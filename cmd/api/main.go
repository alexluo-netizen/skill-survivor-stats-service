package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type GameRun struct {
	ID              string `json:"id"`
	PlayerID        string `json:"player_id"`
	SurvivalSeconds int    `json:"survival_seconds"`
	Level           int    `json:"level"`
	NormalKills     int    `json:"normal_kills"`
	FastKills       int    `json:"fast_kills"`
	TankKills       int    `json:"tank_kills"`
	Result          string `json:"result"`
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", healthHandler)

	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("starting HTTP server", "address", server.Addr)

	if err := server.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{
		"status": "ok",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode health response", "error", err)
	}
}
