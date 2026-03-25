package tester

import (
	"encoding/json"
	"net/http"
	"strings"
)

func RegisterAPI() {
	http.HandleFunc("/api/test", apiTest)
	http.HandleFunc("/api/test/", apiTestSession)
	http.HandleFunc("/api/screenshot/", apiScreenshot)
}

// POST /api/test -- create session
// GET  /api/test -- list all sessions
func apiTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

	switch r.Method {
	case "GET":
		type summary struct {
			ID         string `json:"session_id"`
			Status     string `json:"status"`
			Total      int    `json:"total"`
			Tested     int    `json:"tested"`
			Alive      int    `json:"alive"`
			WithScreen int    `json:"with_screenshot"`
		}

		list := GetAllSessions()
		items := make([]summary, len(list))
		for i, s := range list {
			s.mu.Lock()
			items[i] = summary{
				ID:         s.ID,
				Status:     s.Status,
				Total:      s.Total,
				Tested:     s.Tested,
				Alive:      s.Alive,
				WithScreen: s.WithScreen,
			}
			s.mu.Unlock()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"sessions": items})

	case "POST":
		var req struct {
			Sources struct {
				Streams []string `json:"streams"`
			} `json:"sources"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if len(req.Sources.Streams) == 0 {
			http.Error(w, "sources.streams required", http.StatusBadRequest)
			return
		}

		s := StartSession(req.Sources.Streams)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"session_id": s.ID})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GET    /api/test/{id} -- session status + results
// DELETE /api/test/{id} -- cancel session
func apiTestSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/test/")
	if id == "" {
		http.Error(w, "session id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		s := GetSession(id)
		if s == nil {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}

		s.mu.Lock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)
		s.mu.Unlock()

	case "DELETE":
		DeleteSession(id)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
