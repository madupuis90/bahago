package auth

import (
	"log"
	"net/http"

	. "bahago/internal/layout"
	"bahago/internal/routes"
)

// ── Verify ──────────────────────────────────────────────────────────
func (h *handler) verify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get(tokenParam)
		if !isValidToken(token) {
			HomeLayout(r, "Verification Failed", invalidTokenContent()).Render(w)
			return
		}

		userID, err := h.queries.ConsumeEmailVerification(r.Context(), token)
		if err != nil {
			HomeLayout(r, "Verification Failed", invalidTokenContent()).Render(w)
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
