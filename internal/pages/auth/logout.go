package auth

import (
	"log"
	"net/http"

	"bahago/internal/contextkeys"
	"bahago/internal/routes"
)

// ── Logout ──────────────────────────────────────────────────────────

func (h *handler) logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(string(contextkeys.SessionCookieName))
		if err != nil {
			// No session cookie — already logged out.
			http.Redirect(w, r, routes.LoginPath, http.StatusSeeOther)
			return
		}

		if err := h.queries.DeleteSession(r.Context(), cookie.Value); err != nil {
			log.Printf("logout: delete session: %v", err)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     string(contextkeys.SessionCookieName),
			MaxAge:   -1,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, routes.LoginPath, http.StatusSeeOther)
	}
}
