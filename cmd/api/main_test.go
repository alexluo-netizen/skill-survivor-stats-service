package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/healthz",
		nil,
	)

	recorder := httptest.NewRecorder()

	healthHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	var response map[string]string

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf(
			`expected status value "ok", got %q`,
			response["status"],
		)
	}
}

func TestGameRunsHandlerCreatesRun(t *testing.T) {
	store := &GameRunStore{
		runs: make([]GameRun, 0),
	}

	body := strings.NewReader(`{
		"player_id": "player-001",
		"survival_seconds": 185,
		"level": 4,
		"normal_kills": 12,
		"fast_kills": 5,
		"tank_kills": 2,
		"result": "completed"
	}`)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/game-runs",
		body,
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	gameRunsHandler(recorder, request, store)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d; body=%s",
			http.StatusCreated,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response GameRun

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != "1" {
		t.Errorf(`expected ID "1", got %q`, response.ID)
	}

	if response.PlayerID != "player-001" {
		t.Errorf(
			`expected player ID "player-001", got %q`,
			response.PlayerID,
		)
	}

	if len(store.runs) != 1 {
		t.Fatalf(
			"expected store to contain 1 run, got %d",
			len(store.runs),
		)
	}
}

func TestGameRunsHandlerRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing player ID",
			body: `{
				"survival_seconds": 185,
				"level": 4,
				"result": "completed"
			}`,
		},
		{
			name: "negative survival time",
			body: `{
				"player_id": "player-001",
				"survival_seconds": -1,
				"level": 4,
				"result": "completed"
			}`,
		},
		{
			name: "invalid level",
			body: `{
				"player_id": "player-001",
				"survival_seconds": 185,
				"level": 0,
				"result": "completed"
			}`,
		},
		{
			name: "invalid result",
			body: `{
				"player_id": "player-001",
				"survival_seconds": 185,
				"level": 4,
				"result": "unknown"
			}`,
		},
		{
			name: "invalid JSON",
			body: `{
				"player_id":
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &GameRunStore{
				runs: make([]GameRun, 0),
			}

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/game-runs",
				strings.NewReader(test.body),
			)

			recorder := httptest.NewRecorder()

			gameRunsHandler(recorder, request, store)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf(
					"expected status %d, got %d; body=%s",
					http.StatusBadRequest,
					recorder.Code,
					recorder.Body.String(),
				)
			}

			if len(store.runs) != 0 {
				t.Errorf(
					"expected invalid run not to be stored, got %d runs",
					len(store.runs),
				)
			}
		})
	}
}
