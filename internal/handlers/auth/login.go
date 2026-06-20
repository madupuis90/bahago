package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/starfederation/datastar-go/datastar"
	"golang.org/x/crypto/bcrypt"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	. "bahago/internal/ui"
	"bahago/internal/routes"
)

// ── Login ───────────────────────────────────────────────────────────

func (h *handler) loginPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		verified := r.URL.Query().Get(verifiedParam) == "true"
		reset := r.URL.Query().Get(resetParam) == "true"
		AuthLayout(r, "Sign In", loginContent(verified, reset)).Render(w)
	}
}

type LoginForm struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func loginContent(verified bool, reset bool) Node {
	return Div(Class("auth-wrap"),
		Div(Class("card"),
			Div(Class("auth-crest"),
				authCrest("crown"),
				Div(Class("auth-wordmark"), Text("Bahago")),
				Div(Class("auth-tagline"), Text("A realm awaits your command")),
			),
			Div(Class("auth-divider")),
			Div(Class("auth-body"),
				If(verified, Div(Class("auth-alert auth-alert--success"), Text("Your email has been verified. You can now log in."))),
				If(reset, Div(Class("auth-alert auth-alert--success"), Text("Your password has been reset. You can now log in."))),
				Form(Class("auth-form"),
					Div(Class("field-group"),
						Label(Class("field-label"), For("email"), Text("Email")),
						Input(ID("email"), Type("email"), ds.Bind("email")),
					),
					Div(Class("field-group"),
						Label(Class("field-label"), For("password"), Text("Password")),
						Div(Class("password-field"),
							Input(ID("password"), Type("password"), ds.Bind("password")),
						),
					),
					A(Class("auth-quiet"), Href(routes.ForgotPasswordPath), Text("Forgot your password?")),
				),
				Button(Class("auth-btn"),
					Text("Enter the Realm"),
					ds.On("click", datastar.PostSSE(routes.LoginPath)),
				),
				authAlert(nil),
				Div(ID("resend-verification")),
				Div(Class("auth-foot"),
					Span(Class("auth-foot-text"), Text("New to Bahago?")),
					A(Class("auth-foot-link"), Href(routes.RegisterPath), Text("Join the Realm")),
				),
			),
		),
	)
}

// sessionDuration is how long a freshly minted login session lasts.
const sessionDuration = 90 * 24 * time.Hour // 3 months

// sentinelHash is a precomputed bcrypt hash used for the user-not-found timing
// side-channel defence. Set in init so the per-request authenticateUser call
// has constant work regardless of whether the user exists.
var sentinelHash []byte

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte("sentinel-password"), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("failed to generate sentinel hash: %v", err))
	}
	sentinelHash = h
}

func (h *handler) login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := &LoginForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(ErrInvalidRequest)))
			return
		}

		if errs := validateLoginInput(data); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(errs...)))
			return
		}

		user, err := h.authenticateUser(r.Context(), data.Email, data.Password)
		if err != nil {
			if errors.Is(err, ErrUnverifiedEmail) {
				sse := datastar.NewSSE(w, r)
				sse.PatchElementGostar(authAlert(AlertError(err)))
				sse.PatchElementGostar(resendVerificationComponent())
				return
			}
			if isLoginUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(err)))
				return
			}
			log.Printf("login: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(errors.New("failed to login"))))
			return
		}

		ip, err := parseRemoteIP(r.RemoteAddr)
		if err != nil {
			log.Printf("login: parse remote addr %q: %v", r.RemoteAddr, err)
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(errors.New("could not process request"))))
			return
		}

		token, err := h.startSession(r.Context(), user.ID, ip, r.UserAgent())
		if err != nil {
			log.Printf("login: start session: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(errors.New("failed to login"))))
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     string(contextkeys.SessionCookieName),
			Value:    token,
			Path:     "/",
			MaxAge:   int(sessionDuration.Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		sse := datastar.NewSSE(w, r)
		if err := sse.Redirect(routes.KingdomPath); err != nil {
			sse.PatchElementGostar(authAlert(AlertError(errors.New("failed to login"))))
		}
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

func validateLoginInput(in *LoginForm) []error {
	var errs []error
	if err := validateEmail(in.Email); err != nil {
		errs = append(errs, err)
	}
	return errs
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// authenticateUser verifies the email+password pair using a constant-time
// fallback hash when the user is not found (defence against user-enumeration
// via timing side-channels). Returns ErrInvalidCredentials on any auth failure,
// ErrUnverifiedEmail when credentials match but the email is not verified.
func (h *handler) authenticateUser(ctx context.Context, email, password string) (*db.User, error) {
	user, dbErr := h.queries.GetUserByEmail(ctx, email)
	hashToCompare := []byte(user.PwHash)
	if dbErr != nil {
		hashToCompare = sentinelHash
	}
	if dbErr != nil && !errors.Is(dbErr, pgx.ErrNoRows) {
		log.Printf("login: get user: %v", dbErr)
	}
	if err := bcrypt.CompareHashAndPassword(hashToCompare, []byte(password)); err != nil || dbErr != nil {
		return nil, ErrInvalidCredentials
	}
	if !user.IsVerified {
		return nil, ErrUnverifiedEmail
	}
	return &user, nil
}

// startSession creates a new session row and updates the user's last-login
// timestamp. Returns the session token to be set as the cookie.
func (h *handler) startSession(ctx context.Context, userID int, ip netip.Addr, userAgent string) (string, error) {
	token := generateToken()
	s := db.CreateSessionParams{
		ID:        token,
		UserID:    userID,
		IpAddress: ip,
		UserAgent: userAgent,
		ExpiresAt: time.Now().Add(sessionDuration),
	}
	if _, err := h.queries.CreateSession(ctx, s); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	if err := h.queries.UpdateLastLogin(ctx, userID); err != nil {
		return "", fmt.Errorf("update last login: %w", err)
	}
	return token, nil
}

// parseRemoteIP extracts a netip.Addr from an http.Request.RemoteAddr value,
// falling back to the raw string if the host has no port.
func parseRemoteIP(remoteAddr string) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return netip.ParseAddr(host)
}

func isLoginUserError(err error) bool {
	return errors.Is(err, ErrInvalidCredentials)
}
