package main

import (
	"context"
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

type GameRunStats struct {
	TotalRuns           int64 `json:"total_runs"`
	BestSurvivalSeconds int   `json:"best_survival_seconds"`
	HighestLevel        int   `json:"highest_level"`
	TotalNormalKills    int64 `json:"total_normal_kills"`
	TotalFastKills      int64 `json:"total_fast_kills"`
	TotalTankKills      int64 `json:"total_tank_kills"`
	TotalKills          int64 `json:"total_kills"`
}

type GameRunRepository interface {
	Create(ctx context.Context, run GameRun) (GameRun, error)
	All(ctx context.Context) ([]GameRun, error)
	Stats(ctx context.Context) (GameRunStats, error)
}

type GameRunStore struct {
	mu     sync.RWMutex
	runs   []GameRun
	nextID int
}

func (s *GameRunStore) Create(
	ctx context.Context,
	run GameRun,
) (GameRun, error) {
	if err := ctx.Err(); err != nil {
		return GameRun{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	run.ID = strconv.Itoa(s.nextID)
	s.runs = append(s.runs, run)

	return run, nil
}

func (s *GameRunStore) All(
	ctx context.Context,
) ([]GameRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	copiedRuns := make([]GameRun, len(s.runs))
	copy(copiedRuns, s.runs)

	return copiedRuns, nil
}

func (s *GameRunStore) Stats(ctx context.Context) (GameRunStats, error) {
	if err := ctx.Err(); err != nil {
		return GameRunStats{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := GameRunStats{
		TotalRuns: int64(len(s.runs)),
	}

	for _, run := range s.runs {
		if run.SurvivalSeconds > stats.BestSurvivalSeconds {
			stats.BestSurvivalSeconds = run.SurvivalSeconds
		}

		if run.Level > stats.HighestLevel {
			stats.HighestLevel = run.Level
		}

		stats.TotalNormalKills += int64(run.NormalKills)
		stats.TotalFastKills += int64(run.FastKills)
		stats.TotalTankKills += int64(run.TankKills)
	}

	stats.TotalKills =
		stats.TotalNormalKills +
			stats.TotalFastKills +
			stats.TotalTankKills

	return stats, nil
}

func gameRunStatsHandler(
	store GameRunRepository,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(
				w,
				"method not allowed",
				http.StatusMethodNotAllowed,
			)
			return
		}

		stats, err := store.Stats(r.Context())
		if err != nil {
			slog.Error(
				"could not calculate game statistics",
				"error",
				err,
			)
			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(stats); err != nil {
			slog.Error(
				"could not encode game statistics",
				"error",
				err,
			)
		}
	}
}

func main() {
	config, err := loadConfig()
	if err != nil {
		slog.Error("could not load configuration", "error", err)
		os.Exit(1)
	}

	databaseContext, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	store, err := NewMySQLGameRunStore(
		databaseContext,
		config.MySQLDSN(),
	)
	if err != nil {
		slog.Error("could not connect to MySQL", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", healthHandler)

	mux.HandleFunc("/api/v1/game-runs", func(w http.ResponseWriter, r *http.Request) {
		gameRunsHandler(w, r, store)
	})

	mux.HandleFunc("/api/v1/game-runs/stats", gameRunStatsHandler(store))

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
	store GameRunRepository,
) {
	switch r.Method {
	case http.MethodGet:
		runs, err := store.All(r.Context())
		if err != nil {
			slog.Error(
				"failed to list game runs",
				"error", err,
			)

			writeJSONError(
				w,
				"failed to list game runs",
				http.StatusInternalServerError,
			)
			return
		}

		writeJSON(w, http.StatusOK, runs)
		return

	case http.MethodPost:
	// 继续执行下面原有的 Decode、validation 和 Create 代码。

	default:
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

	createdRun, err := store.Create(
		r.Context(),
		run,
	)

	if err != nil {
		slog.Error(
			"failed to create game run",
			"error", err,
		)

		writeJSONError(
			w,
			"failed to create game run",
			http.StatusInternalServerError,
		)
		return
	}

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
