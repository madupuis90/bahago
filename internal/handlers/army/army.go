package army

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

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
	. "bahago/internal/layout"
	"bahago/internal/router"
	"bahago/internal/routes"
)

// ── Input structs ─────────────────────────────────────────────────────────────

type sendInput struct {
	UnitType      string `json:"unit_type"`
	SendCount     int    `json:"send_count"`
	Action        string `json:"action"`
	TargetName    string `json:"target_name"`
	DurationTicks int    `json:"duration_ticks"`
}

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier, pool *pgxpool.Pool, tickHub *hub.Hub) {
	h := &handler{queries: queries, pool: pool, hub: tickHub}
	r.HandleFunc("GET "+routes.KingdomArmyPath, h.handleArmyPage())
	r.HandleFunc("GET "+routes.KingdomArmyRefreshPath, h.handleArmyRefresh())
	r.HandleFunc("POST "+routes.KingdomArmySendPath, h.handleSend())
	r.HandleFunc("POST "+routes.KingdomArmyCancelPath+"/{id}", h.handleCancel())
}

type handler struct {
	queries db.Querier
	pool    *pgxpool.Pool
	hub     *hub.Hub
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleArmyPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		data, err := h.loadArmyData(r, kingdom.ID)
		if err != nil {
			log.Printf("army page: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		targetName := r.URL.Query().Get("target")
		action := r.URL.Query().Get("action")
		if action != "attack" && action != "defend" {
			action = "attack"
		}
		KingdomLayout(r, "Army", routes.KingdomArmyPath, kingdom, armyContent(kingdom, data, targetName, action)).Render(w)
	}
}

func (h *handler) handleArmyRefresh() http.HandlerFunc {
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
				data, err := h.loadArmyData(r, k.ID)
				if err != nil {
					log.Printf("army refresh: %v", err)
					return
				}
				page := armyContent(&k, data, "", "attack")
				if err := sse.PatchElementGostar(MainContent(page)); err != nil {
					log.Printf("army refresh: patch: %v", err)
					return
				}
			}
		}
	}
}

func (h *handler) handleSend() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		input := &sendInput{}
		if err := datastar.ReadSignals(r, input); err != nil {

			log.Printf("army send: read signals: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("invalid request")))
			return
		}
		if _, ok := game.UnitDefs[input.UnitType]; !ok {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("unknown unit type")))
			return
		}

		if input.SendCount <= 0 {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("count must be at least 1")))
			return
		}
		if input.SendCount > game.MaxUnitInput {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("count is too large")))
			return
		}

		if input.Action != "attack" && input.Action != "defend" {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("invalid action")))
			return
		}

		maxDuration := 20
		if input.Action == "defend" {
			maxDuration = 96
		}
		if input.DurationTicks < 4 || input.DurationTicks > maxDuration || input.DurationTicks%4 != 0 {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("invalid duration")))
			return
		}

		if input.TargetName == "" {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("target kingdom name is required")))
			return
		}

		target, err := h.queries.GetKingdomByName(r.Context(), input.TargetName)
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(fmt.Errorf("kingdom %q not found", input.TargetName)))
			return
		}

		if target.ID == kingdom.ID {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("cannot target your own kingdom")))
			return
		}

		travelTicks := game.TravelTicks(kingdom.X, kingdom.Y, target.X, target.Y)

		params := db.CreateCampaignIfAvailableParams{
			KingdomID:       kingdom.ID,
			TargetKingdomID: target.ID,
			UnitType:        input.UnitType,
			SendCount:       input.SendCount,
			Action:          input.Action,
			TicksRemaining:  travelTicks,
			ActionTicks:     input.DurationTicks,
			TravelTicks:     travelTicks,
		}
		tx, err := h.pool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			log.Printf("army send: begin tx: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("internal error")))
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck
		_, err = db.New(tx).CreateCampaignIfAvailable(r.Context(), params)
		if errors.Is(err, pgx.ErrNoRows) {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("not enough units")))
			return
		}
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.SerializationFailure {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("not enough units")))
			return
		}
		if err != nil {
			log.Printf("army send: create campaign: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("internal error")))
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			log.Printf("army send: commit: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("internal error")))
			return
		}

		data, err := h.loadArmyData(r, kingdom.ID)
		if err != nil {
			log.Printf("army send: reload data: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("internal error")))
			return
		}
		sse := datastar.NewSSE(w, r)
		sse.MarshalAndPatchSignals(map[string]any{"unit_type": firstAvailableUnit(data), "send_count": 1})
		page := armyContent(kingdom, data, "", "attack")
		sse.PatchElementGostar(MainContent(page))
	}
}

func (h *handler) handleCancel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		idStr := r.PathValue("id")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("invalid campaign id")))
			return
		}

		_, err = h.queries.CancelCampaign(r.Context(), db.CancelCampaignParams{
			ID:        id,
			KingdomID: kingdom.ID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("campaign not found or already returning")))
			return
		}
		if err != nil {
			log.Printf("army cancel: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyError(errors.New("internal error")))
			return
		}

		if err := datastar.NewSSE(w, r).Redirect(routes.KingdomArmyPath); err != nil {
			log.Printf("army cancel: redirect: %v", err)
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

type armyData struct {
	availableUnits []db.GetAvailableKingdomUnitsRow
	campaigns      []db.KingdomCampaign
	others         []db.ListOtherKingdomsRow
}

func (h *handler) loadArmyData(r *http.Request, kingdomID int) (armyData, error) {
	availableUnits, err := h.queries.GetAvailableKingdomUnits(r.Context(), kingdomID)
	if err != nil {
		return armyData{}, fmt.Errorf("get available units: %w", err)
	}
	campaigns, err := h.queries.GetCampaignsForKingdom(r.Context(), kingdomID)
	if err != nil {
		return armyData{}, fmt.Errorf("get campaigns: %w", err)
	}
	others, err := h.queries.ListOtherKingdoms(r.Context(), kingdomID)
	if err != nil {
		return armyData{}, fmt.Errorf("list kingdoms: %w", err)
	}
	return armyData{availableUnits: availableUnits, campaigns: campaigns, others: others}, nil
}

// ── Components ────────────────────────────────────────────────────────────────

func firstAvailableUnit(data armyData) string {
	availableSet := make(map[string]bool, len(data.availableUnits))
	for _, u := range data.availableUnits {
		if u.Count > 0 {
			availableSet[u.UnitType] = true
		}
	}
	for _, utype := range append(append([]string{}, game.UnitOrder...), game.SummonOrder...) {
		if availableSet[utype] {
			return utype
		}
	}
	return ""
}

func armyContent(kingdom *db.Kingdom, data armyData, targetName, action string) Node {
	otherIndex := make(map[int]string, len(data.others))
	for _, o := range data.others {
		otherIndex[o.ID] = o.Name
	}

	availableSet := make(map[string]bool, len(data.availableUnits))
	for _, u := range data.availableUnits {
		if u.Count > 0 {
			availableSet[u.UnitType] = true
		}
	}

	allOrdered := append(append([]string{}, game.UnitOrder...), game.SummonOrder...)
	firstUnit := firstAvailableUnit(data)

	return Div(
		H1(Class("page-title"), Text("Army")),
		ds.Signals(map[string]any{
			"unit_type":      firstUnit,
			"send_count":     1,
			"action":         action,
			"target_name":    targetName,
			"duration_ticks": 4,
		}, ds.ModifierIfMissing),
		Div(ds.Init(GetSSENoSignals(routes.KingdomArmyRefreshPath))),
		armyError(nil),
		campaignsSection(data.campaigns, otherIndex),
		armyUnitsSection(data.availableUnits),
		sendForm(allOrdered, availableSet),
	)
}

func campaignsSection(campaigns []db.KingdomCampaign, otherIndex map[int]string) Node {
	return Div(Class("army-section panel"),
		P(Class("panel-title"), Text("Active Campaigns")),
		Iff(len(campaigns) == 0, func() Node {
			return P(Class("army-empty"), Text("No active campaigns."))
		}),
		Iff(len(campaigns) > 0, func() Node {
			return Table(Class("army-table"),
				THead(Tr(
					Th(Text("Unit")),
					Th(Text("Sent")),
					Th(Text("Action")),
					Th(Text("Target")),
					Th(Text("Status")),
					Th(Text("Phase ends in")),
					Th(Text("Returns in")),
					Th(Text("")),
				)),
				TBody(Group(Map(campaigns, func(m db.KingdomCampaign) Node {
					return campaignRow(m, otherIndex)
				}))),
			)
		}),
	)
}

func campaignRow(m db.KingdomCampaign, otherIndex map[int]string) Node {
	statusLabel := campaignStatusLabel(m.Status, m.Action)
	eta := campaignETA(m)
	targetName := otherIndex[m.TargetKingdomID]
	if targetName == "" {
		targetName = strconv.Itoa(m.TargetKingdomID)
	}
	unitDef := game.UnitDefs[m.UnitType]

	canCancel := m.Status != "returning"

	return Tr(
		Td(Text(unitDef.Name)),
		Td(Text(strconv.Itoa(m.Count))),
		Td(Text(m.Action)),
		Td(Text(targetName)),
		Td(Text(statusLabel)),
		Td(Text(fmt.Sprintf("%d ticks", m.TicksRemaining))),
		Td(Text(fmt.Sprintf("%d ticks", eta))),
		Td(
			Iff(canCancel, func() Node {
				return Button(
					Class("btn btn-text"),
					ds.On("click", datastar.PostSSE(routes.KingdomArmyCancelPath+"/%d", m.ID)),
					Text("Cancel"),
				)
			}),
		),
	)
}

func campaignStatusLabel(status, action string) string {
	switch status {
	case "en_route":
		return "En route"
	case "active":
		if action == "attack" {
			return "Attacking"
		}
		return "Defending"
	case "returning":
		return "Returning"
	}
	return status
}

// campaignETA returns the number of ticks until the campaign's units return home.
func campaignETA(m db.KingdomCampaign) int {
	switch m.Status {
	case "en_route":
		return m.TicksRemaining + m.ActionTicks + m.TravelTicks
	case "active":
		return m.TicksRemaining + m.TravelTicks
	case "returning":
		return m.TicksRemaining
	}
	return 0
}

func armyUnitsSection(units []db.GetAvailableKingdomUnitsRow) Node {
	return Div(Class("army-section panel"),
		P(Class("panel-title"), Text("Available Units")),
		Iff(len(units) == 0, func() Node {
			return P(Class("army-empty"), Text("No units available."))
		}),
		Iff(len(units) > 0, func() Node {
			return Table(Class("army-table"),
				THead(Tr(
					Th(Text("Unit")),
					Th(Text("Available")),
					Th(Text("Power")),
				)),
				TBody(Group(Map(units, func(u db.GetAvailableKingdomUnitsRow) Node {
					def := game.UnitDefs[u.UnitType]
					return Tr(
						Td(Text(def.Name)),
						Td(Text(strconv.Itoa(u.Count))),
						Td(Text(strconv.Itoa(def.Power))),
					)
				}))),
			)
		}),
	)
}

func sendForm(allOrdered []string, ownedSet map[string]bool) Node {
	attackOptions := durationOptions(4, 20, 4)
	defendOptions := durationOptions(4, 96, 4)

	return Div(Class("army-section panel"),
		P(Class("panel-title"), Text("Send Units")),
		Div(Class("army-form"),
			Label(For("army-unit-type"), Text("Unit type")),
			Select(
				ID("army-unit-type"),
				ds.Bind("unit_type"),
				Group(Map(allOrdered, func(utype string) Node {
					return Iff(ownedSet[utype], func() Node {
						return Option(Value(utype), Text(game.UnitDefs[utype].Name))
					})
				})),
			),

			Label(For("army-count"), Text("Count")),
			Input(
				ID("army-count"),
				Type("number"),
				Min("1"),
				Value("1"),
				ds.Bind("send_count"),
			),

			Label(For("army-action"), Text("Action")),
			Select(
				ID("army-action"),
				ds.Bind("action"),
				ds.On("change", "$duration_ticks = 4"),
				Option(Value("attack"), Text("Attack")),
				Option(Value("defend"), Text("Defend")),
			),

			Label(For("army-duration"), Text("Duration")),
			Div(
				Select(
					ID("army-duration"),
					ds.Bind("duration_ticks"),
					ds.Show(`$action === 'attack'`),
					Group(attackOptions),
				),
				Select(
					ds.Bind("duration_ticks"),
					ds.Show(`$action === 'defend'`),
					Group(defendOptions),
				),
			),

			Label(For("army-target"), Text("Target kingdom")),
			Input(
				ID("army-target"),
				Type("text"),
				Placeholder("Kingdom name"),
				ds.Bind("target_name"),
			),

			Button(
				Class("btn"),
				ds.On("click", datastar.PostSSE(routes.KingdomArmySendPath)),
				Text("Send"),
			),
		),
	)
}

func durationOptions(min, max, step int) []Node {
	var opts []Node
	for t := min; t <= max; t += step {
		hours := t / 4
		label := fmt.Sprintf("%d ticks (%dh)", t, hours)
		opts = append(opts, Option(Value(strconv.Itoa(t)), Text(label)))
	}
	return opts
}

func armyError(err error) Node {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return Div(
		ID("army-alert"),
		Classes{"alert--error": err != nil},
		Text(msg),
	)
}
