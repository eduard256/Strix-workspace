package tester

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const sessionTTL = 30 * time.Minute
const screenshotTTL = 32 * time.Minute

var sessions = map[string]*Session{}
var sessionsMu sync.Mutex

// StartSession creates new session, launches workers in background
func StartSession(urls []string) *Session {
	id := randID()
	s := NewSession(id, len(urls))

	sessionsMu.Lock()
	sessions[id] = s
	sessionsMu.Unlock()

	go runWorkers(s, urls)

	return s
}

func GetSession(id string) *Session {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	return sessions[id]
}

func GetAllSessions() []*Session {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	list := make([]*Session, 0, len(sessions))
	for _, s := range sessions {
		list = append(list, s)
	}
	return list
}

func DeleteSession(id string) {
	sessionsMu.Lock()
	if s, ok := sessions[id]; ok {
		s.Cancel()
		delete(sessions, id)
	}
	sessionsMu.Unlock()
}

// cleanup removes expired sessions
func init() {
	go func() {
		for {
			time.Sleep(time.Minute)

			sessionsMu.Lock()
			for id, s := range sessions {
				s.mu.Lock()
				expired := s.Status == "done" && time.Since(s.ExpiresAt) > 0
				s.mu.Unlock()
				if expired {
					delete(sessions, id)
				}
			}
			sessionsMu.Unlock()
		}
	}()
}

func randID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
