package auth

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/starfederation/datastar-go/datastar"
	"golang.org/x/crypto/bcrypt"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/routes"
	. "bahago/internal/ui"
)

// ── Login ───────────────────────────────────────────────────────────

func (h *handler) loginPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		verified := r.URL.Query().Get(verifiedParam) == "true"
		reset := r.URL.Query().Get(resetParam) == "true"
		NewPage("Login", AppLayout(r), loginContent(verified, reset, loginSigDef.New())).Render(w)
	}
}

type LoginForm struct {
	Email    Signal[string] `json:"email"`
	Password Signal[string] `json:"password"`
}

var loginSigDef = NewSignalDef[LoginForm]()

func loginContent(verified bool, reset bool, sigs LoginForm) Node {
	return Group([]Node{
		H1(Text("Login")),
		If(verified, P(Text("Your email has been verified. You can now log in."))),
		If(reset, P(Text("Your password has been reset. You can now log in."))),
		Div(
			Label(
				Text("Email"),
				Input(ds.Bind(sigs.Email.Key)),
			),
			Label(
				Text("Password"),
				Input(Type("password"), ds.Bind(sigs.Password.Key)),
			),
		),
		Button(Class("btn"),
			Text("Login"),
			ds.On("click", datastar.PostSSE(routes.LoginPath)),
		),
		P(
			Text("Don't have an account? "),
			A(Href(routes.RegisterPath), Text("Register")),
		),
		P(
			A(Href(routes.ForgotPasswordPath), Text("Forgot your password?")),
		),
		errorComponent(nil),
		Div(ID(resendVerificationID)),
	})
}

func (h *handler) login() http.HandlerFunc {

	// Sentinel password to compare when doing dummy comparaison
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

		validateEmail(&errs, data.Email.Value)

		if len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent(errs))
			return
		}

		// Look up user. If not found, run a dummy bcrypt comparison against the
		// sentinel hash to prevent user-enumeration via timing side-channel.
		user, dbErr := h.queries.GetUserByEmail(r.Context(), data.Email.Value)
		hashToCompare := []byte(user.PwHash)
		if dbErr != nil {
			hashToCompare = sentinelHash
		}

		if err := bcrypt.CompareHashAndPassword(hashToCompare, []byte(data.Password.Value)); err != nil || dbErr != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("invalid email or password")}))
			return
		}

		if !user.IsVerified {
			sse := datastar.NewSSE(w, r)
			sse.PatchElementGostar(errorComponent([]error{errors.New("please verify your email before logging in")}))
			sse.PatchElementGostar(resendVerificationComponent())
			return
		}

		// TODO: behind a reverse proxy, r.RemoteAddr will be the proxy's address.
		// Read X-Forwarded-For or X-Real-IP instead when deploying with a proxy.
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr // fallback if no port present
		}
		ip, err := netip.ParseAddr(host)
		if err != nil {
			log.Printf("login: parse remote addr %q: %v", host, err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("could not process request")}))
			return
		}

		const sessionDuration = 90 * 24 * time.Hour // 3 months

		s := db.CreateSessionParams{
			ID:        generateToken(),
			UserID:    user.ID,
			IpAddress: ip,
			UserAgent: r.UserAgent(),
			ExpiresAt: time.Now().Add(sessionDuration),
		}

		if _, err := h.queries.CreateSession(r.Context(), s); err != nil {
			log.Printf("login: create session: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to login")}))
			return
		}

		if err := h.queries.UpdateLastLogin(r.Context(), user.ID); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to login")}))
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     string(contextkeys.SessionCookieName),
			Value:    s.ID,
			Path:     "/",
			MaxAge:   int(sessionDuration.Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		sse := datastar.NewSSE(w, r)
		if err := sse.Redirect(routes.KingdomPath); err != nil {
			sse.PatchElementGostar(errorComponent([]error{errors.New("failed to login")}))
		}
	}
}
