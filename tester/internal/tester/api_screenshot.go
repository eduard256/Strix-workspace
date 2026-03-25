package tester

import (
	"net/http"
	"strconv"
	"strings"
)

// GET /api/screenshot/{session_id}/{index}
func apiScreenshot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/screenshot/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}

	s := GetSession(parts[0])
	if s == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	idx, err := strconv.Atoi(parts[1])
	if err != nil {
		http.Error(w, "bad index", http.StatusBadRequest)
		return
	}

	data := s.GetScreenshot(idx)
	if data == nil {
		http.Error(w, "screenshot not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}
