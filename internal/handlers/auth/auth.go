package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"

	"github.com/jackc/pgx/v5/pgxpool"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"bahago/internal/database/db"
	"bahago/internal/email"
	. "bahago/internal/ui"
	"bahago/internal/router"
	"bahago/internal/routes"
)

/*
Send reset link:
Error: PatchElementsNoTargetsFound
More info: https://data-star.dev/errors/patch_elements_no_targets_found?metadata=%7B%22plugin%22%3A%7B%22type%22%3A%22watcher%22%2C%22name%22%3A%22datastar-patch-elements%22%7D%2C%22element%22%3A%7B%7D%7D
*/

// Query parameter names used across the auth flow.
const (
	tokenParam    = "token"
	verifiedParam = "verified"
	resetParam    = "reset"
)

// ── Routing & handler setup ─────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier, pool *pgxpool.Pool, sender *email.Sender, appURL string) {
	h := newHandler(queries, pool, sender, appURL)
	r.HandleFunc("GET "+routes.LoginPath, h.loginPage())
	r.HandleFunc("GET "+routes.RegisterPath, h.registerPage())
	r.HandleFunc("GET "+routes.VerifyPath, h.verify())
	r.HandleFunc("GET "+routes.ForgotPasswordPath, h.forgotPasswordPage())
	r.HandleFunc("GET "+routes.ResetPasswordPath, h.resetPasswordPage())
	r.HandleFunc("POST "+routes.LoginPath, h.login())
	r.HandleFunc("POST "+routes.RegisterPath, h.register())
	r.HandleFunc("POST "+routes.LogoutPath, h.logout())
	r.HandleFunc("POST "+routes.ForgotPasswordPath, h.forgotPassword())
	r.HandleFunc("POST "+routes.ResetPasswordPath, h.resetPassword())
	r.HandleFunc("POST "+routes.ResendVerificationPath, h.resendVerification())
}

type handler struct {
	queries db.Querier
	pool    *pgxpool.Pool
	sender  *email.Sender
	appURL  string
}

func newHandler(queries db.Querier, pool *pgxpool.Pool, sender *email.Sender, appURL string) *handler {
	return &handler{
		queries: queries,
		pool:    pool,
		sender:  sender,
		appURL:  appURL,
	}
}

// ── Shared templates & helpers ──────────────────────────────────────

func authAlert(inner Node) Node { return AlertContainer("auth-alert", inner) }

func authCrest() Node {
	return Raw(`<svg class="crest crest-lg" width="54" height="62" viewBox="0 0 20 23" aria-hidden="true"><g class="crest-frame"><path class="crest-shield" d="M2 2 L18 2 L18 11 C18 17 14 21 10 22 C6 21 2 17 2 11 Z" stroke="currentColor" stroke-width="0.9" stroke-linejoin="round"/><path d="M3.5 3.5 L16.5 3.5 L16.5 10.8 C16.5 16 13 19.5 10 20.4 C7 19.5 3.5 16 3.5 10.8 Z" fill="none" stroke="currentColor" stroke-width="0.35" stroke-linejoin="round" opacity="0.5"/></g><use class="crest-glyph" href="#g-crown"/></svg>`)
}

func invalidTokenContent() Node {
	return Div(Class("auth-wrap"),
		authCrest(),
		P(Class("auth-wordmark"), Text("Bahago")),
		Hr(Class("auth-divider")),
		Div(Class("auth-body"),
			P(Class("auth-instruct"), Text("This verification link is invalid or has expired.")),
			A(Class("auth-quiet"), Href(routes.RegisterPath), Text("← Back to Register")),
		),
	)
}

func verificationEmail(verifyURL string) Node {
	return HTML(
		Head(
			Meta(Charset("utf-8")),
		),
		Body(
			H1(Text("Verify your email address")),
			P(Text("Thanks for signing up! Click the link below to verify your email address. The link expires in 24 hours.")),
			P(
				A(Href(verifyURL), Text("Verify my email")),
			),
			P(Text("If you didn't create an account, you can safely ignore this email.")),
		),
	)
}

func resetPasswordEmail(resetURL string) Node {
	return HTML(
		Head(
			Meta(Charset("utf-8")),
		),
		Body(
			H1(Text("Reset your password")),
			P(Text("We received a request to reset your password. Click the link below to choose a new one. The link expires in 1 hour.")),
			P(
				A(Href(resetURL), Text("Reset my password")),
			),
			P(Text("If you didn't request a password reset, you can safely ignore this email.")),
		),
	)
}

// generateToken returns a cryptographically random 256-bit hex string.
func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return hex.EncodeToString(b)
}

// isValidToken checks that a token is a 64-character hex string.
func isValidToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}

// ── Sentinel errors ─────────────────────────────────────────────────
//
// These are shared across the auth handlers and form the contract between
// per-handler validators/orchestrators and the handler that maps them to
// alerts. Their error message *is* the user-facing alert text; the handler
// does not translate.

var (
	// Validation sentinels (per-handler validators).
	ErrInvalidEmail   = errors.New("invalid email format")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password must be at most 72 characters")
	ErrMissingToken     = errors.New("missing token")
	ErrInvalidRequest   = errors.New("invalid request")

	// Orchestrator sentinels.
	ErrInvalidCredentials      = errors.New("invalid email or password")
	ErrUnverifiedEmail         = errors.New("please verify your email before logging in")
	ErrEmailTaken              = errors.New("email already in use")
	ErrEmailSendFailed         = errors.New("failed to send verification email — please try again")
	ErrInvalidOrExpiredToken   = errors.New("reset link is invalid or has expired")
)

// ── Shared field validators ─────────────────────────────────────────
//
// Single-field rule checks. Return error (at most one) so per-handler
// validateXxxInput funcs can compose them into a []error.

func validateEmail(email string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrInvalidEmail
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	if len(password) > 72 {
		return ErrPasswordTooLong
	}
	return nil
}
