package login

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/netip"
	"time"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/router"
	. "bahago/internal/ui"

	"github.com/starfederation/datastar-go/datastar"
	"golang.org/x/crypto/bcrypt"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

func RegisterRoutes(router router.Router, queries *db.Queries) {
	h := newHandler(queries)
	router.HandleFunc("GET /login", h.loginPage())
	router.HandleFunc("POST /register", h.register())
	router.HandleFunc("POST /login", h.login())
}

type handler struct {
	queries *db.Queries
}

func newHandler(queries *db.Queries) *handler {
	return &handler{
		queries: queries,
	}
}

func (h *handler) loginPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loginPage().Render(w)
	}
}

func loginPage() Node {
	return Layout(
		LayoutArgs{
			Title: "Login",
		},
		H1(Text("Login Page")),
		Div(
			Label(
				Text("email"),
				Input(ds.Bind("email")),
			),
			Label(
				Text("password"),
				Input(ds.Bind("password")),
			),
		),
		Div(
			Button(
				Text("Register"),
				ds.On("click", datastar.PostSSE("/register")),
			),
			Button(
				Text("Login"),
				ds.On("click", datastar.PostSSE("/login")),
			),
		),
		errorComponent(nil),
	)
}

func errorComponent(errors []error) Node {
	return Div(
		ID("errors"),
		Map(errors, func(e error) Node {
			return P(Text(e.Error()))
		}),
	)
}

type LoginForm struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *handler) register() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// --- Phase 1: parse + validate (accumulate all errors) ---
		var errs []error

		data := &LoginForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			errs = append(errs, errors.New("invalid request"))
		}

		if _, err := mail.ParseAddress(data.Email); err != nil {
			errs = append(errs, errors.New("invalid email format"))
		}

		if len(data.Password) < 8 {
			errs = append(errs, errors.New("password must be at least 8 characters"))
		}

		if len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent(errs))
			return
		}

		// --- Phase 2: execute (fail fast) ---
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.Password), bcrypt.DefaultCost)
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to create account")}))
			return
		}

		user := db.CreateUserParams{
			Email:  data.Email,
			PwHash: string(hashedPassword),
		}

		if _, err := h.queries.CreateUser(r.Context(), user); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("email already in use")}))
			return
		}

		datastar.NewSSE(w, r).PatchElementGostar(errorComponent(nil))
	}
}

func (h *handler) login() http.HandlerFunc {
	// Computed once when the handler is registered, captured by the closure.
	// Used to perform a constant-time dummy bcrypt comparison when an email is
	// not found, preventing user-enumeration via timing side-channel.
	sentinelHash, err := bcrypt.GenerateFromPassword([]byte("sentinel-password"), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("failed to generate sentinel hash: %v", err))
	}

	return func(w http.ResponseWriter, r *http.Request) {

		var errs []error

		data := &LoginForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			errs = append(errs, errors.New("invalid request"))
		}

		if _, err := mail.ParseAddress(data.Email); err != nil {
			errs = append(errs, errors.New("invalid email format"))
		}

		if len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent(errs))
			return
		}

		// --- Phase 2: execute (fail fast) ---

		// Look up user. If not found, run a dummy bcrypt comparison against the
		// sentinel hash to prevent user-enumeration via timing side-channel.
		user, dbErr := h.queries.GetUserByEmail(r.Context(), data.Email)
		hashToCompare := []byte(user.PwHash)
		if dbErr != nil {
			hashToCompare = sentinelHash
		}

		if err := bcrypt.CompareHashAndPassword(hashToCompare, []byte(data.Password)); err != nil || dbErr != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("invalid email or password")}))
			return
		}

		// TODO: check user.IsActive and user.IsVerified once email verification
		// and account management flows are implemented.

		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr // fallback if no port present
		}
		ip, err := netip.ParseAddr(host)
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("could not process request")}))
			return
		}

		const sessionDuration = 90 * 24 * time.Hour // 3 months

		s := db.CreateSessionParams{
			ID:        generateSessionID(),
			UserID:    user.ID,
			IpAddress: ip,
			UserAgent: r.UserAgent(),
			ExpiresAt: time.Now().Add(sessionDuration),
		}

		if _, err := h.queries.CreateSession(r.Context(), s); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to login")}))
			return
		}

		if err := h.queries.UpdateLastLogin(r.Context(), user.ID); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to login")}))
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     string(contextkeys.SessionID),
			Value:    s.ID,
			Path:     "/",
			MaxAge:   int(sessionDuration.Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		sse := datastar.NewSSE(w, r)
		if err := sse.Redirect("/realm"); err != nil {
			// Headers already sent at this point; best effort patch.
			sse.PatchElementGostar(errorComponent([]error{errors.New("failed to login")}))
		}
	}
}

// generateSessionID returns a cryptographically random 256-bit hex string.
// It panics if the system CSPRNG is unavailable, since a broken random source
// makes it impossible to generate secure session tokens safely.
func generateSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return hex.EncodeToString(b)
}
