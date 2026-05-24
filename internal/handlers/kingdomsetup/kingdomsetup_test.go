package kingdomsetup

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/routes"
	"bahago/internal/testhelper"
)

// ── Stub querier ──────────────────────────────────────────────────────────────

type stubQuerier struct {
	db.Querier
	onGetKingdomsInViewport func(ctx context.Context, arg db.GetKingdomsInViewportParams) ([]db.GetKingdomsInViewportRow, error)
	onCreateKingdom         func(ctx context.Context, arg db.CreateKingdomParams) (db.Kingdom, error)
}

func (s *stubQuerier) GetKingdomsInViewport(ctx context.Context, arg db.GetKingdomsInViewportParams) ([]db.GetKingdomsInViewportRow, error) {
	if s.onGetKingdomsInViewport != nil {
		return s.onGetKingdomsInViewport(ctx, arg)
	}
	panic("stubQuerier: unexpected call to GetKingdomsInViewport")
}

func (s *stubQuerier) CreateKingdom(ctx context.Context, arg db.CreateKingdomParams) (db.Kingdom, error) {
	if s.onCreateKingdom != nil {
		return s.onCreateKingdom(ctx, arg)
	}
	panic("stubQuerier: unexpected call to CreateKingdom")
}

// emptyMap returns a viewport stub that reports no occupied tiles — the next
// pickFreePosition call will succeed on the first sample.
func emptyMap() func(context.Context, db.GetKingdomsInViewportParams) ([]db.GetKingdomsInViewportRow, error) {
	return func(_ context.Context, _ db.GetKingdomsInViewportParams) ([]db.GetKingdomsInViewportRow, error) {
		return nil, nil
	}
}

// ── validateKingdomName ───────────────────────────────────────────────────────

func TestValidateKingdomName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     string
		wantErr  error
	}{
		{"simple_lowercase_titlecased", "bobtown", "Bobtown", nil},
		{"mixed_case_titlecased", "bObTown", "Bobtown", nil},
		{"whitespace_trimmed", "  Bob  ", "Bob", nil},
		{"empty", "", "", ErrNameRequired},
		{"whitespace_only", "   ", "", ErrNameRequired},
		{"contains_digit", "Bob1", "", ErrNameLettersOnly},
		{"contains_space", "Bob town", "", ErrNameLettersOnly},
		{"contains_hyphen", "Bob-town", "", ErrNameLettersOnly},
		{"unicode_letters_accepted", "Bjørnholm", "Bjørnholm", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateKingdomName(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ── createKingdom ─────────────────────────────────────────────────────────────

func TestCreateKingdom_Success(t *testing.T) {
	var seen db.CreateKingdomParams
	q := &stubQuerier{
		onGetKingdomsInViewport: emptyMap(),
		onCreateKingdom: func(_ context.Context, arg db.CreateKingdomParams) (db.Kingdom, error) {
			seen = arg
			return db.Kingdom{ID: 1, Name: arg.Name, UserID: arg.UserID, X: arg.X, Y: arg.Y}, nil
		},
	}
	h := &handler{queries: q}
	if err := h.createKingdom(context.Background(), 42, "Bobtown"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if seen.UserID != 42 || seen.Name != "Bobtown" {
		t.Errorf("params = %+v, want UserID=42 Name=Bobtown", seen)
	}
}

func TestCreateKingdom_NameTaken(t *testing.T) {
	q := &stubQuerier{
		onGetKingdomsInViewport: emptyMap(),
		onCreateKingdom: func(_ context.Context, _ db.CreateKingdomParams) (db.Kingdom, error) {
			return db.Kingdom{}, &pgconn.PgError{Code: "23505"} // unique_violation
		},
	}
	h := &handler{queries: q}
	err := h.createKingdom(context.Background(), 42, "Bobtown")
	if !errors.Is(err, ErrNameTaken) {
		t.Fatalf("err = %v, want ErrNameTaken", err)
	}
}

func TestCreateKingdom_CreateOtherError(t *testing.T) {
	boom := errors.New("connection refused")
	q := &stubQuerier{
		onGetKingdomsInViewport: emptyMap(),
		onCreateKingdom: func(_ context.Context, _ db.CreateKingdomParams) (db.Kingdom, error) {
			return db.Kingdom{}, boom
		},
	}
	h := &handler{queries: q}
	err := h.createKingdom(context.Background(), 42, "Bobtown")
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
	if isCreateKingdomUserError(err) {
		t.Errorf("unexpected user-error classification for wrapped infra error: %v", err)
	}
}

func TestCreateKingdom_PickPositionError(t *testing.T) {
	boom := errors.New("viewport read failed")
	q := &stubQuerier{
		onGetKingdomsInViewport: func(_ context.Context, _ db.GetKingdomsInViewportParams) ([]db.GetKingdomsInViewportRow, error) {
			return nil, boom
		},
	}
	h := &handler{queries: q}
	err := h.createKingdom(context.Background(), 42, "Bobtown")
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
	if isCreateKingdomUserError(err) {
		t.Errorf("unexpected user-error classification: %v", err)
	}
}

// ── handler shell smoke tests ─────────────────────────────────────────────────

func createHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	RegisterRoutes(cr, q)
	return cr.Handlers["POST "+routes.KingdomCreatePath]
}

func createReq(body string, user *contextkeys.SessionUser) *http.Request {
	r := httptest.NewRequest("POST", routes.KingdomCreatePath, strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), contextkeys.User, user))
}

var sessionUser = &contextkeys.SessionUser{ID: 42, Email: "u@example.com"}

func TestHandleCreateKingdom_ValidatorErrorRenders(t *testing.T) {
	h := createHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, createReq(`{"kingdom_name":""}`, sessionUser))
	testhelper.AssertContains(t, w.Body.String(), "kingdom name is required")
}

func TestHandleCreateKingdom_LettersOnlyRenders(t *testing.T) {
	h := createHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, createReq(`{"kingdom_name":"Bob1"}`, sessionUser))
	testhelper.AssertContains(t, w.Body.String(), "letters")
}

func TestHandleCreateKingdom_NameTakenRenders(t *testing.T) {
	q := &stubQuerier{
		onGetKingdomsInViewport: emptyMap(),
		onCreateKingdom: func(_ context.Context, _ db.CreateKingdomParams) (db.Kingdom, error) {
			return db.Kingdom{}, &pgconn.PgError{Code: "23505"}
		},
	}
	h := createHandler(q)
	w := httptest.NewRecorder()
	h(w, createReq(`{"kingdom_name":"Bobtown"}`, sessionUser))
	testhelper.AssertContains(t, w.Body.String(), "already taken")
}
