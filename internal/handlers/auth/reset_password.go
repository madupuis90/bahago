package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/starfederation/datastar-go/datastar"
	"golang.org/x/crypto/bcrypt"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/database/db"
	. "bahago/internal/ui"
	"bahago/internal/routes"
)

// ── Reset password ──────────────────────────────────────────────────

type ResetPasswordForm struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *handler) resetPasswordPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get(tokenParam)
		if !isValidToken(token) {
			HomeLayout(r, "Verification Failed", invalidTokenContent()).Render(w)
			return
		}
		HomeLayout(r, "Reset Password", resetPasswordContent(token)).Render(w)
	}
}

func resetPasswordContent(token string) Node {
	return Div(Class("auth-card panel"),
		ds.Signals(map[string]any{
			"token":        token,
			"showPassword": false,
		}),
		H1(Text("Choose a new password")),
		Div(Class("form-fields"),
			Label(
				Text("New password"),
				Div(Class("password-field"),
					Input(ds.Bind("password"), ds.Attr("type", "$showPassword ? 'text' : 'password'")),
					Button(Class("btn-text"),
						Type("button"),
						ds.Text("$showPassword ? 'Hide' : 'Show'"),
						ds.On("click", "$showPassword = !$showPassword"),
					),
				),
			),
		),
		Button(Class("btn"),
			Text("Reset password"),
			ds.On("click", datastar.PostSSE(routes.ResetPasswordPath)),
		),
		authAlert(nil),
	)
}

func (h *handler) resetPassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := &ResetPasswordForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(ErrInvalidRequest)))
			return
		}

		if errs := validateResetPasswordInput(data); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(errs...)))
			return
		}

		if err := h.resetUserPassword(r.Context(), data.Token, data.Password); err != nil {
			if isResetPasswordUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(err)))
				return
			}
			log.Printf("reset-password: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(errors.New("failed to reset password"))))
			return
		}

		sse := datastar.NewSSE(w, r)
		if err := sse.Redirect(routes.LoginPath + "?" + resetParam + "=true"); err != nil {
			sse.PatchElementGostar(authAlert(AlertError(errors.New("failed to redirect"))))
		}
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

func validateResetPasswordInput(in *ResetPasswordForm) []error {
	var errs []error
	if in.Token == "" {
		errs = append(errs, ErrMissingToken)
	}
	if err := validatePassword(in.Password); err != nil {
		errs = append(errs, err)
	}
	return errs
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// resetUserPassword consumes the reset token, updates the password hash, and
// invalidates all existing sessions for the user — all inside a single
// transaction. Returns ErrInvalidOrExpiredToken when the token lookup fails;
// session deletion failure is logged but not fatal (a stale session won't
// have valid credentials anyway).
func (h *handler) resetUserPassword(ctx context.Context, token, newPassword string) error {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := db.New(tx)

	userID, err := qtx.ConsumePasswordResetToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidOrExpiredToken
		}
		return fmt.Errorf("consume token: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("bcrypt hash: %w", err)
	}

	if err := qtx.UpdatePassword(ctx, db.UpdatePasswordParams{
		ID:     userID,
		PwHash: string(hashedPassword),
	}); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	if err := qtx.DeleteSessionsByUserID(ctx, userID); err != nil {
		log.Printf("reset-password: delete sessions: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func isResetPasswordUserError(err error) bool {
	return errors.Is(err, ErrInvalidOrExpiredToken)
}
