package auth

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/database/db"
	"bahago/internal/routes"
	. "bahago/internal/ui"
)

// ── Forgot password ─────────────────────────────────────────────────

type ForgotPasswordForm struct {
	Email Signal[string] `json:"email"`
}

var forgotPasswordSigDef = NewSignalDef[ForgotPasswordForm]()

func (h *handler) forgotPasswordPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		NewPage("Forgot Password", AppLayout(r), forgotPasswordContent(forgotPasswordSigDef.New())).Render(w)
	}
}

func forgotPasswordContent(sigs ForgotPasswordForm) Node {
	return Group([]Node{
		H1(Text("Reset your password")),
		Div(
			Label(Text("Email"), Input(ds.Bind(sigs.Email.Key))),
		),
		Button(Class("btn"),
			Text("Send reset link"),
			ds.On("click", datastar.PostSSE(routes.ForgotPasswordPath)),
		),
		errorComponent(nil),
		Div(ID(forgotResultID)),
	})
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
		validateEmail(&errs, data.Email.Value)
		if len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent(errs))
			return
		}

		// Always respond with the same generic message regardless of outcome to prevent enumeration.
		user, err := h.queries.GetUserByEmail(r.Context(), data.Email.Value)
		if err != nil {
			genericMessage(w, r)
			return
		}

		if err := h.queries.DeletePasswordResetTokensByUserID(r.Context(), user.ID); err != nil {
			log.Printf("forgot-password: delete old tokens: %v", err)
		}

		token := generateToken()
		pwReset := db.CreatePasswordResetTokenParams{
			Token:     token,
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		if err := h.queries.CreatePasswordResetToken(r.Context(), pwReset); err != nil {
			log.Printf("forgot-password: create token: %v", err)
			genericMessage(w, r)
			return
		}

		resetURL := h.appURL + routes.ResetPasswordPath + "?" + tokenParam + "=" + token
		fmt.Println(resetURL) // TODO: remove - only use for testing until I get a domain so e-mail are not flagged
		if err := h.sender.Send(r.Context(), data.Email.Value, "Reset your password", resetPasswordEmail(resetURL)); err != nil {
			log.Printf("forgot-password: send email: %v", err)
		}
		genericMessage(w, r)
	}
}

func genericMessage(w http.ResponseWriter, r *http.Request) {
	datastar.NewSSE(w, r).PatchElementGostar(
		Div(ID(forgotResultID), P(Text("If that email is registered, you'll receive a reset link shortly."))),
	)
}
