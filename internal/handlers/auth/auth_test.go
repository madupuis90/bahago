package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"bahago/internal/database/db"
	"bahago/internal/routes"
	"bahago/internal/testhelper"
)

// ── Stub querier ──────────────────────────────────────────────────────────────

// stubQuerier embeds a nil db.Querier. Any method not explicitly overridden
// panics via a nil pointer dereference, making unexpected DB calls immediately
// visible. Override only the methods a specific test expects to be called.
type stubQuerier struct {
	db.Querier
	onGetUserByEmail              func(ctx context.Context, email string) (db.User, error)
	onCreatePasswordResetToken    func(ctx context.Context, arg db.CreatePasswordResetTokenParams) error
	onDeletePasswordResetTokens   func(ctx context.Context, userID int) error
	onCreateEmailVerification     func(ctx context.Context, arg db.CreateEmailVerificationParams) error
	onDeleteEmailVerificationByID func(ctx context.Context, userID int) error
}

func (s *stubQuerier) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	if s.onGetUserByEmail != nil {
		return s.onGetUserByEmail(ctx, email)
	}
	panic("stubQuerier: unexpected call to GetUserByEmail")
}

func (s *stubQuerier) CreatePasswordResetToken(ctx context.Context, arg db.CreatePasswordResetTokenParams) error {
	if s.onCreatePasswordResetToken != nil {
		return s.onCreatePasswordResetToken(ctx, arg)
	}
	panic("stubQuerier: unexpected call to CreatePasswordResetToken")
}

func (s *stubQuerier) DeletePasswordResetTokensByUserID(ctx context.Context, userID int) error {
	if s.onDeletePasswordResetTokens != nil {
		return s.onDeletePasswordResetTokens(ctx, userID)
	}
	panic("stubQuerier: unexpected call to DeletePasswordResetTokensByUserID")
}

func (s *stubQuerier) CreateEmailVerification(ctx context.Context, arg db.CreateEmailVerificationParams) error {
	if s.onCreateEmailVerification != nil {
		return s.onCreateEmailVerification(ctx, arg)
	}
	panic("stubQuerier: unexpected call to CreateEmailVerification")
}

func (s *stubQuerier) DeleteEmailVerificationByUserID(ctx context.Context, userID int) error {
	if s.onDeleteEmailVerificationByID != nil {
		return s.onDeleteEmailVerificationByID(ctx, userID)
	}
	panic("stubQuerier: unexpected call to DeleteEmailVerificationByUserID")
}

// ── Shared validators ─────────────────────────────────────────────────────────

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr error
	}{
		{"valid", "user@example.com", nil},
		{"empty", "", ErrInvalidEmail},
		{"missing_at", "userexample.com", ErrInvalidEmail},
		{"missing_domain", "user@", ErrInvalidEmail},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEmail(tc.email)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("unexpected err: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"valid_minimum", "12345678", nil},
		{"valid_long", strings.Repeat("a", 72), nil},
		{"too_short", "1234567", ErrPasswordTooShort},
		{"empty", "", ErrPasswordTooShort},
		{"too_long", strings.Repeat("a", 73), ErrPasswordTooLong},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePassword(tc.input)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("unexpected err: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// ── authenticateUser ──────────────────────────────────────────────────────────

func TestAuthenticateUser_UserNotFound(t *testing.T) {
	q := &stubQuerier{
		onGetUserByEmail: func(_ context.Context, _ string) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
	}
	h := &handler{queries: q}
	_, err := h.authenticateUser(context.Background(), "ghost@example.com", "anything")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthenticateUser_WrongPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("real-password"), bcrypt.MinCost)
	q := &stubQuerier{
		onGetUserByEmail: func(_ context.Context, _ string) (db.User, error) {
			return db.User{Email: "u@example.com", PwHash: string(hash), IsVerified: true}, nil
		},
	}
	h := &handler{queries: q}
	_, err := h.authenticateUser(context.Background(), "u@example.com", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthenticateUser_Unverified(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	q := &stubQuerier{
		onGetUserByEmail: func(_ context.Context, _ string) (db.User, error) {
			return db.User{Email: "u@example.com", PwHash: string(hash), IsVerified: false}, nil
		},
	}
	h := &handler{queries: q}
	_, err := h.authenticateUser(context.Background(), "u@example.com", "correct-password")
	if !errors.Is(err, ErrUnverifiedEmail) {
		t.Fatalf("err = %v, want ErrUnverifiedEmail", err)
	}
}

func TestAuthenticateUser_Success(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	q := &stubQuerier{
		onGetUserByEmail: func(_ context.Context, _ string) (db.User, error) {
			return db.User{ID: 42, Email: "u@example.com", PwHash: string(hash), IsVerified: true}, nil
		},
	}
	h := &handler{queries: q}
	user, err := h.authenticateUser(context.Background(), "u@example.com", "correct-password")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if user.ID != 42 {
		t.Errorf("user.ID = %d, want 42", user.ID)
	}
}

// ── validateLoginInput / validateRegisterInput / etc. ─────────────────────────

func TestValidateLoginInput(t *testing.T) {
	tests := []struct {
		name     string
		input    *LoginForm
		wantErrs []error
	}{
		{"valid", &LoginForm{Email: "u@example.com", Password: "12345678"}, nil},
		{"invalid_email", &LoginForm{Email: "bad", Password: "12345678"}, []error{ErrInvalidEmail}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateLoginInput(tc.input)
			if len(got) != len(tc.wantErrs) {
				t.Fatalf("got %d errs (%v), want %d", len(got), got, len(tc.wantErrs))
			}
			for i, want := range tc.wantErrs {
				if !errors.Is(got[i], want) {
					t.Errorf("errs[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

func TestValidateRegisterInput(t *testing.T) {
	tests := []struct {
		name     string
		input    *RegisterForm
		wantErrs []error
	}{
		{"valid", &RegisterForm{Email: "u@example.com", Password: "12345678"}, nil},
		{"both_bad", &RegisterForm{Email: "bad", Password: "short"}, []error{ErrInvalidEmail, ErrPasswordTooShort}},
		{"email_bad", &RegisterForm{Email: "bad", Password: "12345678"}, []error{ErrInvalidEmail}},
		{"password_bad", &RegisterForm{Email: "u@example.com", Password: "short"}, []error{ErrPasswordTooShort}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateRegisterInput(tc.input)
			if len(got) != len(tc.wantErrs) {
				t.Fatalf("got %d errs (%v), want %d", len(got), got, len(tc.wantErrs))
			}
			for i, want := range tc.wantErrs {
				if !errors.Is(got[i], want) {
					t.Errorf("errs[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

func TestValidateResetPasswordInput(t *testing.T) {
	tests := []struct {
		name     string
		input    *ResetPasswordForm
		wantErrs []error
	}{
		{"valid", &ResetPasswordForm{Token: "abc", Password: "12345678"}, nil},
		{"missing_token", &ResetPasswordForm{Token: "", Password: "12345678"}, []error{ErrMissingToken}},
		{"both_bad", &ResetPasswordForm{Token: "", Password: "short"}, []error{ErrMissingToken, ErrPasswordTooShort}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateResetPasswordInput(tc.input)
			if len(got) != len(tc.wantErrs) {
				t.Fatalf("got %d errs (%v), want %d", len(got), got, len(tc.wantErrs))
			}
			for i, want := range tc.wantErrs {
				if !errors.Is(got[i], want) {
					t.Errorf("errs[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

// ── initiatePasswordReset & resendEmailVerification (silent paths) ────────────
//
// Both orchestrators absorb unknown-user errors silently to prevent
// enumeration. They return nil on user-not-found; the create+send workflow
// only runs for known users.

func TestInitiatePasswordReset_UnknownEmail_Silent(t *testing.T) {
	q := &stubQuerier{
		onGetUserByEmail: func(_ context.Context, _ string) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
	}
	h := &handler{queries: q}
	if err := h.initiatePasswordReset(context.Background(), "ghost@example.com"); err != nil {
		t.Fatalf("expected silent nil, got %v", err)
	}
}

func TestResendEmailVerification_UnknownEmail_Silent(t *testing.T) {
	q := &stubQuerier{
		onGetUserByEmail: func(_ context.Context, _ string) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
	}
	h := &handler{queries: q}
	if err := h.resendEmailVerification(context.Background(), "ghost@example.com"); err != nil {
		t.Fatalf("expected silent nil, got %v", err)
	}
}

func TestResendEmailVerification_AlreadyVerified_Silent(t *testing.T) {
	q := &stubQuerier{
		onGetUserByEmail: func(_ context.Context, _ string) (db.User, error) {
			return db.User{ID: 42, Email: "u@example.com", IsVerified: true}, nil
		},
	}
	h := &handler{queries: q}
	if err := h.resendEmailVerification(context.Background(), "u@example.com"); err != nil {
		t.Fatalf("expected silent nil for already-verified user, got %v", err)
	}
}

// ── Handler shell smoke tests ─────────────────────────────────────────────────

func loginHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	RegisterRoutes(cr, q, nil, nil, "")
	return cr.Handlers["POST "+routes.LoginPath]
}

func loginReq(body string) *http.Request {
	return httptest.NewRequest("POST", routes.LoginPath, strings.NewReader(body))
}

func TestHandleLogin_EmptyEmail(t *testing.T) {
	h := loginHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, loginReq(`{"email":"","password":"somepassword"}`))
	testhelper.AssertContains(t, w.Body.String(), "invalid email format")
}

func TestHandleLogin_InvalidEmailFormat(t *testing.T) {
	h := loginHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, loginReq(`{"email":"notanemail","password":"somepassword"}`))
	testhelper.AssertContains(t, w.Body.String(), "invalid email format")
}

func TestHandleLogin_InvalidCredentials(t *testing.T) {
	stub := &stubQuerier{
		onGetUserByEmail: func(_ context.Context, _ string) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
	}
	h := loginHandler(stub)
	w := httptest.NewRecorder()
	h(w, loginReq(`{"email":"test@example.com","password":"wrongpassword"}`))
	testhelper.AssertContains(t, w.Body.String(), "invalid email or password")
}

func TestHandleLogin_UnverifiedEmail(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	stub := &stubQuerier{
		onGetUserByEmail: func(_ context.Context, _ string) (db.User, error) {
			return db.User{
				Email:      "unverified@example.com",
				PwHash:     string(hash),
				IsVerified: false,
			}, nil
		},
	}
	h := loginHandler(stub)
	w := httptest.NewRecorder()
	h(w, loginReq(`{"email":"unverified@example.com","password":"correctpassword"}`))
	testhelper.AssertContains(t, w.Body.String(), "please verify your email before logging in")
}
