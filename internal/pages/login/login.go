package login

import (
	"bytes"
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
	"bahago/internal/router"
	. "bahago/internal/ui"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"
	"golang.org/x/crypto/bcrypt"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

func RegisterRoutes(r router.Router, queries *db.Queries, pool *pgxpool.Pool, sender *email.Sender, appURL string) {
	h := newHandler(queries, pool, sender, appURL)
	r.HandleFunc("GET /login", h.loginPage())
	r.HandleFunc("POST /register", h.register())
	r.HandleFunc("POST /login", h.login())
	r.HandleFunc("GET /verify", h.verify())
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

func (h *handler) loginPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		verified := r.URL.Query().Get("verified") == "true"
		loginPage(verified).Render(w)
	}
}

func loginPage(verified bool) Node {
	return Layout(
		LayoutArgs{
			Title: "Login",
		},
		H1(Text("Login Page")),
		If(verified, P(Text("Your email has been verified. You can now log in."))),
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

// verificationEmail renders an HTML email containing a verification link.
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

// Token not found, already used, or expired — show a friendly page.
func invalidToken() Node {
	return Layout(LayoutArgs{Title: "Verification Failed"},
		H1(Text("Verification link invalid or expired")),
		P(Text("Please register again to receive a new link.")),
	)
}

type LoginForm struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *handler) register() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

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
		var buf bytes.Buffer
		if err := verificationEmail(verifyURL).Render(&buf); err != nil {
			log.Printf("register: render verification email: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(errorComponent([]error{errors.New("failed to create account")}))
			return
		}

		if err := h.sender.Send(r.Context(), data.Email, "Verify your email", buf.String()); err != nil {
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

func (h *handler) verify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			invalidToken().Render(w)
			return
		}

		userID, err := h.queries.ConsumeEmailVerification(r.Context(), token)
		if err != nil {
			invalidToken().Render(w)
			return
		}

		if err := h.queries.VerifyUser(r.Context(), userID); err != nil {
			log.Printf("verify: update user verified: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/login?verified=true", http.StatusSeeOther)
	}
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
			sse.PatchElementGostar(errorComponent([]error{errors.New("failed to login")}))
		}
	}
}

// generateToken returns a cryptographically random 256-bit hex string.
func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return hex.EncodeToString(b)
}
