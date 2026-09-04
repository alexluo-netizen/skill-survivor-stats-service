package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
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

type GameRunStore struct {
	mu     sync.RWMutex
	runs   []GameRun
	nextID int
}

func (s *GameRunStore) Create(run GameRun) GameRun {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	run.ID = strconv.Itoa(s.nextID)
	s.runs = append(s.runs, run)

	return run
}

func main() {
	store := &GameRunStore{
		runs: make([]GameRun, 0),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", healthHandler)

	mux.HandleFunc("/api/v1/game-runs", func(w http.ResponseWriter, r *http.Request) {
		gameRunsHandler(w, r, store)
	})

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
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func gameRunsHandler(
	w http.ResponseWriter,
	r *http.Request,
	store *GameRunStore,
) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var run GameRun

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&run); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(run.PlayerID) == "" {
		writeJSONError(w, "player_id is required", http.StatusBadRequest)
		return
	}

	if run.SurvivalSeconds < 0 {
		writeJSONError(w, "survival_seconds cannot be negative", http.StatusBadRequest)
		return
	}

	if run.Level < 1 {
		writeJSONError(w, "level must be at least 1", http.StatusBadRequest)
		return
	}

	if run.Result != "completed" && run.Result != "defeated" {
		writeJSONError(
			w,
			"result must be completed or defeated",
			http.StatusBadRequest,
		)
		return
	}

	createdRun := store.Create(run)

	slog.Info(
		"game run created",
		"id", createdRun.ID,
		"player_id", createdRun.PlayerID,
		"level", createdRun.Level,
	)

	writeJSON(w, http.StatusCreated, createdRun)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

func writeJSONError(w http.ResponseWriter, message string, status int) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}
