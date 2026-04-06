package auth

import (
	"log"
	"net/http"

	"bahago/internal/routes"
	. "bahago/internal/ui"
)

// ── Verify ──────────────────────────────────────────────────────────
func (h *handler) verify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get(tokenParam)
		if !isValidToken(token) {
			NewPage("Verification Failed", AppLayout(r), invalidTokenContent()).Render(w)
			return
		}

		userID, err := h.queries.ConsumeEmailVerification(r.Context(), token)
		if err != nil {
			NewPage("Verification Failed", AppLayout(r), invalidTokenContent()).Render(w)
			return
		}

		if err := h.queries.VerifyUser(r.Context(), userID); err != nil {
			log.Printf("verify: update user verified: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, routes.LoginPath+"?"+verifiedParam+"=true", http.StatusSeeOther)
	}
}
