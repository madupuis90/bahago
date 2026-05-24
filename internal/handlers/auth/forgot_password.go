package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/database/db"
	. "bahago/internal/ui"
	"bahago/internal/routes"
)

// ── Forgot password ─────────────────────────────────────────────────

type ForgotPasswordForm struct {
	Email string `json:"email"`
}

func (h *handler) forgotPasswordPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		HomeLayout(r, "Forgot Password", forgotPasswordContent()).Render(w)
	}
}

func forgotPasswordContent() Node {
	return Div(Class("auth-card panel"),
		H1(Text("Reset your password")),
		Div(Class("form-fields"),
			Label(Text("Email"), Input(Type("email"), ds.Bind("email"))),
		),
		Button(Class("btn"),
			Text("Send reset link"),
			ds.On("click", datastar.PostSSE(routes.ForgotPasswordPath)),
		),
		authAlert(nil),
	)
}

func (h *handler) forgotPassword() http.HandlerFunc {
	// TODO: add rate limiting to prevent email spam (cost + deliverability risk).
	return func(w http.ResponseWriter, r *http.Request) {
		data := &ForgotPasswordForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(ErrInvalidRequest)))
			return
		}

		if errs := validateForgotPasswordInput(data); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(errs...)))
			return
		}

		// Always respond generically to prevent user-enumeration via the
		// response. Orchestrator errors are logged but do not change the UI.
		if err := h.initiatePasswordReset(r.Context(), data.Email); err != nil {
			log.Printf("forgot-password: %v", err)
		}
		genericMessage(w, r)
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

func validateForgotPasswordInput(in *ForgotPasswordForm) []error {
	var errs []error
	if err := validateEmail(in.Email); err != nil {
		errs = append(errs, err)
	}
	return errs
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// initiatePasswordReset deletes any existing reset tokens for the user,
// creates a fresh one, and sends the reset email. Returns nil silently when
// the email is not registered (no user-enumeration leak). Returns wrapped
// errors for true infra failures so they can be logged by the caller.
func (h *handler) initiatePasswordReset(ctx context.Context, email string) error {
	user, err := h.queries.GetUserByEmail(ctx, email)
	if err != nil {
		// Unknown email — silently absorb to prevent enumeration.
		return nil
	}

	if err := h.queries.DeletePasswordResetTokensByUserID(ctx, user.ID); err != nil {
		// Non-fatal — log and continue creating a fresh token.
		log.Printf("forgot-password: delete old tokens: %v", err)
	}

	token := generateToken()
	if err := h.queries.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}); err != nil {
		return fmt.Errorf("create token: %w", err)
	}

	resetURL := h.appURL + routes.ResetPasswordPath + "?" + tokenParam + "=" + token
	fmt.Println(resetURL) // TODO: remove once domain is set up
	if err := h.sender.Send(ctx, email, "Reset your password", resetPasswordEmail(resetURL)); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

func genericMessage(w http.ResponseWriter, r *http.Request) {
	datastar.NewSSE(w, r).PatchElementGostar(
		authAlert(AlertSuccess("If that email is registered, you'll receive a reset link shortly.")),
	)
}
