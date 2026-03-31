package login

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/netip"
	"time"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/email"
	"bahago/internal/pages/realm"
	"bahago/internal/router"
	. "bahago/internal/ui"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"
	"golang.org/x/crypto/bcrypt"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

// Routes owned by this package.
const (
	LoginPath          = "/login"
	RegisterPath       = "/register"
	VerifyPath         = "/verify"
	LogoutPath         = "/logout"
	ForgotPasswordPath = "/forgot-password"
	ResetPasswordPath  = "/reset-password"
)

// ── Routing & handler setup ─────────────────────────────────────────

func RegisterRoutes(r router.Router, queries *db.Queries, pool *pgxpool.Pool, sender *email.Sender, appURL string) {
	h := newHandler(queries, pool, sender, appURL)
	r.HandleFunc("GET "+LoginPath, h.loginPage())
	r.HandleFunc("GET "+RegisterPath, h.registerPage())
	r.HandleFunc("GET "+VerifyPath, h.verify())
	r.HandleFunc("GET "+ForgotPasswordPath, h.forgotPasswordPage())
	r.HandleFunc("GET "+ResetPasswordPath, h.resetPasswordPage())
	r.HandleFunc("POST "+LoginPath, h.login())
	r.HandleFunc("POST "+RegisterPath, h.register())
	r.HandleFunc("POST "+LogoutPath, h.logout())
	r.HandleFunc("POST "+ForgotPasswordPath, h.forgotPassword())
	r.HandleFunc("POST "+ResetPasswordPath, h.resetPassword())
}

type handler struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	sender  *email.Sender
	appURL  string
}

func newHandler(queries *db.Queries, pool *pgxpool.Pool, sender *email.Sender, appURL string) *handler {
	return &handler{
		queries: queries,
		pool:    pool,
		sender:  sender,
		appURL:  appURL,
	}
}

// ── Login ───────────────────────────────────────────────────────────

func (h *handler) loginPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		verified := r.URL.Query().Get("verified") == "true"
		reset := r.URL.Query().Get("reset") == "true"
		loginPage(verified, reset).Render(w)
	}
}

type LoginForm struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

var loginSignals = LoginForm{Email: "email", Password: "password"}

func loginPage(verified bool, reset bool) Node {
	return Layout(
		LayoutArgs{
			Title: "Login",
			User:  nil,
		},
		H1(Text("Login")),
		If(verified, P(Text("Your email has been verified. You can now log in."))),
		If(reset, P(Text("Your password has been reset. You can now log in."))),
		Div(
			Label(
				Text("Email"),
				Input(ds.Bind(loginSignals.Email)),
			),
			Label(
				Text("Password"),
				Input(Type("password"), ds.Bind(loginSignals.Password)),
			),
		),
		Button(
			Text("Login"),
			ds.On("click", datastar.PostSSE(LoginPath)),
		),
		P(
			Text("Don't have an account? "),
			A(Href(RegisterPath), Text("Register")),
		),
		P(
			A(Href(ForgotPasswordPath), Text("Forgot your password?")),
		),
		errorComponent(nil),
	)
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

		if _, err := mail.ParseAddress(data.Email); err != nil {
			errs = append(errs, errors.New("invalid email format"))
		}

		if len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent(errs))
			return
		}

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

		if !user.IsVerified {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("please verify your email before logging in")}))
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
		if err := sse.Redirect(realm.RealmPath); err != nil {
			sse.PatchElementGostar(errorComponent([]error{errors.New("failed to login")}))
		}
	}
}

// ── Register ────────────────────────────────────────────────────────
type RegisterForm struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

var registerSignals = RegisterForm{Email: "email", Password: "password"}

func (h *handler) registerPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		registerPage().Render(w)
	}
}

func registerPage() Node {
	return Layout(
		LayoutArgs{Title: "Register", User: nil},
		H1(Text("Create an account")),
		Div(
			ds.Signals(map[string]any{"showPassword": false}),
			Label(
				Text("Email"),
				Input(ds.Bind(registerSignals.Email)),
			),
			Label(
				Text("Password"),
				Input(ds.Bind(registerSignals.Password), ds.Attr("type", "$showPassword ? 'text' : 'password'")),
			),
			Button(
				Type("button"),
				ds.Text("$showPassword ? 'Hide password' : 'Show password'"),
				ds.On("click", "$showPassword = !$showPassword"),
			),
		),
		Button(
			Text("Register"),
			ds.On("click", datastar.PostSSE(RegisterPath)),
		),
		P(
			Text("Already have an account? "),
			A(Href(LoginPath), Text("Login")),
		),
		errorComponent(nil),
	)
}

func (h *handler) register() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var errs []error

		data := &RegisterForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			errs = append(errs, errors.New("invalid request"))
		}

		if _, err := mail.ParseAddress(data.Email); err != nil {
			errs = append(errs, errors.New("invalid email format"))
		}

		if len(data.Password) < 8 {
			errs = append(errs, errors.New("password must be at least 8 characters"))
		}

		if len(data.Password) > 72 {
			errs = append(errs, errors.New("password must be at most 72 characters"))
		}

		if len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent(errs))
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("register: bcrypt hash: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to create account")}))
			return
		}

		tx, err := h.pool.Begin(r.Context())
		if err != nil {
			log.Printf("register: begin transaction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to create account")}))
			return
		}
		defer tx.Rollback(r.Context()) // no-op after Commit

		qtx := h.queries.WithTx(tx)

		userID, err := qtx.CreateUser(r.Context(), db.CreateUserParams{
			Email:  data.Email,
			PwHash: string(hashedPassword),
		})
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("email already in use")}))
			return
		}

		token := generateToken()

		if err := qtx.CreateEmailVerification(r.Context(), db.CreateEmailVerificationParams{
			Token:     token,
			UserID:    userID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}); err != nil {
			log.Printf("register: create email verification: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to create account")}))
			return
		}

		verifyURL := h.appURL + "/verify?token=" + token

		if err := h.sender.Send(r.Context(), data.Email, "Verify your email", verificationEmail(verifyURL)); err != nil {
			log.Printf("send verification email: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to send verification email — please try again")}))
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			log.Printf("register: commit transaction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to create account")}))
			return
		}

		datastar.NewSSE(w, r).PatchElementGostar(
			Div(ID("errors"), P(Text("Account created! Check your email to verify your account."))),
		)
	}
}

// ── Verify ──────────────────────────────────────────────────────────

func (h *handler) verify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if !isValidToken(token) {
			invalidTokenPage().Render(w)
			return
		}

		userID, err := h.queries.ConsumeEmailVerification(r.Context(), token)
		if err != nil {
			invalidTokenPage().Render(w)
			return
		}

		if err := h.queries.VerifyUser(r.Context(), userID); err != nil {
			log.Printf("verify: update user verified: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, LoginPath+"?verified=true", http.StatusSeeOther)
	}
}

// ── Logout ──────────────────────────────────────────────────────────

func (h *handler) logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(string(contextkeys.SessionCookieName))
		if err != nil {
			// No session cookie — already logged out.
			http.Redirect(w, r, LoginPath, http.StatusSeeOther)
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
		http.Redirect(w, r, LoginPath, http.StatusSeeOther)
	}
}

// ── Forgot password ─────────────────────────────────────────────────
type ForgotPasswordForm struct {
	Email string `json:"email"`
}

var forgotPasswordSignals = ForgotPasswordForm{Email: "email"}

func (h *handler) forgotPasswordPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		forgotPasswordPage().Render(w)
	}
}

func forgotPasswordPage() Node {
	return Layout(
		LayoutArgs{Title: "Forgot Password", User: nil},
		H1(Text("Reset your password")),
		Div(
			Label(Text("Email"), Input(ds.Bind(forgotPasswordSignals.Email))),
		),
		Button(
			Text("Send reset link"),
			ds.On("click", datastar.PostSSE(ForgotPasswordPath)),
		),
		errorComponent(nil),
	)
}

func (h *handler) forgotPassword() http.HandlerFunc {
	// TODO: add rate limiting to prevent email spam (cost + deliverability risk).
	return func(w http.ResponseWriter, r *http.Request) {
		data := &ForgotPasswordForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("invalid request")}))
			return
		}

		if _, err := mail.ParseAddress(data.Email); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("invalid email format")}))
			return
		}

		// Look up user — but always show a generic success response to prevent enumeration.
		user, err := h.queries.GetUserByEmail(r.Context(), data.Email)
		if err == nil && user.IsVerified {
			token := generateToken()

			if err := h.queries.DeletePasswordResetTokensByUserID(r.Context(), user.ID); err != nil {
				log.Printf("forgot-password: delete old tokens: %v", err)
			}

			pwReset := db.CreatePasswordResetTokenParams{
				Token:     token,
				UserID:    user.ID,
				ExpiresAt: time.Now().Add(1 * time.Hour),
			}
			if err := h.queries.CreatePasswordResetToken(r.Context(), pwReset); err != nil {
				log.Printf("forgot-password: create token: %v", err)
			} else {
				resetURL := h.appURL + "/reset-password?token=" + token
				if err := h.sender.Send(r.Context(), data.Email, "Reset your password", resetPasswordEmail(resetURL)); err != nil {
					log.Printf("forgot-password: send email: %v", err)
				}
			}
		}

		datastar.NewSSE(w, r).PatchElementGostar(
			Div(ID("forgot-result"), P(Text("If that email is registered, you'll receive a reset link shortly."))),
		)
	}
}

// ── Reset password ──────────────────────────────────────────────────
type ResetPasswordForm struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

var resetPasswordSignals = ResetPasswordForm{Token: "token", Password: "password"}

func (h *handler) resetPasswordPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if !isValidToken(token) {
			invalidTokenPage().Render(w)
			return
		}
		resetPasswordPage(token).Render(w)
	}
}

func resetPasswordPage(token string) Node {
	return Layout(
		LayoutArgs{Title: "Reset Password", User: nil},
		H1(Text("Choose a new password")),
		Div(
			ds.Signals(map[string]any{resetPasswordSignals.Token: token, "showPassword": false}),
			Label(
				Text("New password"),
				Input(ds.Bind(resetPasswordSignals.Password), ds.Attr("type", "$showPassword ? 'text' : 'password'")),
			),
			Button(
				Type("button"),
				ds.Text("$showPassword ? 'Hide password' : 'Show password'"),
				ds.On("click", "$showPassword = !$showPassword"),
			),
			Button(
				Text("Reset password"),
				ds.On("click", datastar.PostSSE(ResetPasswordPath)),
			),
			errorComponent(nil),
		),
	)
}

func (h *handler) resetPassword() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := &ResetPasswordForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("invalid request")}))
			return
		}

		if data.Token == "" {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("missing token")}))
			return
		}

		var errs []error
		if len(data.Password) < 8 {
			errs = append(errs, errors.New("password must be at least 8 characters"))
		}
		if len(data.Password) > 72 {
			errs = append(errs, errors.New("password must be at most 72 characters"))
		}
		if len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent(errs))
			return
		}

		tx, err := h.pool.Begin(r.Context())
		if err != nil {
			log.Printf("reset-password: begin transaction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to reset password")}))
			return
		}
		defer tx.Rollback(r.Context()) // no-op after Commit

		qtx := h.queries.WithTx(tx)

		userID, err := qtx.ConsumePasswordResetToken(r.Context(), data.Token)
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("reset link is invalid or has expired")}))
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("reset-password: bcrypt hash: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to reset password")}))
			return
		}

		if err := qtx.UpdatePassword(r.Context(), db.UpdatePasswordParams{
			ID:     userID,
			PwHash: string(hashedPassword),
		}); err != nil {
			log.Printf("reset-password: update password: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to reset password")}))
			return
		}

		if err := qtx.DeleteSessionsByUserID(r.Context(), userID); err != nil {
			log.Printf("reset-password: delete sessions: %v", err)
		}

		if err := tx.Commit(r.Context()); err != nil {
			log.Printf("reset-password: commit transaction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to reset password")}))
			return
		}

		sse := datastar.NewSSE(w, r)
		if err := sse.Redirect(LoginPath + "?reset=true"); err != nil {
			sse.PatchElementGostar(errorComponent([]error{errors.New("failed to redirect")}))
		}
	}
}

// ── Shared templates & helpers ──────────────────────────────────────

func errorComponent(errors []error) Node {
	return Div(
		ID("errors"),
		Map(errors, func(e error) Node {
			return P(Text(e.Error()))
		}),
	)
}

func invalidTokenPage() Node {
	return Layout(LayoutArgs{Title: "Verification Failed", User: nil},
		H1(Text("Verification link invalid or expired")),
		P(Text("Please register again to receive a new link.")),
	)
}

func verificationEmail(verifyURL string) Node {
	return HTML(
		Head(
			Meta(Charset("utf-8")),
		),
		Body(
			H1(Text("Verify your email address")),
			P(Text("Thanks for signing up! Click the link below to verify your email address. The link expires in 24 hours.")),
			P(
				A(Href(verifyURL), Text("Verify my email")),
			),
			P(Text("If you didn't create an account, you can safely ignore this email.")),
		),
	)
}

func resetPasswordEmail(resetURL string) Node {
	return HTML(
		Head(
			Meta(Charset("utf-8")),
		),
		Body(
			H1(Text("Reset your password")),
			P(Text("We received a request to reset your password. Click the link below to choose a new one. The link expires in 1 hour.")),
			P(
				A(Href(resetURL), Text("Reset my password")),
			),
			P(Text("If you didn't request a password reset, you can safely ignore this email.")),
		),
	)
}

// generateToken returns a cryptographically random 256-bit hex string.
func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return hex.EncodeToString(b)
}

// isValidToken checks that a token is a 64-character hex string.
func isValidToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}
