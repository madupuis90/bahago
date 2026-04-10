package auth

import (
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

// ── Resend verification ─────────────────────────────────────────────

type ResendVerificationForm struct {
	Email Signal[string] `json:"email"`
}

func resendVerificationComponent() Node {
	return Div(ID(resendVerificationID),
		Button(Class("btn"),
			Text("Resend verification email"),
			ds.On("click", datastar.PostSSE(routes.ResendVerificationPath)),
		),
	)
}

func (h *handler) resendVerification() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := &ResendVerificationForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			genericResendMessage(w, r)
			return
		}

		// Always respond generically to prevent enumeration.
		user, err := h.queries.GetUserByEmail(r.Context(), data.Email.Value)
		if err != nil || user.IsVerified {
			genericResendMessage(w, r)
			return
		}

		if err := h.queries.DeleteEmailVerificationByUserID(r.Context(), user.ID); err != nil {
			log.Printf("resend-verification: delete old tokens: %v", err)
		}

		token := generateToken()
		params := db.CreateEmailVerificationParams{
			Token:     token,
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		if err := h.queries.CreateEmailVerification(r.Context(), params); err != nil {
			log.Printf("resend-verification: create token: %v", err)
			genericResendMessage(w, r)
			return
		}

		verifyURL := h.appURL + routes.VerifyPath + "?" + tokenParam + "=" + token
		fmt.Println(verifyURL) // TODO: remove - only use for testing until I get a domain so e-mail are not flagged

		if err := h.sender.Send(r.Context(), data.Email.Value, "Verify your email", verificationEmail(verifyURL)); err != nil {
			log.Printf("resend-verification: send email: %v", err)
		}
		genericResendMessage(w, r)
	}
}

func genericResendMessage(w http.ResponseWriter, r *http.Request) {
	datastar.NewSSE(w, r).PatchElementGostar(
		Div(ID(resendVerificationID), P(Text("If that email is registered and unverified, a new verification link has been sent."))),
	)
}
