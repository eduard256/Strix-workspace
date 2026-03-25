package camdb

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var defaultPorts = map[string]int{
	"rtsp": 554, "rtsps": 322, "http": 80, "https": 443,
	"rtmp": 1935, "mms": 554, "rtp": 5004,
}

// StreamsHandler returns handler for GET /api/streams?ids=...&ip=...&user=...&pass=...&channel=0
func StreamsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		ids := q.Get("ids")
		if ids == "" {
			http.Error(w, "camdb: ids required", http.StatusBadRequest)
			return
		}

		ip := q.Get("ip")
		if ip == "" {
			http.Error(w, "camdb: ip required", http.StatusBadRequest)
			return
		}

		user := q.Get("user")
		pass := q.Get("pass")
		channel, _ := strconv.Atoi(q.Get("channel"))

		// Step 1. Collect raw streams from all ids
		type raw struct {
			url, protocol string
			port          int
		}

		var raws []raw
		for _, id := range strings.Split(ids, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}

			var rows *sql.Rows
			var err error

			switch {
			case strings.HasPrefix(id, "b:"):
				// ex. b:zosi -- all streams for brand
				brandID := id[2:]
				rows, err = db.Query(
					"SELECT url, protocol, port FROM streams WHERE brand_id = ?", brandID,
				)

			case strings.HasPrefix(id, "m:"):
				// ex. m:zosi:ZG23213M -- streams for specific model
				parts := strings.SplitN(id[2:], ":", 2)
				if len(parts) != 2 {
					http.Error(w, "camdb: invalid model id: "+id, http.StatusBadRequest)
					return
				}
				rows, err = db.Query(
					`SELECT s.url, s.protocol, s.port
					FROM stream_models sm
					JOIN streams s ON s.id = sm.stream_id
					WHERE s.brand_id = ? AND sm.model = ?`,
					parts[0], parts[1],
				)

			case strings.HasPrefix(id, "p:"):
				// ex. p:top-150 -- all preset streams
				presetID := id[2:]
				rows, err = db.Query(
					"SELECT url, protocol, port FROM preset_streams WHERE preset_id = ?", presetID,
				)

			default:
				http.Error(w, "camdb: unknown id prefix: "+id, http.StatusBadRequest)
				return
			}

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			found := false
			for rows.Next() {
				var r raw
				if err = rows.Scan(&r.url, &r.protocol, &r.port); err != nil {
					rows.Close()
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				raws = append(raws, r)
				found = true
			}
			rows.Close()

			if !found {
				http.Error(w, "camdb: not found: "+id, http.StatusNotFound)
				return
			}
		}

		// Step 2. Build full URLs, deduplicate
		seen := map[string]bool{}
		var streams []string

		for _, r := range raws {
			if len(streams) >= 20000 {
				break
			}

			port := r.port
			if port == 0 {
				if p, ok := defaultPorts[r.protocol]; ok {
					port = p
				} else {
					port = 80
				}
			}

			u := buildURL(r.protocol, r.url, ip, port, user, pass, channel)
			if seen[u] {
				continue
			}
			seen[u] = true
			streams = append(streams, u)
		}

		// Step 3. Response
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]any{"streams": streams})
	}
}

// internals

func buildURL(protocol, path, ip string, port int, user, pass string, channel int) string {
	path = replacePlaceholders(path, ip, port, user, pass, channel)

	var auth string
	if user != "" {
		auth = user + ":" + pass + "@"
	}

	host := ip
	if p, ok := defaultPorts[protocol]; !ok || p != port {
		host = ip + ":" + strconv.Itoa(port)
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return protocol + "://" + auth + host + path
}

func replacePlaceholders(s, ip string, port int, user, pass string, channel int) string {
	auth := ""
	if user != "" && pass != "" {
		auth = base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	}

	pairs := []string{
		"[CHANNEL]", strconv.Itoa(channel),
		"[channel]", strconv.Itoa(channel),
		"{CHANNEL}", strconv.Itoa(channel),
		"{channel}", strconv.Itoa(channel),
		"[CHANNEL+1]", strconv.Itoa(channel + 1),
		"[channel+1]", strconv.Itoa(channel + 1),
		"{CHANNEL+1}", strconv.Itoa(channel + 1),
		"{channel+1}", strconv.Itoa(channel + 1),
		"[USERNAME]", user, "[username]", user,
		"[USER]", user, "[user]", user,
		"[PASSWORD]", pass, "[password]", pass,
		"[PASWORD]", pass, "[pasword]", pass,
		"[PASS]", pass, "[pass]", pass,
		"[PWD]", pass, "[pwd]", pass,
		"[WIDTH]", "640", "[width]", "640",
		"[HEIGHT]", "480", "[height]", "480",
		"[IP]", ip, "[ip]", ip,
		"[PORT]", strconv.Itoa(port), "[port]", strconv.Itoa(port),
		"[AUTH]", auth, "[auth]", auth,
		"[TOKEN]", "", "[token]", "",
	}

	r := strings.NewReplacer(pairs...)
	return r.Replace(s)
}

// ValidateID checks if id format is valid
func ValidateID(id string) error {
	switch {
	case strings.HasPrefix(id, "b:"):
		if len(id) < 3 {
			return errors.New("camdb: empty brand id")
		}
	case strings.HasPrefix(id, "m:"):
		if strings.Count(id, ":") < 2 {
			return fmt.Errorf("camdb: invalid model id: %s", id)
		}
	case strings.HasPrefix(id, "p:"):
		if len(id) < 3 {
			return errors.New("camdb: empty preset id")
		}
	default:
		return fmt.Errorf("camdb: unknown prefix: %s", id)
	}
	return nil
}
