// Package auth provides authentication and session management utilities.
//
// Session keys should be 32 or 64 bytes for HMAC authentication,
// and 16, 24, or 32 bytes for AES encryption. Production deployments
// must use cryptographically random keys generated with:
//
//	openssl rand -base64 32
package auth

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/gob"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "session:"

// RedisStore is a sessions.Store backed by Redis.
// Session data is stored server-side in Redis; only an encrypted session ID
// travels in the client cookie (HttpOnly, Secure in production, SameSite Lax).
//
// Redis keys: "session:<id>" with TTL equal to the session MaxAge.
// Values are gob-encoded; register custom types via gob.Register before use.
type RedisStore struct {
	client  *redis.Client
	codecs  []securecookie.Codec
	options *sessions.Options
}

// NewSessionStore creates a Redis-backed session store.
//
// Parameters:
//   - client: redis.Client instance (from pkg/cache.RedisClient.Client())
//   - authKey: 32 or 64 bytes for HMAC authentication (verifies cookie integrity)
//   - encryptionKey: 16, 24, or 32 bytes for AES encryption (encrypts session ID cookie)
//   - secureCookie: set true in production (HTTPS only); false for localhost dev
//
// Sessions are configured with a 7-day expiration, HttpOnly, and SameSite Lax.
//
// Example:
//
//	store := auth.NewSessionStore(
//	    app.Redis.Client(),
//	    []byte(cfg.SessionAuthKey),
//	    []byte(cfg.SessionEncryptionKey),
//	    cfg.Environment == config.EnvProduction,
//	)
func NewSessionStore(client *redis.Client, authKey, encryptionKey []byte, secureCookie bool) *RedisStore {
	return &RedisStore{
		client: client,
		codecs: securecookie.CodecsFromPairs(authKey, encryptionKey),
		options: &sessions.Options{
			Path:     "/",
			MaxAge:   86400 * 7,            // 7 days
			HttpOnly: true,                 // No JavaScript access (XSS protection)
			Secure:   secureCookie,         // HTTPS only in production
			SameSite: http.SameSiteLaxMode, // CSRF protection, allows top-level navigation
		},
	}
}

// Get returns a session for the given name, loading from Redis if a valid
// session cookie exists.
func (s *RedisStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

// New creates a session. If a valid cookie exists, it decodes the session ID
// and loads data from Redis. A missing/expired/invalid cookie yields a fresh session.
func (s *RedisStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	opts := *s.options
	session.Options = &opts
	session.IsNew = true

	c, err := r.Cookie(name)
	if err != nil {
		return session, nil // no cookie → new session, no error
	}

	var id string
	if err := securecookie.DecodeMulti(name, c.Value, &id, s.codecs...); err != nil {
		return session, nil // invalid/tampered/expired cookie → new session
	}

	session.ID = id
	if err := s.load(r.Context(), session); err != nil {
		return session, nil // Redis key missing or expired → new session
	}
	session.IsNew = false
	return session, nil
}

// Save persists the session to Redis and writes the encrypted session cookie.
// If MaxAge < 0, the session and its Redis key are deleted.
func (s *RedisStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	if session.Options.MaxAge < 0 {
		if session.ID != "" {
			_ = s.client.Del(r.Context(), sessionKeyPrefix+session.ID).Err()
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return nil
	}

	if session.ID == "" {
		session.ID = strings.TrimRight(
			base32.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32)),
			"=",
		)
	}

	if err := s.save(r.Context(), session); err != nil {
		return fmt.Errorf("persist session: %w", err)
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, s.codecs...)
	if err != nil {
		return fmt.Errorf("encode session cookie: %w", err)
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

func (s *RedisStore) save(ctx context.Context, session *sessions.Session) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(session.Values); err != nil {
		return fmt.Errorf("encode session values: %w", err)
	}
	ttl := time.Duration(session.Options.MaxAge) * time.Second
	if err := s.client.Set(ctx, sessionKeyPrefix+session.ID, buf.Bytes(), ttl).Err(); err != nil {
		return fmt.Errorf("set session in redis: %w", err)
	}
	return nil
}

func (s *RedisStore) load(ctx context.Context, session *sessions.Session) error {
	data, err := s.client.Get(ctx, sessionKeyPrefix+session.ID).Bytes()
	if err != nil {
		return fmt.Errorf("get session from redis: %w", err)
	}
	return gob.NewDecoder(bytes.NewBuffer(data)).Decode(&session.Values)
}
