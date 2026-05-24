package prayers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/game"
	"bahago/internal/hub"
	. "bahago/internal/ui"
	"bahago/internal/router"
	"bahago/internal/routes"
)

// maxPrayers is the default maximum number of prayers a kingdom may have active at once.
// Unlockable via skills in the future.
const maxPrayers = 1

// ── Input struct ──────────────────────────────────────────────────────────────

type castInput struct {
	PrayerType  string `json:"prayer_type"`
	PrayerTicks int    `json:"prayer_ticks"`
	// TargetKingdomName is bound to the hidden target_kingdom signal in the UI.
	// Cross-kingdom resolution is not yet implemented; the handler currently
	// ignores this value and always targets the casting kingdom itself.
	TargetKingdomName string `json:"target_kingdom"`
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrUnknownPrayerType     = errors.New("unknown prayer type")
	ErrInvalidPrayerDuration = errors.New("duration must be between 1 and 48 ticks")
	ErrMaxPrayersReached     = fmt.Errorf("you may only have %d active prayer at a time", maxPrayers)
	ErrPrayerAlreadyActive   = errors.New("this prayer is already active on this kingdom")
	ErrInvalidPrayerID       = errors.New("invalid prayer ID")
	ErrPrayerNotFound        = errors.New("prayer not found")
)

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier, pool *pgxpool.Pool, tickHub *hub.Hub) {
	h := newHandler(queries, pool, tickHub)
	r.HandleFunc("GET "+routes.KingdomPrayersPath, h.handlePrayersPage())
	r.HandleFunc("GET "+routes.KingdomPrayersRefreshPath, h.handlePrayersRefresh())
	r.HandleFunc("POST "+routes.KingdomPrayerCastPath, h.handleCast())
	r.HandleFunc("POST "+routes.KingdomPrayerCancelPath, h.handleCancel())
}

func cancelPrayerPath(id int) string {
	return strings.ReplaceAll(routes.KingdomPrayerCancelPath, "{id}", strconv.Itoa(id))
}

type handler struct {
	queries db.Querier
	pool    *pgxpool.Pool
	hub     *hub.Hub
}

func newHandler(queries db.Querier, pool *pgxpool.Pool, tickHub *hub.Hub) *handler {
	return &handler{queries: queries, pool: pool, hub: tickHub}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handlePrayersPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom, _ := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		prayers, err := h.queries.ListKingdomPrayers(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("prayers page: list prayers: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		KingdomLayout(r, "Prayers", r.URL.Path, kingdom, prayersContent(*kingdom, prayers)).Render(w)
	}
}

func (h *handler) handlePrayersRefresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		ch, cleanup := h.hub.Subscribe(kingdom.ID)
		defer cleanup()

		sse := datastar.NewSSE(w, r)
		for {
			select {
			case <-r.Context().Done():
				return
			case k := <-ch:
				prayers, err := h.queries.ListKingdomPrayers(r.Context(), k.ID)
				if err != nil {
					log.Printf("prayers refresh: list prayers: %v", err)
					return
				}
				if err := sse.PatchElementGostar(MainContent(prayersContent(k, prayers))); err != nil {
					log.Printf("prayers refresh: patch: %v", err)
					return
				}
			}
		}
	}
}

func (h *handler) handleCast() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom, _ := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		input := &castInput{}
		if err := datastar.ReadSignals(r, input); err != nil {
			log.Printf("cast prayer: read signals: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(prayerAlert(AlertError(errors.New("invalid request"))))
			return
		}

		if errs := validateCastInput(input); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(prayerAlert(AlertError(errs...)))
			return
		}

		if err := h.castPrayer(r.Context(), kingdom.ID, input); err != nil {
			if isCastUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(prayerAlert(AlertError(err)))
				return
			}
			log.Printf("cast prayer: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(prayerAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.renderPrayersPage(w, r, kingdom.ID, "cast prayer")
	}
}

func (h *handler) handleCancel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom, _ := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		prayerID, err := validatePrayerID(r.PathValue("id"))
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(prayerAlert(AlertError(err)))
			return
		}

		if err := h.cancelPrayer(r.Context(), kingdom.ID, prayerID); err != nil {
			if isCancelUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(prayerAlert(AlertError(err)))
				return
			}
			log.Printf("cancel prayer: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(prayerAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.renderPrayersPage(w, r, kingdom.ID, "cancel prayer")
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

// validateCastInput runs every field-level rule that does not need DB access.
func validateCastInput(input *castInput) []error {
	var errs []error
	if _, ok := game.PrayerDefs[input.PrayerType]; !ok {
		errs = append(errs, ErrUnknownPrayerType)
	}
	if input.PrayerTicks < 1 || input.PrayerTicks > 48 {
		errs = append(errs, ErrInvalidPrayerDuration)
	}
	return errs
}

// validatePrayerID parses and bounds-checks the path id.
func validatePrayerID(idStr string) (int, error) {
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return 0, ErrInvalidPrayerID
	}
	return id, nil
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// castPrayer opens a SERIALIZABLE transaction, checks the active-prayer cap,
// and inserts the new prayer. The serializable isolation ensures concurrent
// double-casts of different prayer types cannot both pass the cap check.
// Returns ErrMaxPrayersReached when the kingdom already has maxPrayers active,
// ErrPrayerAlreadyActive when the unique (kingdom_id, prayer_type) constraint
// fires. Other errors are wrapped for logging.
func (h *handler) castPrayer(ctx context.Context, kingdomID int, input *castInput) error {
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txq := db.New(tx)

	existing, err := txq.ListKingdomPrayers(ctx, kingdomID)
	if err != nil {
		return fmt.Errorf("list prayers: %w", err)
	}
	if len(existing) >= maxPrayers {
		return ErrMaxPrayersReached
	}

	if _, err := txq.CreatePrayer(ctx, db.CreatePrayerParams{
		KingdomID:  kingdomID,
		PrayerType: input.PrayerType,
		// TODO: resolve input.TargetKingdomName to an ID when cross-kingdom targeting is enabled.
		TargetKingdomID: kingdomID,
		TicksTotal:      input.PrayerTicks,
	}); err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("%w: %s", ErrPrayerAlreadyActive, game.PrayerDefs[input.PrayerType].Name)
		}
		return fmt.Errorf("create prayer: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// cancelPrayer deletes a prayer owned by kingdomID.
func (h *handler) cancelPrayer(ctx context.Context, kingdomID, prayerID int) error {
	if err := h.queries.DeletePrayer(ctx, db.DeletePrayerParams{
		ID:        prayerID,
		KingdomID: kingdomID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPrayerNotFound
		}
		return fmt.Errorf("delete prayer: %w", err)
	}
	return nil
}

func isCancelUserError(err error) bool {
	return errors.Is(err, ErrPrayerNotFound)
}

func isCastUserError(err error) bool {
	return errors.Is(err, ErrMaxPrayersReached) || errors.Is(err, ErrPrayerAlreadyActive)
}

// renderPrayersPage reloads the kingdom and its prayers from the DB and patches
// the main content area with a fresh prayers page. It is called at the end of
// both handleCast and handleCancel after the mutating DB operation succeeds.
func (h *handler) renderPrayersPage(w http.ResponseWriter, r *http.Request, kingdomID int, logPrefix string) {
	prayers, err := h.queries.ListKingdomPrayers(r.Context(), kingdomID)
	if err != nil {
		log.Printf("%s: list prayers: %v", logPrefix, err)
		datastar.NewSSE(w, r).PatchElementGostar(prayerAlert(AlertError(errors.New("internal error"))))
		return
	}
	k, err := h.queries.GetKingdomByID(r.Context(), kingdomID)
	if err != nil {
		log.Printf("%s: reload kingdom: %v", logPrefix, err)
		datastar.NewSSE(w, r).PatchElementGostar(prayerAlert(AlertError(errors.New("internal error"))))
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := sse.PatchElementGostar(MainContent(prayersContent(k, prayers))); err != nil {
		log.Printf("%s: patch: %v", logPrefix, err)
	}
}

// ── Page ──────────────────────────────────────────────────────────────────────

func prayersContent(kingdom db.Kingdom, prayers []db.KingdomPrayer) Node {
	return Group([]Node{
		Div(ds.Init(GetSSENoSignals(routes.KingdomPrayersRefreshPath))),
		H1(Class("page-title"), Text("Prayers")),
		ds.Signals(map[string]any{
			"prayer_type":    game.PrayerManaPrayer,
			"prayer_ticks":   8,
			"target_kingdom": kingdom.Name,
		}, ds.ModifierIfMissing),
		prayerAlert(nil),
		activePrayersSection(prayers),
		availablePrayersSection(),
	})
}

func activePrayersSection(prayers []db.KingdomPrayer) Node {
	prayerKeys := make([]string, 0, len(game.PrayerDefs))
	for k := range game.PrayerDefs {
		prayerKeys = append(prayerKeys, k)
	}
	sort.Strings(prayerKeys)

	castDisabled := len(prayers) >= maxPrayers

	return Div(Class("prayer-active-section panel"),
		P(Class("panel-title"), Text("Active Prayers")),
		Div(Class("prayer-cast-form"),
			Label(For("prayer_type"), Text("Prayer")),
			Select(ID("prayer_type"), ds.Bind("prayer_type"),
				Group(Map(prayerKeys, func(key string) Node {
					def := game.PrayerDefs[key]
					return Option(Value(key), Text(def.Name))
				})),
			),
			Label(For("prayer_ticks"), Text("Duration")),
			Input(Type("number"), ID("prayer_ticks"), Min("1"), Max("48"), ds.Bind("prayer_ticks")),
			Input(Type("hidden"), ds.Bind("target_kingdom")),
			Button(
				Classes{"btn": true, "btn--locked": castDisabled},
				ds.On("click", datastar.PostSSE(routes.KingdomPrayerCastPath)),
				If(castDisabled, Disabled()),
				Text("Pray"),
			),
		),
		Iff(len(prayers) == 0, func() Node {
			return P(Class("prayer-empty"), Text("No active prayers."))
		}),
		Iff(len(prayers) > 0, func() Node {
			return Table(Class("prayer-active-table"),
				THead(Tr(
					Th(Text("Prayer")),
					Th(Text("Effect")),
					Th(Text("Ticks Remaining")),
					Th(Text("")),
				)),
				TBody(Map(prayers, func(p db.KingdomPrayer) Node {
					return activePrayerRow(p)
				})),
			)
		}),
	)
}

func activePrayerRow(p db.KingdomPrayer) Node {
	def, ok := game.PrayerDefs[p.PrayerType]
	name := p.PrayerType
	effect := ""
	if ok {
		name = def.Name
		effect = resourceBonusText(def)
	}
	return Tr(
		Td(Text(name)),
		Td(Text(effect)),
		Td(Text(fmt.Sprintf("%d / %d", p.TicksRemaining, p.TicksTotal))),
		Td(Button(
			Class("btn btn--danger"),
			ds.On("click", datastar.PostSSE("%s", cancelPrayerPath(p.ID))),
			Text("Cancel"),
		)),
	)
}

func availablePrayersSection() Node {
	cards := make([]Node, 0, len(game.PrayerDefs))
	for _, def := range game.PrayerDefs {
		cards = append(cards, availablePrayerCard(def))
	}
	return Group([]Node{
		H2(Class("section-heading"), Text("Available Prayers")),
		Div(Class("prayers-grid"), Group(cards)),
	})
}

func availablePrayerCard(def game.PrayerDef) Node {
	effectText := resourceBonusText(def)
	return Div(Class("prayer-card panel"),
		P(Class("panel-title"), Text(def.Name)),
		Iff(def.Description != "", func() Node {
			return P(Class("prayer-card__description"), Text(def.Description))
		}),
		P(Class("prayer-card__effect"), Text(effectText)),
		P(Class("prayer-card__upkeep"), Text(fmt.Sprintf("Devotion upkeep: %d/tick", def.DevotionUpkeep))),
	)
}

func prayerAlert(inner Node) Node { return AlertContainer("prayer-alert", inner) }

func resourceBonusText(def game.PrayerDef) string {
	b := def.ResourceBonusPct
	var parts []string
	if b.Wood != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% wood production", b.Wood))
	}
	if b.Stone != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% stone production", b.Stone))
	}
	if b.Food != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% food production", b.Food))
	}
	if b.Mana != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% mana production", b.Mana))
	}
	if b.Devotion != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% devotion production", b.Devotion))
	}
	if b.Knowledge != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% knowledge production", b.Knowledge))
	}
	if len(parts) == 0 {
		return "No resource bonus"
	}
	return strings.Join(parts, ", ")
}
