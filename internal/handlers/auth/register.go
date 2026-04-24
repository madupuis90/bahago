package auth

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/starfederation/datastar-go/datastar"
	"golang.org/x/crypto/bcrypt"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/database/db"
	. "bahago/internal/layout"
	"bahago/internal/routes"
)

// ── Register ────────────────────────────────────────────────────────

type RegisterForm struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *handler) registerPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		HomeLayout(r, "Register", registerContent()).Render(w)
	}
}

func registerContent() Node {
	return Div(Class("auth-card panel"),
		H1(Text("Create an account")),
		Div(Class("form-fields"),
			ds.Signals(map[string]any{
				"showPassword": false,
			}),
			Label(
				Text("Email"),
				Input(Type("email"), ds.Bind("email")),
			),
			Label(
				Text("Password"),
				Div(Class("password-field"),
					Input(ds.Bind("password"), ds.Attr("type", "$showPassword ? 'text' : 'password'")),
					Button(Class("btn-text"),
						Type("button"),
						ds.Text("$showPassword ? 'Hide' : 'Show'"),
						ds.On("click", "$showPassword = !$showPassword"),
					),
				),
			),
		),
		Button(Class("btn"),
			Text("Register"),
			ds.On("click", datastar.PostSSE(routes.RegisterPath)),
		),
		P(
			Text("Already have an account? "),
			A(Href(routes.LoginPath), Text("Login")),
		),
		alertComponent(nil),
	)
}

func (h *handler) register() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var errs []error

		data := &RegisterForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			errs = append(errs, errors.New("invalid request"))
		}

		validateEmail(&errs, data.Email)
		validatePassword(&errs, data.Password)

		if len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent(errs)))
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(data.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("register: bcrypt hash: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to create account")})))
			return
		}

		tx, err := h.pool.Begin(r.Context())
		if err != nil {
			log.Printf("register: begin transaction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to create account")})))
			return
		}
		defer tx.Rollback(r.Context()) // no-op after Commit

		qtx := db.New(tx)

		userID, err := qtx.CreateUser(r.Context(), db.CreateUserParams{
			Email:  data.Email,
			PwHash: string(hashedPassword),
		})
		if err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
				datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("email already in use")})))
				return
			}
			log.Printf("register: create user: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to create account")})))
			return
		}

		token := generateToken()

		verifyEmail := db.CreateEmailVerificationParams{
			Token:     token,
			UserID:    userID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		if err := qtx.CreateEmailVerification(r.Context(), verifyEmail); err != nil {
			log.Printf("register: create email verification: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to create account")})))
			return
		}

		verifyURL := h.appURL + routes.VerifyPath + "?" + tokenParam + "=" + token
		fmt.Println(verifyURL) // TODO: remove - only use for testing until I get a domain so e-mail are not flagged

		if err := h.sender.Send(r.Context(), data.Email, "Verify your email", verificationEmail(verifyURL)); err != nil {
			log.Printf("send verification email: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to send verification email — please try again")})))
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			log.Printf("register: commit transaction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to create account")})))
			return
		}

		datastar.NewSSE(w, r).PatchElementGostar(
			alertComponent(successComponent("Account created! Check your email to verify your account.")),
		)
	}
}
