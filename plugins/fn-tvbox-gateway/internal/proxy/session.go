package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Session is a short-lived media proxy session.
type Session struct {
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

// Store holds proxy sessions.
type Store struct {
	mu       sync.Mutex
	sessions map[string]Session
	ttl      time.Duration
	client   *http.Client
	listen   string // for building playUrl host
}

// NewStore creates a proxy session store.
func NewStore(client *http.Client, ttl time.Duration, listen string) *Store {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &Store{
		sessions: map[string]Session{},
		ttl:      ttl,
		client:   client,
		listen:   listen,
	}
}

// Create registers a session and returns play URL + expiry.
func (s *Store) Create(mediaURL string, headers map[string]string) (playURL string, expiresAt time.Time, err error) {
	mediaURL = trim(mediaURL)
	if mediaURL == "" {
		return "", time.Time{}, fmt.Errorf("url is required")
	}
	token, err := randomToken(16)
	if err != nil {
		return "", time.Time{}, err
	}
	exp := time.Now().UTC().Add(s.ttl)
	s.mu.Lock()
	s.sessions[token] = Session{URL: mediaURL, Headers: clone(headers), ExpiresAt: exp}
	s.cleanupLocked(time.Now())
	s.mu.Unlock()

	host := s.listen
	if host == "" {
		host = "127.0.0.1:18765"
	}
	playURL = fmt.Sprintf("http://%s/api/proxy/play/%s", host, token)
	return playURL, exp, nil
}

// Get returns a non-expired session.
func (s *Store) Get(token string) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	sess, ok := s.sessions[token]
	if !ok || time.Now().After(sess.ExpiresAt) {
		return Session{}, false
	}
	return sess, true
}

// ServePlay streams upstream media to the client.
func (s *Store) ServePlay(w http.ResponseWriter, r *http.Request, token string) {
	sess, ok := s.Get(token)
	if !ok {
		http.Error(w, `{"error":{"code":"not_found","message":"proxy session expired or missing"}}`, http.StatusNotFound)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, sess.URL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	for k, v := range sess.Headers {
		req.Header.Set(k, v)
	}
	// forward Range for seeking
	if rng := r.Header.Get("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Cache-Control"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (s *Store) cleanupLocked(now time.Time) {
	for k, v := range s.sessions {
		if now.After(v.ExpiresAt) {
			delete(s.sessions, k)
		}
	}
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func clone(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}
