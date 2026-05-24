package auth

import (
	"context"
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
	. "bahago/internal/ui"
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
		authAlert(nil),
	)
}

func (h *handler) register() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := &RegisterForm{}
		if err := datastar.ReadSignals(r, data); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(ErrInvalidRequest)))
			return
		}

		if errs := validateRegisterInput(data); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(errs...)))
			return
		}

		if err := h.registerUser(r.Context(), data.Email, data.Password); err != nil {
			if isRegisterUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(err)))
				return
			}
			log.Printf("register: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(authAlert(AlertError(errors.New("failed to create account"))))
			return
		}

		datastar.NewSSE(w, r).PatchElementGostar(
			authAlert(AlertSuccess("Account created! Check your email to verify your account.")),
		)
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

func validateRegisterInput(in *RegisterForm) []error {
	var errs []error
	if err := validateEmail(in.Email); err != nil {
		errs = append(errs, err)
	}
	if err := validatePassword(in.Password); err != nil {
		errs = append(errs, err)
	}
	return errs
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// registerUser hashes the password, inserts the user, creates a verification
// token, and sends the verification email — all inside a single transaction
// so the user is rolled back if email delivery fails. Returns ErrEmailTaken
// for a unique-violation on email, ErrEmailSendFailed for SMTP failures.
func (h *handler) registerUser(ctx context.Context, email, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("bcrypt hash: %w", err)
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := db.New(tx)

	userID, err := qtx.CreateUser(ctx, db.CreateUserParams{
		Email:  email,
		PwHash: string(hashedPassword),
	})
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrEmailTaken
		}
		return fmt.Errorf("create user: %w", err)
	}

	token := generateToken()
	if err := qtx.CreateEmailVerification(ctx, db.CreateEmailVerificationParams{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		return fmt.Errorf("create email verification: %w", err)
	}

	verifyURL := h.appURL + routes.VerifyPath + "?" + tokenParam + "=" + token
	fmt.Println(verifyURL) // TODO: remove once domain is set up

	if err := h.sender.Send(ctx, email, "Verify your email", verificationEmail(verifyURL)); err != nil {
		return ErrEmailSendFailed
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func isRegisterUserError(err error) bool {
	return errors.Is(err, ErrEmailTaken) || errors.Is(err, ErrEmailSendFailed)
}
