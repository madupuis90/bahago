package middleware

import (
	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"context"
	"net/http"
)

func AuthMiddleware(queries *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			cookies, err := r.Cookie(string(contextkeys.SessionID))
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			session, err := queries.GetUserBySessionID(r.Context(), cookies.Value)
			if err != nil {
				http.SetCookie(w, &http.Cookie{Name: string(contextkeys.SessionID), MaxAge: -1, Path: "/"})
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			// TODO: check session.IsActive and session.IsVerified once email
			// verification and account management flows are implemented.
			ctx := context.WithValue(r.Context(), contextkeys.UserID, session.ID)
			ctx = context.WithValue(ctx, contextkeys.SessionID, session.SessionID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
