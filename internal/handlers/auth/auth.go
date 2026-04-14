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

func RegisterRoutes(r router.Router, queries *db.Queries, pool *pgxpool.Pool, sender *email.Sender, appURL string) {
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
	queries *db.Queries
	pool    *pgxpool.Pool
	sender  *email.Sender
	appURL  string
}

func newHandler(queries *db.Queries, pool *pgxpool.Pool, sender *email.Sender, appURL string) *handler {
	return &handler{
		queries: queries,
		pool:    pool,
		sender:  sender,
		appURL:  appURL,
	}
}

// ── Shared templates & helpers ──────────────────────────────────────

const (
	authAlertID          = "auth-alert"
	resendVerificationID = "resend-verification"
)

// alertComponent is the single SSE patch target for all auth feedback.
// Always patch this — never patch errorComponent or successComponent directly.
func alertComponent(inner Node) Node {
	return Div(ID(authAlertID), inner)
}

// errorComponent returns the inner error content for use inside alertComponent.
// Returns nil when errs is empty, producing a clean empty placeholder.
func errorComponent(errs []error) Node {
	if len(errs) == 0 {
		return nil
	}
	return Div(Class("alert-error"),
		Map(errs, func(e error) Node {
			return P(Text(e.Error()))
		}),
	)
}

// successComponent returns the inner success content for use inside alertComponent.
func successComponent(msg string) Node {
	return Div(Class("alert-success"),
		P(Text(msg)),
	)
}

func invalidTokenContent() Node {
	return Div(Class("auth-card panel"),
		H1(Text("Verification link invalid or expired")),
		P(Text("Please register again to receive a new link.")),
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

// ── Validation helpers ──────────────────────────────────────────────

func validateEmail(errs *[]error, email string) {
	if _, err := mail.ParseAddress(email); err != nil {
		*errs = append(*errs, errors.New("invalid email format"))
	}
}

func validatePassword(errs *[]error, password string) {
	if len(password) < 8 {
		*errs = append(*errs, errors.New("password must be at least 8 characters"))
	}
	if len(password) > 72 {
		*errs = append(*errs, errors.New("password must be at most 72 characters"))
	}
}
