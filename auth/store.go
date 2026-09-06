package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	_ "modernc.org/sqlite"
)

var (
	ErrAlreadySetup       = errors.New("administrator is already configured")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidUsername    = errors.New("invalid username")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrInvalidSession     = errors.New("invalid session")
)

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	db *sql.DB
}

const (
	sessionLifetime = 7 * 24 * time.Hour
	argonMemory     = 64 * 1024
	argonIterations = 3
	argonThreads    = 2
	argonSaltSize   = 16
	argonKeySize    = 32
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,64}$`)

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Ready() error {
	if s == nil || s.db == nil {
		return errors.New("auth store is not initialized")
	}
	return s.db.Ping()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, ?)`, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) Setup(username, password string) (User, error) {
	if !usernamePattern.MatchString(username) {
		return User{}, ErrInvalidUsername
	}
	if len([]rune(password)) < 12 {
		return User{}, ErrInvalidPassword
	}
	hash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return User{}, err
	}
	if count != 0 {
		return User{}, ErrAlreadySetup
	}
	now := time.Now().UTC()
	id, err := randomToken(16)
	if err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(`INSERT INTO users(id, username, password_hash, created_at) VALUES (?, ?, ?, ?)`, id, username, hash, now.Format(time.RFC3339Nano)); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return User{ID: id, Username: username, CreatedAt: now}, nil
}

func (s *Store) Login(username, password string) (User, string, time.Time, error) {
	var user User
	var hash string
	var created string
	err := s.db.QueryRow(`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`, username).Scan(&user.ID, &user.Username, &hash, &created)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, "", time.Time{}, ErrInvalidCredentials
		}
		return User{}, "", time.Time{}, err
	}
	valid, err := verifyPassword(password, hash)
	if err != nil {
		return User{}, "", time.Time{}, err
	}
	if !valid {
		return User{}, "", time.Time{}, ErrInvalidCredentials
	}
	user.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return User{}, "", time.Time{}, err
	}
	token, err := randomToken(32)
	if err != nil {
		return User{}, "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(sessionLifetime)
	if _, err := s.db.Exec(`INSERT INTO sessions(token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, hashToken(token), user.ID, expires.Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return User{}, "", time.Time{}, err
	}
	return user, token, expires, nil
}

func (s *Store) Current(token string) (User, bool, error) {
	if strings.TrimSpace(token) == "" {
		return User{}, false, nil
	}
	var user User
	var created, expires string
	err := s.db.QueryRow(`SELECT u.id, u.username, u.created_at, sess.expires_at FROM sessions sess JOIN users u ON u.id = sess.user_id WHERE sess.token_hash = ?`, hashToken(token)).Scan(&user.ID, &user.Username, &created, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return User{}, false, err
	}
	if !expiresAt.After(time.Now().UTC()) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
		return User{}, false, nil
	}
	user.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return User{}, false, err
	}
	return user, true, nil
}

func (s *Store) Logout(token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

func hashPassword(password string) (string, error) {
	salt, err := randomBytes(argonSaltSize)
	if err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonThreads, argonKeySize)
	encoded := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonIterations, argonThreads, encoded.EncodeToString(salt), encoded.EncodeToString(key)), nil
}

func verifyPassword(password, encodedHash string) (bool, error) {
	var memory, iterations, threads uint32
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, errors.New("invalid password hash")
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false, errors.New("invalid password parameters")
	}
	if _, err := fmt.Sscanf(params[0], "m=%d", &memory); err != nil {
		return false, err
	}
	if _, err := fmt.Sscanf(params[1], "t=%d", &iterations); err != nil {
		return false, err
	}
	if _, err := fmt.Sscanf(params[2], "p=%d", &threads); err != nil {
		return false, err
	}
	encoded := base64.RawStdEncoding
	salt, err := encoded.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := encoded.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(threads), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func randomToken(size int) (string, error) {
	b, err := randomBytes(size)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomBytes(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	return b, err
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
