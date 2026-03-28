package contextkeys

type contextKey string

const (
	SessionCookieName contextKey = "session_id"
	User              contextKey = "user"
)

// SessionUser holds the authenticated user data attached to the request context
// by the LoadUser middleware. A nil value means the request is unauthenticated.
type SessionUser struct {
	ID        int64
	Email     string
	SessionID string
}
