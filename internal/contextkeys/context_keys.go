package contextkeys

type contextKey string

const (
	UserID    contextKey = "user_id"
	SessionID contextKey = "session_id"
)
