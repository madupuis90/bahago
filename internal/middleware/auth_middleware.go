package middleware

import (
	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/pages/auth"
	"context"
	"net/http"
)

// LoadUser runs on all routes. If the request carries a valid session cookie,
// it resolves the user and attaches a *contextkeys.SessionUser to the context.
// If the session is missing, invalid, or the account is inactive/unverified,
// it clears the cookie and continues without a user — no redirect.
func LoadUser(queries *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(string(contextkeys.SessionCookieName))
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			session, err := queries.GetUserBySessionID(r.Context(), cookie.Value)
			if err != nil || !session.IsActive || !session.IsVerified {
				http.SetCookie(w, &http.Cookie{Name: string(contextkeys.SessionCookieName), MaxAge: -1, Path: "/"})
				next.ServeHTTP(w, r)
				return
			}

			user := &contextkeys.SessionUser{
				ID:        session.ID,
				Email:     session.Email,
				SessionID: session.SessionID,
			}
			ctx := context.WithValue(r.Context(), contextkeys.User, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth runs only on protected routes. It checks that LoadUser has
// already resolved a user for this request; if not, it redirects to /login.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser); !ok {
			http.Redirect(w, r, auth.LoginPath, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
