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
	prayerKeys := make([]string, 0, len(game.PrayerDefs))
	for k := range game.PrayerDefs {
		prayerKeys = append(prayerKeys, k)
	}
	sort.Strings(prayerKeys)

	return Group([]Node{
		Div(ds.Init(GetSSENoSignals(routes.KingdomPrayersRefreshPath))),
		ds.Signals(map[string]any{
			"prayer_type":    game.PrayerManaPrayer,
			"prayer_ticks":   8,
			"target_kingdom": kingdom.Name,
		}, ds.ModifierIfMissing),
		prayerAlert(nil),
		PageHeader("Prayers"),
		devotionLedger(kingdom, prayers),
		sanctumSection(kingdom, prayers, prayerKeys),
		availablePrayersSection(prayers, prayerKeys),
	})
}

// devotionLedger is the altar's reckoning strip: the kingdom's current devotion
// stock (sun gem) and, while a rite burns, its per-tick drain.
func devotionLedger(kingdom db.Kingdom, prayers []db.KingdomPrayer) Node {
	items := []Node{
		Span(Class("ledger-item"),
			ResourceGem("sun", 22),
			Span(Class("ledger-val"), Text(strconv.Itoa(kingdom.Devotion))),
			Span(Class("ledger-lbl"), Text("Devotion")),
		),
	}
	if len(prayers) > 0 {
		drain := 0
		for _, p := range prayers {
			if def, ok := game.PrayerDefs[p.PrayerType]; ok {
				drain += def.DevotionUpkeep
			}
		}
		items = append(items,
			Span(Class("ledger-sep")),
			Span(Class("ledger-item"),
				Icon("sandglass", 18, false),
				Span(Class("ledger-val is-drain"), Text(fmt.Sprintf("−%d", drain))),
				Span(Class("ledger-lbl"), Text("Per tick")),
			),
		)
	}
	return Div(Class("devotion-ledger"), Group(items))
}

// sanctumSection is the altar card. When a rite is burning it shows the active
// rite with a brass tick meter and a cancel button; when the altar is free it
// shows the offering form (rite · duration stepper · cost · Pray).
func sanctumSection(kingdom db.Kingdom, prayers []db.KingdomPrayer, prayerKeys []string) Node {
	busy := len(prayers) > 0
	used := len(prayers)
	if used > maxPrayers {
		used = maxPrayers
	}

	body := Iff(busy, func() Node { return activeRiteView(prayers[0]) })
	if !busy {
		body = offeringForm(kingdom, prayerKeys)
	}

	return Div(Class("card sanctum"),
		Div(Class("card-inner"),
			Div(Class("card-header-row"),
				P(Class("card-title"), Text("The Sanctum")),
				Div(Class("slot-gauge"),
					Div(Class("slot-gauge-dots"),
						Range(maxPrayers, func(i int) Node {
							return Div(Classes{"slot-gauge-dot": true, "is-on": i < used})
						}),
					),
					Span(Class("slot-gauge-label"),
						Text(fmt.Sprintf("%d/%d in use", used, maxPrayers))),
				),
			),
			body,
		),
	)
}

// Range emits n zero-indexed child nodes via fn.
func Range(n int, fn func(i int) Node) Group {
	nodes := make([]Node, 0, n)
	for i := range n {
		nodes = append(nodes, fn(i))
	}
	return Group(nodes)
}

// activeRiteView renders the single burning rite: glyph + name/effect + drain,
// a brass tick meter, and a cancel footer.
func activeRiteView(p db.KingdomPrayer) Node {
	def, ok := game.PrayerDefs[p.PrayerType]
	name, effect, upkeep := p.PrayerType, "", 0
	gemID := "sun"
	if ok {
		name = def.Name
		effect = resourceBonusText(def)
		upkeep = def.DevotionUpkeep
		gemID = prayerGemID(def)
	}

	fillPct := 0.0
	if p.TicksTotal > 0 {
		fillPct = float64(p.TicksTotal-p.TicksRemaining) / float64(p.TicksTotal) * 100
	}
	notches := make([]Node, 0, p.TicksTotal)
	for range int(p.TicksTotal) {
		notches = append(notches, Span(Class("meter-notch")))
	}

	return Div(Class("rite-active"),
		Div(Class("rite-top"),
			ResourceGem(gemID, 44),
			Div(Class("rite-id"),
				Span(Class("rite-target"),
					Icon("crown", 13, false),
					Text("Upon your realm"),
				),
				Span(Class("rite-name"), Text(name)),
				Iff(effect != "", func() Node {
					return Span(Class("rite-effect"), Raw(effectHTML(def)))
				}),
			),
			Div(Class("rite-aside"),
				Div(Class("rite-drain"),
					Span(Class("rite-drain-lbl"), Text("Devotion drain")),
					Span(Class("stat-pill"),
						Span(Classes{"pill-neg": true}, Text(fmt.Sprintf("−%d", upkeep))),
						Text("/tick"),
					),
				),
			),
		),
		Div(Class("meter"),
			Div(Class("meter-top"),
				Span(Class("meter-name"), Text("Time burning")),
				Span(Class("meter-eta"),
					Text(fmt.Sprintf("%d of %d ticks left", p.TicksRemaining, p.TicksTotal))),
			),
			Div(Class("meter-track"),
				Div(Class("meter-fill"), Style(fmt.Sprintf("width:%.1f%%", fillPct))),
				Div(Class("meter-notches"), Group(notches)),
			),
		),
		Div(Class("rite-foot"),
			P(Class("rite-foot-note"),
				Text("The rite drains devotion each tick and ends when it runs dry or the ticks elapse.")),
			Button(Class("btn btn--danger"),
				ds.On("click", datastar.PostSSE("%s", cancelPrayerPath(p.ID))),
				Text("Cancel rite"),
			),
		),
	)
}

// offeringForm renders the cast form when the altar is free. The duration
// stepper is driven by the $prayer_ticks signal; the total-cost readout is a
// datastar expression (every current rite costs 20 devotion/tick, so the
// multiplier is fixed — update here if a future rite changes that).
func offeringForm(kingdom db.Kingdom, prayerKeys []string) Node {
	return Div(Class("offering"),
		P(Class("offering-lede"),
			Text("The altar stands open. Choose a rite and the devotion you will spend per tick."),
		),
		Form(Class("offering-form"),
			ds.On("submit", datastar.PostSSE(routes.KingdomPrayerCastPath)),
			Div(Class("field-group"),
				Label(For("prayer_type"), Class("field-label"), Text("Rite")),
				Select(ID("prayer_type"), Class("select"), ds.Bind("prayer_type"),
					Group(Map(prayerKeys, func(key string) Node {
						return Option(Value(key), Text(game.PrayerDefs[key].Name))
					})),
				),
			),
			Div(Class("field-group"),
				Label(For("prayer_ticks"), Class("field-label"), Text("Duration")),
				Div(Class("dur-rod"),
					Div(Class("slider-wrap"),
						Div(Class("slider-track"),
							Div(Class("slider-fill"),
								ds.Style("width", "'calc(20px + ('+($prayer_ticks-1)/47+' * (100% - 40px)))'")),
							Div(Class("slider-ticks")),
							Div(Class("slider-thumb v-badge"),
								ds.Style("left", "'calc(20px + ('+($prayer_ticks-1)/47+' * (100% - 40px)))'"),
								ds.Text("$prayer_ticks")),
						),
						Input(ID("prayer_ticks"), Class("slider-input"),
							Type("range"), Min("1"), Max("48"),
							Value("8"), ds.Bind("prayer_ticks")),
					),
					Span(Class("dur-unit"), Text("ticks")),
				),
				Div(Class("dur-presets"),
					presetChip(8), presetChip(16), presetChip(24), presetChip(48),
				),
			),
			Div(Class("field-group target-perk-group"),
				Label(Class("field-label"), Text("Target")),
				Div(Class("target-perk"),
					Icon("crown", 14, false),
					Text("Cross-kingdom targeting awaits a perk — rites fall upon your own realm."),
				),
			),
			Input(Type("hidden"), ds.Bind("target_kingdom"), Value(kingdom.Name)),
			Div(Class("offering-form-foot"),
				Div(Class("offer-cost"),
					Span(Class("offer-cost-lbl"), Text("Total devotion")),
					Span(Class("offer-cost-val"),
						ResourceGem("sun", 20),
						// 20 = current per-tick upkeep for every rite; see func comment.
						ds.Text("$prayer_ticks * 20"),
					),
				),
				Button(Type("submit"), Class("btn btn--primary"), Text("Offer rite")),
			),
		),
	)
}

func presetChip(ticks int) Node {
	return Span(Classes{"dur-chip": true},
		ds.On("click", fmt.Sprintf("$prayer_ticks = %d", ticks)),
		Role("button"),
		Text(strconv.Itoa(ticks)),
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
		Div(Class("section-header"),
			Span(Class("section-title"), Text("Available Prayers")),
			Span(Class("section-rule")),
			Span(Class("section-meta"),
				Text(fmt.Sprintf("%d rites", len(prayerKeys)))),
		),
		Div(Class("prayer-grid"), Group(cards)),
	})
}

func availablePrayerCard(key string, def game.PrayerDef, isActive, capReached bool) Node {
	cls := "prayer-card card"
	if isActive {
		cls += " is-active"
	} else if capReached {
		cls += " is-sealed"
	}

	var foot Node
	switch {
	case isActive:
		foot = Div(Class("prayer-card-foot"),
			Span(Class("rite-burning"),
				Icon("sandglass", 16, false),
				Text("Burning in the Sanctum"),
			),
		)
	case capReached:
		foot = Div(Class("prayer-card-foot"),
			Span(Class("cap-note"), Text("The altar holds one rite at a time — cancel the burning rite to offer another.")),
		)
	default:
		expr := fmt.Sprintf("$prayer_type='%s';$prayer_ticks=8;%s",
			key, datastar.PostSSE(routes.KingdomPrayerCastPath))
		foot = Div(Class("prayer-card-foot"),
			Button(Class("btn btn--primary"), ds.On("click", expr), Text("Offer rite")),
		)
	}

	return Div(Class(cls),
		Div(Class("prayer-card-head"),
			Span(Class("seal-glyph"), ResourceGem(prayerGemID(def), 22)),
			Span(Class("prayer-card-name"), Text(def.Name)),
		),
		P(Class("prayer-flavour"), Text(def.Description)),
		Div(Class("prayer-stats"),
			Span(Class("prayer-effect"),
				Icon("res-"+prayerResKey(def), 16, false),
				Raw(effectHTML(def)),
			),
			Span(Class("prayer-upkeep"),
				Icon("sandglass", 16, false),
				Text(fmt.Sprintf("%d devotion/tick", def.DevotionUpkeep)),
			),
		),
		foot,
	)
}

// prayerResKey returns the sprite resource-symbol key (wood/stone/food/mana/
// devotion/knowledge) for a prayer's boosted resource, defaulting to "devotion".
func prayerResKey(def game.PrayerDef) string {
	switch {
	case def.ResourceBonusPct.Wood > 0:
		return "wood"
	case def.ResourceBonusPct.Stone > 0:
		return "stone"
	case def.ResourceBonusPct.Food > 0:
		return "food"
	case def.ResourceBonusPct.Mana > 0:
		return "mana"
	case def.ResourceBonusPct.Knowledge > 0:
		return "knowledge"
	default:
		return "devotion"
	}
}

// resKeyToGemID maps a resource symbol key to its gem colour id.
var resKeyToGemID = map[string]string{
	"wood": "tree", "stone": "mountain", "food": "wheat",
	"mana": "flame", "devotion": "sun", "knowledge": "star",
}

// prayerGemID returns the gem colour id for a prayer's boosted resource.
func prayerGemID(def game.PrayerDef) string {
	return resKeyToGemID[prayerResKey(def)]
}

// effectHTML returns the bonus text wrapped so the boosted resource reads as an
// ink-stroked <b>. It mirrors resourceBonusText but allows inline emphasis.
func effectHTML(def game.PrayerDef) string {
	b := def.ResourceBonusPct
	var parts []string
	if b.Wood > 0 {
		parts = append(parts, fmt.Sprintf("<b>+%d%%</b> wood", b.Wood))
	}
	if b.Stone > 0 {
		parts = append(parts, fmt.Sprintf("<b>+%d%%</b> stone", b.Stone))
	}
	if b.Food > 0 {
		parts = append(parts, fmt.Sprintf("<b>+%d%%</b> food", b.Food))
	}
	if b.Mana > 0 {
		parts = append(parts, fmt.Sprintf("<b>+%d%%</b> mana", b.Mana))
	}
	if b.Devotion > 0 {
		parts = append(parts, fmt.Sprintf("<b>+%d%%</b> devotion", b.Devotion))
	}
	if b.Knowledge > 0 {
		parts = append(parts, fmt.Sprintf("<b>+%d%%</b> knowledge", b.Knowledge))
	}
	if len(parts) == 0 {
		return "No resource bonus"
	}
	return strings.Join(parts, " · ")
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
