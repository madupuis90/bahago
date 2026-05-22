package prayers

import (
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
			datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(errors.New("invalid request")))
			return
		}

		prayerType := input.PrayerType
		def, ok := game.PrayerDefs[prayerType]
		if !ok {
			datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(errors.New("unknown prayer type")))
			return
		}

		if input.PrayerTicks < 1 || input.PrayerTicks > 48 {
			datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(errors.New("duration must be between 1 and 48 ticks")))
			return
		}

		// SERIALIZABLE transaction: the prayer count read and the insert share the same
		// transaction so PostgreSQL SSI detects concurrent double-casts of different prayer
		// types that would otherwise both pass the maxPrayers check independently.
		tx, err := h.pool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			log.Printf("cast prayer: begin tx: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(errors.New("internal error")))
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck

		txq := db.New(tx)

		existing, err := txq.ListKingdomPrayers(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("cast prayer: list prayers: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(errors.New("internal error")))
			return
		}
		if len(existing) >= maxPrayers {
			datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(fmt.Errorf("you may only have %d active prayer at a time", maxPrayers)))
			return
		}

		_, err = txq.CreatePrayer(r.Context(), db.CreatePrayerParams{
			KingdomID:  kingdom.ID,
			PrayerType: prayerType,
			// TODO: resolve input.TargetKingdomName to an ID when cross-kingdom targeting is enabled.
			TargetKingdomID: kingdom.ID,
			TicksTotal:      input.PrayerTicks,
		})
		if err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
				datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(fmt.Errorf("%s is already active on this kingdom", def.Name)))
				return
			}
			log.Printf("cast prayer: create: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(errors.New("internal error")))
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			log.Printf("cast prayer: commit: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(errors.New("internal error")))
			return
		}

		h.renderPrayersPage(w, r, kingdom.ID, "cast prayer")
	}
}

func (h *handler) handleCancel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom, _ := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		prayerID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(errors.New("invalid prayer ID")))
			return
		}

		if err := h.queries.DeletePrayer(r.Context(), db.DeletePrayerParams{
			ID:        prayerID,
			KingdomID: kingdom.ID,
		}); err != nil {
			log.Printf("cancel prayer: delete: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(errors.New("internal error")))
			return
		}

		h.renderPrayersPage(w, r, kingdom.ID, "cancel prayer")
	}
}

// renderPrayersPage reloads the kingdom and its prayers from the DB and patches
// the main content area with a fresh prayers page. It is called at the end of
// both handleCast and handleCancel after the mutating DB operation succeeds.
func (h *handler) renderPrayersPage(w http.ResponseWriter, r *http.Request, kingdomID int, logPrefix string) {
	prayers, err := h.queries.ListKingdomPrayers(r.Context(), kingdomID)
	if err != nil {
		log.Printf("%s: list prayers: %v", logPrefix, err)
		datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(errors.New("internal error")))
		return
	}
	k, err := h.queries.GetKingdomByID(r.Context(), kingdomID)
	if err != nil {
		log.Printf("%s: reload kingdom: %v", logPrefix, err)
		datastar.NewSSE(w, r).PatchElementGostar(prayerErrorComponent(errors.New("internal error")))
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
		prayerErrorComponent(nil),
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

func prayerErrorComponent(err error) Node {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return Div(ID("prayer-error"), Class("prayer-error"), Text(msg))
}

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
