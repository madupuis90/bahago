package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"bahago/internal/database/db"
	"bahago/internal/handlers/auth"
	"bahago/internal/routes"
	"bahago/internal/testhelper"
)

// stubQuerier embeds a nil db.Querier. Only GetUserByEmail is overridden;
// any other DB call panics to catch unexpected interactions.
type stubQuerier struct {
	db.Querier
	onGetUserByEmail func(email string) (db.User, error)
}

func (s *stubQuerier) GetUserByEmail(_ context.Context, email string) (db.User, error) {
	if s.onGetUserByEmail != nil {
		return s.onGetUserByEmail(email)
	}
	panic("stubQuerier: unexpected call to GetUserByEmail")
}

// loginHandler extracts the POST login handler from RegisterRoutes using the
// provided querier. Pass &stubQuerier{} for tests that return before the DB call.
func loginHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	auth.RegisterRoutes(cr, q, nil, nil, "")
	return cr.Handlers["POST "+routes.LoginPath]
}

func loginReq(body string) *http.Request {
	return httptest.NewRequest("POST", routes.LoginPath, strings.NewReader(body))
}

func TestLogin_EmptyEmail(t *testing.T) {
	h := loginHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, loginReq(`{"email":"","password":"somepassword"}`))
	testhelper.AssertContains(t, w.Body.String(), "invalid email format")
}

func TestLogin_InvalidEmailFormat(t *testing.T) {
	h := loginHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, loginReq(`{"email":"notanemail","password":"somepassword"}`))
	testhelper.AssertContains(t, w.Body.String(), "invalid email format")
}

func TestLogin_InvalidCredentials(t *testing.T) {
	// Querier returns no user; sentinel hash comparison will fail.
	stub := &stubQuerier{
		onGetUserByEmail: func(_ string) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
	}
	h := loginHandler(stub)
	w := httptest.NewRecorder()
	h(w, loginReq(`{"email":"test@example.com","password":"wrongpassword"}`))
	testhelper.AssertContains(t, w.Body.String(), "invalid email or password")
}

func TestLogin_UnverifiedEmail(t *testing.T) {
	// Pre-hash the password so bcrypt.CompareHashAndPassword succeeds and the
	// handler reaches the IsVerified check. MinCost keeps the test fast.
	password := "correctpassword"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	stub := &stubQuerier{
		onGetUserByEmail: func(_ string) (db.User, error) {
			return db.User{
				Email:      "unverified@example.com",
				PwHash:     string(hash),
				IsVerified: false,
			}, nil
		},
	}
	h := loginHandler(stub)
	w := httptest.NewRecorder()
	h(w, loginReq(`{"email":"unverified@example.com","password":"correctpassword"}`))
	testhelper.AssertContains(t, w.Body.String(), "please verify your email before logging in")
}
