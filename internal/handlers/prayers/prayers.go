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
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/game"
	"bahago/internal/hub"
	"bahago/internal/router"
	"bahago/internal/routes"
	. "bahago/internal/ui"
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
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

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
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

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
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

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
	prayerKeys := make([]string, 0, len(game.PrayerDefs))
	for k := range game.PrayerDefs {
		prayerKeys = append(prayerKeys, k)
	}
	sort.Strings(prayerKeys)

	return Div(
		ds.Signals(map[string]any{
			"prayer_type":    game.PrayerManaPrayer,
			"prayer_ticks":   8,
			"target_kingdom": kingdom.Name,
		}, ds.ModifierIfMissing),
		Div(ds.Init(GetSSENoSignals(routes.KingdomPrayersRefreshPath))),
		prayerAlert(nil),
		PageHeader("Prayers"),
		sanctumSection(kingdom, prayers, prayerKeys),
		availablePrayersSection(prayers, prayerKeys),
	)
}

// sanctumSection is the altar card. When a prayer is burning it shows the
// active prayer with a brass tick meter and a cancel button; when the altar
// is free it shows the offering form (prayer · duration stepper · cost · Pray).
func sanctumSection(kingdom db.Kingdom, prayers []db.KingdomPrayer, prayerKeys []string) Node {
	var body Node
	if len(prayers) > 0 {
		body = activePrayerView(prayers[0])
	} else {
		body = offeringForm(kingdom, prayerKeys)
	}
	return Div(Class("card sanctum"),
		Div(Class("card-inner"), body),
	)
}

// activePrayerView renders the single burning prayer: glyph + name/effect +
// drain, a brass tick meter, and a cancel footer.
func activePrayerView(p db.KingdomPrayer) Node {
	def, ok := game.PrayerDefs[p.PrayerType]
	name, upkeep, gemID := p.PrayerType, 0, "sun"
	if ok {
		name = def.Name
		upkeep = def.DevotionUpkeep
		gemID = prayerGemID(def)
	}

	return Div(Class("active-prayer"),
		Div(Class("active-prayer-top"),
			ResourceGem(gemID, 44),
			Div(Class("active-prayer-id"),
				Span(Class("active-prayer-target"),
					Icon("crown", 13, false),
					Text("Upon your realm"),
				),
				Span(Class("active-prayer-name"), Text(name)),
				Iff(ok, func() Node {
					return Span(Class("active-prayer-effect"), Raw(effectHTML(def)))
				}),
			),
			Div(Class("active-prayer-aside"),
				Div(Class("active-prayer-drain"),
					Span(Class("active-prayer-drain-lbl"), Text("Devotion drain")),
					// Per-tick devotion drain: StaticCostPill with CostUpkeep shows
					// devotion gem + amount + sandglass/arrow-down. The down-arrow
					// carries the negative direction, so no explicit minus sign.
					StaticCostPill(
						game.ResourceValues{Devotion: upkeep},
						WithGemSize(18),
						WithCostKind(CostUpkeep),
					),
				),
			),
		),
		TickMeter(Text("Time burning"),
			fmt.Sprintf("%d of %d ticks left", p.TicksRemaining, p.TicksTotal),
			p.TicksRemaining, p.TicksTotal),
		Div(Class("active-prayer-foot"),
			P(Class("active-prayer-foot-note"),
				Text("The prayer drains devotion each tick and ends when it runs dry or the ticks elapse.")),
			Button(Class("btn btn--danger"),
				ds.On("click", datastar.PostSSE("%s", cancelPrayerPath(p.ID))),
				Text("Cancel prayer"),
			),
		),
	)
}

// offeringForm renders the cast form when the altar is free. The duration
// stepper is driven by the $prayer_ticks signal; the total-cost readout is a
// datastar expression (every current prayer costs 20 devotion/tick, so the
// multiplier is fixed — update here if a future prayer changes that).
func offeringForm(kingdom db.Kingdom, prayerKeys []string) Node {
	return Div(Class("offering"),
		Form(Class("offering-form"),
			ds.On("submit", datastar.PostSSE(routes.KingdomPrayerCastPath)),
			Div(Class("field-group"),
				Label(For("prayer_type"), Class("field-label"), Text("Prayer")),
				Select(ID("prayer_type"), Class("select"), ds.Bind("prayer_type"),
					Group(Map(prayerKeys, func(key string) Node {
						return Option(Value(key), Text(game.PrayerDefs[key].Name))
					})),
				),
			),
			Div(Class("field-group"),
				Label(For("prayer_ticks"), Class("field-label"), Text("Duration")),
				Div(Class("dur-rod"),
					Div(Class("slider-wrap v-badge"),
						Div(Class("slider-track"),
							Div(Class("slider-fill"),
								ds.Style("width", "'calc(20px + ('+($prayer_ticks - 1)/47+' * (100% - 40px)))'")),
							Div(Class("slider-ticks")),
							Div(Class("slider-thumb"),
								ds.Style("left", "'calc(20px + ('+($prayer_ticks - 1)/47+' * (100% - 40px)))'"),
								ds.Text("$prayer_ticks")),
						),
						Input(ID("prayer_ticks"), Class("slider-input"),
							Type("range"), Min("1"), Max("48"),
							Value("8"), ds.Bind("prayer_ticks")),
					),
					Span(Class("dur-unit"), Text("ticks")),
				),
			),
			Div(Class("field-group target-perk-group"),
				Label(Class("field-label"), Text("Target")),
				Div(Class("target-perk"),
					Icon("crown", 14, false),
					Text("Cross-kingdom targeting awaits a perk — prayers fall upon your own realm."),
				),
			),
			Input(Type("hidden"), ds.Bind("target_kingdom"), Value(kingdom.Name)),
			Div(Class("offering-form-foot"),
				Div(Class("offer-cost"),
					Span(Class("offer-cost-lbl"), Text("Total devotion")),
					// 20 = current per-tick upkeep for every prayer; see func comment.
					DynamicCostPill(
						[]DynamicCostEntry{{Resource: "devotion", Expr: "$prayer_ticks * 20"}},
						WithGemSize(20), WithSignalAvailability(),
					),
				),
				Button(Type("submit"), Class("btn btn--primary"), Text("Offer prayer")),
			),
		),
	)
}

func availablePrayersSection(active []db.KingdomPrayer, prayerKeys []string) Node {
	activeTypes := make(map[string]bool, len(active))
	for _, p := range active {
		activeTypes[p.PrayerType] = true
	}
	capReached := len(active) >= maxPrayers

	cards := make([]Node, 0, len(prayerKeys))
	for _, key := range prayerKeys {
		cards = append(cards, availablePrayerCard(key, game.PrayerDefs[key],
			activeTypes[key], capReached))
	}

	return Group([]Node{
		SectionHeader("Available Prayers", fmt.Sprintf("%d prayers", len(prayerKeys))),
		Div(Class("prayer-grid"), Group(cards)),
	})
}

func availablePrayerCard(key string, def game.PrayerDef, isActive, capReached bool) Node {
	cls := "prayer-card card"
	if isActive {
		cls += " is-active"
	} else if capReached {
		cls += " is-locked"
	}

	var foot Node
	switch {
	case isActive:
		foot = Div(Class("prayer-card-foot"),
			Span(Class("prayer-burning"),
				Icon("sandglass", 16, false),
				Text("Burning in the Sanctum"),
			),
		)
	case capReached:
		foot = Div(Class("prayer-card-foot"),
			Span(Class("cap-note"), Text("The altar holds one prayer at a time — cancel the burning prayer to offer another.")),
		)
	default:
		// Selecting a card sets the offering form's dropdown to that prayer
		// (and resets the duration) so the user confirms via the Sanctum form.
		expr := fmt.Sprintf("$prayer_type='%s';$prayer_ticks=8", key)
		foot = Div(Class("prayer-card-foot"),
			Button(Class("btn"), ds.On("click", expr), Text("Select prayer")),
		)
	}

	return Div(Class(cls),
		Div(Class("prayer-card-head"),
			Span(Class("prayer-medallion"), ResourceGem(prayerGemID(def), 22)),
			Span(Class("prayer-card-name"), Text(def.Name)),
		),
		P(Class("prayer-flavour"), Text(def.Description)),
		Div(Class("prayer-stats"),
			// Production modifier: gem + "+N%" + sandglass/arrow-up. The +%
			// distinguishes a per-tick multiplier from an absolute amount
			// (glossary: Production Rate / Upkeep). Prayers with no resource
			// bonus render nil here (StaticCostPill skips all-zero amounts).
			StaticCostPill(
				game.ResourceValues(def.ResourceBonusPct),
				WithGemSize(18),
				WithCostKind(CostProduction),
				WithPercent(),
			),
			// Devotion upkeep: gem + amount + sandglass/arrow-down.
			StaticCostPill(
				game.ResourceValues{Devotion: def.DevotionUpkeep},
				WithGemSize(18),
				WithCostKind(CostUpkeep),
			),
		),
		foot,
	)
}

// prayerResKey returns the sprite resource-symbol key (wood/stone/food/mana/
// devotion/knowledge) for a prayer's boosted resource, defaulting to "devotion".
func prayerResKey(def game.PrayerDef) string {
	bonus := game.ResourceValues(def.ResourceBonusPct)
	for _, res := range game.ResourceOrder {
		if bonus.Amount(res) > 0 {
			return res
		}
	}
	return "devotion"
}

// prayerGemID returns the gem colour id for a prayer's boosted resource.
func prayerGemID(def game.PrayerDef) string {
	return GemIDForResource(prayerResKey(def))
}

// effectHTML returns the bonus text wrapped so the boosted resource reads as an
// ink-stroked <b>.
func effectHTML(def game.PrayerDef) string {
	bonus := game.ResourceValues(def.ResourceBonusPct)
	var parts []string
	for _, res := range game.ResourceOrder {
		if n := bonus.Amount(res); n > 0 {
			parts = append(parts, fmt.Sprintf("<b>+%d%%</b> %s", n, res))
		}
	}
	if len(parts) == 0 {
		return "No resource bonus"
	}
	return strings.Join(parts, " · ")
}

func prayerAlert(inner Node) Node { return AlertContainer("prayer-alert", inner) }
