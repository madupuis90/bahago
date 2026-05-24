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
	"bahago/internal/routes"
)

// ── Resend verification ─────────────────────────────────────────────

type ResendVerificationForm struct {
	Email string `json:"email"`
}

func resendVerificationComponent() Node {
	return Div(ID("resend-verification"),
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

		// Always respond generically to prevent user-enumeration via the
		// response. Orchestrator errors are logged but do not change the UI.
		if err := h.resendEmailVerification(r.Context(), data.Email); err != nil {
			log.Printf("resend-verification: %v", err)
		}
		genericResendMessage(w, r)
	}
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// resendEmailVerification regenerates and sends a verification email for the
// given address. Silently absorbs unknown emails and already-verified accounts
// (no user-enumeration leak). Returns wrapped errors for true infra failures
// so they can be logged by the caller.
func (h *handler) resendEmailVerification(ctx context.Context, email string) error {
	user, err := h.queries.GetUserByEmail(ctx, email)
	if err != nil || user.IsVerified {
		return nil
	}

	if err := h.queries.DeleteEmailVerificationByUserID(ctx, user.ID); err != nil {
		log.Printf("resend-verification: delete old tokens: %v", err)
	}

	token := generateToken()
	if err := h.queries.CreateEmailVerification(ctx, db.CreateEmailVerificationParams{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		return fmt.Errorf("create token: %w", err)
	}

	verifyURL := h.appURL + routes.VerifyPath + "?" + tokenParam + "=" + token
	fmt.Println(verifyURL) // TODO: remove once domain is set up

	if err := h.sender.Send(ctx, email, "Verify your email", verificationEmail(verifyURL)); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

func genericResendMessage(w http.ResponseWriter, r *http.Request) {
	datastar.NewSSE(w, r).PatchElementGostar(
		Div(ID("resend-verification"), Class("alert--success"), P(Text("If that email is registered and unverified, a new verification link has been sent."))),
	)
}
