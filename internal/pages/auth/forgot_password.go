package auth

import (
	"errors"
	"log"
	"net/http"
	"time"

	"bahago/internal/database/db"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	. "bahago/internal/ui"
)

// ── Forgot password ─────────────────────────────────────────────────

type ForgotPasswordForm struct {
	Email string `json:"email"`
}

var forgotPasswordSignals = ForgotPasswordForm{Email: "email"}

func (h *handler) forgotPasswordPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		forgotPasswordPage().Render(w)
	}
}

func forgotPasswordPage() Node {
	return Layout(
		LayoutArgs{Title: "Forgot Password", User: nil},
		H1(Text("Reset your password")),
		Div(
			Label(Text("Email"), Input(ds.Bind(forgotPasswordSignals.Email))),
		),
		Button(
			Text("Send reset link"),
			ds.On("click", datastar.PostSSE(ForgotPasswordPath)),
		),
		errorComponent(nil),
	)
}

func (h *handler) forgotPassword() http.HandlerFunc {
	// TODO: add rate limiting to prevent email spam (cost + deliverability risk).
	return func(w http.ResponseWriter, r *http.Request) {
		data := &ForgotPasswordForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("invalid request")}))
			return
		}

		var errs []error
		validateEmail(&errs, data.Email)
		if len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent(errs))
			return
		}

		// Look up user — but always show a generic success response to prevent enumeration.
		user, err := h.queries.GetUserByEmail(r.Context(), data.Email)
		if err == nil && user.IsVerified {
			token := generateToken()

			if err := h.queries.DeletePasswordResetTokensByUserID(r.Context(), user.ID); err != nil {
				log.Printf("forgot-password: delete old tokens: %v", err)
			}

			pwReset := db.CreatePasswordResetTokenParams{
				Token:     token,
				UserID:    user.ID,
				ExpiresAt: time.Now().Add(1 * time.Hour),
			}
			if err := h.queries.CreatePasswordResetToken(r.Context(), pwReset); err != nil {
				log.Printf("forgot-password: create token: %v", err)
			} else {
				resetURL := h.appURL + "/reset-password?token=" + token
				if err := h.sender.Send(r.Context(), data.Email, "Reset your password", resetPasswordEmail(resetURL)); err != nil {
					log.Printf("forgot-password: send email: %v", err)
				}
			}
		}

		datastar.NewSSE(w, r).PatchElementGostar(
			Div(ID("forgot-result"), P(Text("If that email is registered, you'll receive a reset link shortly."))),
		)
	}
}
