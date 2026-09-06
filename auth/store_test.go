package auth

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStoreSetupLoginLogout(t *testing.T) {
	store, err := Open(t.TempDir() + "/noteblog.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Setup("admin", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Setup("other", "correct horse battery staple"); !errors.Is(err, ErrAlreadySetup) {
		t.Fatalf("second setup error = %v, want ErrAlreadySetup", err)
	}
	if _, _, _, err := store.Login("admin", "wrong password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v, want ErrInvalidCredentials", err)
	}
	user, token, expires, err := store.Login("admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || !expires.After(time.Now()) || user.Username != "admin" {
		t.Fatalf("login result = %#v %q %s", user, token, expires)
	}
	current, found, err := store.Current(token)
	if err != nil || !found || current.ID != user.ID {
		t.Fatalf("current = %#v %v %v", current, found, err)
	}
	if err := store.Logout(token); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.Current(token); err != nil || found {
		t.Fatalf("session after logout = found=%v err=%v", found, err)
	}
}

func TestStoreValidationAndHashing(t *testing.T) {
	store, err := Open(t.TempDir() + "/noteblog.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Setup("ab", "long enough password"); !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("short username error = %v", err)
	}
	if _, err := store.Setup("admin", ""); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("empty password error = %v", err)
	}
	if _, err := store.Setup("admin", "short"); err != nil {
		t.Fatal(err)
	}
	var hash string
	if err := store.db.QueryRow(`SELECT password_hash FROM users`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, "short") || !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("stored password hash = %q", hash)
	}
	if _, err := store.db.Exec(`UPDATE sessions SET expires_at = ?`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)); err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
}
