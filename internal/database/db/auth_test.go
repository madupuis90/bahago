package db_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"bahago/internal/database/db"
	"bahago/internal/testhelper"
)

func TestCreateUser_Success(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()

	userID, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:  "alice@example.com",
		PwHash: "hashvalue",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if userID <= 0 {
		t.Errorf("CreateUser returned ID %d, want > 0", userID)
	}
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()

	params := db.CreateUserParams{Email: "bob@example.com", PwHash: "h"}
	if _, err := q.CreateUser(ctx, params); err != nil {
		t.Fatalf("first CreateUser: %v", err)
	}
	_, err := q.CreateUser(ctx, params)
	if err == nil {
		t.Fatal("expected error on duplicate email, got nil")
	}
}

func TestGetUserByEmail_Found(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()

	if _, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:  "carol@example.com",
		PwHash: "h",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	user, err := q.GetUserByEmail(ctx, "carol@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if user.Email != "carol@example.com" {
		t.Errorf("Email = %q, want %q", user.Email, "carol@example.com")
	}
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()

	_, err := q.GetUserByEmail(ctx, "nobody@example.com")
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}
	if err != pgx.ErrNoRows {
		t.Errorf("error = %v, want pgx.ErrNoRows", err)
	}
}

func TestCreateSession_Success(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()

	userID, err := q.CreateUser(ctx, db.CreateUserParams{Email: "dave@example.com", PwHash: "h"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	session, err := q.CreateSession(ctx, db.CreateSessionParams{
		ID:        "test-session-id-001",
		UserID:    userID,
		IpAddress: netip.MustParseAddr("127.0.0.1"),
		UserAgent: "go-test",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if session.ID != "test-session-id-001" {
		t.Errorf("session ID = %q, want %q", session.ID, "test-session-id-001")
	}
	if session.UserID != userID {
		t.Errorf("session UserID = %d, want %d", session.UserID, userID)
	}
}

func TestGetUserBySessionID_Valid(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()

	userID, err := q.CreateUser(ctx, db.CreateUserParams{Email: "eve@example.com", PwHash: "h"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := q.CreateSession(ctx, db.CreateSessionParams{
		ID:        "valid-session-002",
		UserID:    userID,
		IpAddress: netip.MustParseAddr("127.0.0.1"),
		UserAgent: "go-test",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	row, err := q.GetUserBySessionID(ctx, "valid-session-002")
	if err != nil {
		t.Fatalf("GetUserBySessionID: %v", err)
	}
	if row.Email != "eve@example.com" {
		t.Errorf("Email = %q, want %q", row.Email, "eve@example.com")
	}
	if row.SessionID != "valid-session-002" {
		t.Errorf("SessionID = %q, want %q", row.SessionID, "valid-session-002")
	}
}

func TestGetUserBySessionID_NotFound(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()

	_, err := q.GetUserBySessionID(ctx, "nonexistent-session-id")
	if err != pgx.ErrNoRows {
		t.Errorf("error = %v, want pgx.ErrNoRows for unknown session", err)
	}
}
