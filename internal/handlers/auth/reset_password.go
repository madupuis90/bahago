package auth

import (
	"errors"
	"log"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
	"golang.org/x/crypto/bcrypt"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/database/db"
	. "bahago/internal/layout"
	"bahago/internal/routes"
	"bahago/internal/signals"
)

// ── Reset password ──────────────────────────────────────────────────

type ResetPasswordForm struct {
	Token    signals.Signal[string] `json:"token"`
	Password signals.Signal[string] `json:"password"`
}

var resetPasswordSigDef = signals.NewSignalDef[ResetPasswordForm]()

func (h *handler) resetPasswordPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get(tokenParam)
		if !isValidToken(token) {
			HomeLayout(r, "Verification Failed", invalidTokenContent()).Render(w)
			return
		}
		sigs := resetPasswordSigDef.New()
		sigs.Token.Value = token
		HomeLayout(r, "Reset Password", resetPasswordContent(sigs)).Render(w)
	}
}

func resetPasswordContent(sigs ResetPasswordForm) Node {
	return Div(Class("auth-card panel"),
		ds.Signals(signals.SignalMap(sigs)),
		H1(Text("Choose a new password")),
		Div(Class("form-fields"),
			ds.Signals(map[string]any{"showPassword": false}),
			Label(
				Text("New password"),
				Div(Class("password-field"),
					Input(ds.Bind(sigs.Password.Key), ds.Attr("type", "$showPassword ? 'text' : 'password'")),
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
		alertComponent(nil),
	)
}

func (h *handler) resetPassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := &ResetPasswordForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("invalid request")})))
			return
		}

		if data.Token.Value == "" {
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("missing token")})))
			return
		}

		var errs []error
		validatePassword(&errs, data.Password.Value)
		if len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent(errs)))
			return
		}

		tx, err := h.pool.Begin(r.Context())
		if err != nil {
			log.Printf("reset-password: begin transaction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to reset password")})))
			return
		}
		defer tx.Rollback(r.Context()) // no-op after Commit

		qtx := h.queries.WithTx(tx)

		userID, err := qtx.ConsumePasswordResetToken(r.Context(), data.Token.Value)
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("reset link is invalid or has expired")})))
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.Password.Value), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("reset-password: bcrypt hash: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to reset password")})))
			return
		}

		if err := qtx.UpdatePassword(r.Context(), db.UpdatePasswordParams{
			ID:     userID,
			PwHash: string(hashedPassword),
		}); err != nil {
			log.Printf("reset-password: update password: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to reset password")})))
			return
		}

		if err := qtx.DeleteSessionsByUserID(r.Context(), userID); err != nil {
			log.Printf("reset-password: delete sessions: %v", err)
		}

		if err := tx.Commit(r.Context()); err != nil {
			log.Printf("reset-password: commit transaction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to reset password")})))
			return
		}

		sse := datastar.NewSSE(w, r)
		if err := sse.Redirect(routes.LoginPath + "?" + resetParam + "=true"); err != nil {
			sse.PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to redirect")})))
		}
	}
}
