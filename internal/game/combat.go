package game

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"bahago/internal/database/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CombatUnitRecord captures the aggregate state of one unit type in a combat round,
// summed across all kingdoms contributing that type on the same side.
// Stored as JSON in the kingdom_combat_log attacker_units / defender_units columns.
type CombatUnitRecord struct {
	UnitType   string `json:"unit_type"`
	Count      int    `json:"count"`      // pre-combat effective count
	Power      int    `json:"power"`      // total base power
	Casualties int    `json:"casualties"` // units lost this round
}

// CombatParticipant records a kingdom's role and population outcome in a combat round.
// Stored in kingdom_combat_log_participants to link kingdoms to the shared log entry.
type CombatParticipant struct {
	KingdomID        int
	Role             string // "attacker" or "defender"
	PopulationGained int    // share of stolen population; 0 for defenders
}

// CombatResult captures the full outcome of one combat round.
// Returned by resolveCombatAtKingdom for insertion into kingdom_combat_log.
type CombatResult struct {
	TargetKingdomID    int
	AttackerUnits      []CombatUnitRecord
	DefenderUnits      []CombatUnitRecord
	AttackerPower      int
	DefenderPower      int
	Winner             string // "attacker" or "defender"
	AttackerCasualties int
	DefenderCasualties int
	PopulationStolen   int
	Participants       []CombatParticipant
}

// totalUnitPower returns the base combat power for a number of units of the given type.
func totalUnitPower(unitType string, count int) int {
	unit, ok := UnitDefs[unitType]
	if !ok {
		log.Printf("combat: unknown unit type %q — no power contribution", unitType)
		return 0
	}
	return unit.Power * count
}

// computeLossRatios returns the fraction of units each side should lose given the
// two power totals. Both ratios are zero when either side has no power (no combat).
// Each ratio is capped at 0.9 so the losing side is never fully wiped in one round.
func computeLossRatios(atkPow, defPow int) (atkLoss, defLoss float64) {
	if atkPow > 0 && defPow > 0 {
		atkLoss = min(float64(defPow)/float64(atkPow)*0.3, 0.9)
		defLoss = min(float64(atkPow)/float64(defPow)*0.3, 0.9)
	}
	return
}

// casKey identifies a (kingdom, unit type) pair for aggregating kingdom_units deductions.
type casKey struct {
	kingdomID int
	unitType  string
}

// resolveCombatAtKingdom runs one combat round between all attackers and defenders
// at the given kingdom. campaignUnitsByID holds unit composition for every campaign.
// atHomeLegionUnits holds all at-home legion units for the target kingdom.
// All data is pre-fetched by the caller; this function only performs writes.
// Returns nil when there is no power on either side and combat is skipped.
func resolveCombatAtKingdom(
	ctx context.Context,
	q db.Querier,
	targetKingdom db.Kingdom,
	targetAvailableUnits []db.KingdomAvailableUnit,
	atHomeLegionUnits []db.GetAtHomeLegionUnitsByKingdomIDsRow,
	attackers []db.KingdomCampaign,
	defenders []db.KingdomCampaign,
	campaignUnitsByID map[int][]db.KingdomCampaignUnit,
) (*CombatResult, error) {
	// Compute attacker power across all attacking campaigns' unit types.
	var atkPow int
	for _, c := range attackers {
		for _, u := range campaignUnitsByID[c.ID] {
			atkPow += totalUnitPower(u.UnitType, u.Count)
		}
	}

	// Compute defender power: reserve + at-home legions + campaign defenders.
	var defPow int
	for _, u := range targetAvailableUnits {
		defPow += totalUnitPower(u.UnitType, u.Count)
	}
	for _, u := range atHomeLegionUnits {
		defPow += totalUnitPower(u.UnitType, u.Count)
	}
	for _, c := range defenders {
		for _, u := range campaignUnitsByID[c.ID] {
			defPow += totalUnitPower(u.UnitType, u.Count)
		}
	}

	if atkPow == 0 && defPow == 0 {
		return nil, nil
	}

	atkLoss, defLoss := computeLossRatios(atkPow, defPow)

	// ── Phase 1: compute outcomes ─────────────────────────────────────────────

	// casMap aggregates kingdom_units deductions by (kingdom_id, unit_type).
	casMap := make(map[casKey]int)

	// Pending campaign unit row writes.
	type campaignUnitRow struct {
		campaignID int
		unitType   string
		count      int
	}
	var campUpdates []campaignUnitRow // surviving rows (count > 0)
	var campZeros []campaignUnitRow   // wiped rows (count == 0)
	campHadUnits := make(map[int]bool)
	campHasSurvivors := make(map[int]bool)

	atkByType := make(map[string]*CombatUnitRecord)
	var totalAtkCasualties int

	for _, c := range attackers {
		for _, u := range campaignUnitsByID[c.ID] {
			campHadUnits[c.ID] = true
			survivors := max(0, int(float64(u.Count)*(1-atkLoss)))
			casualties := u.Count - survivors
			if survivors > 0 {
				campUpdates = append(campUpdates, campaignUnitRow{c.ID, u.UnitType, survivors})
				campHasSurvivors[c.ID] = true
			} else {
				campZeros = append(campZeros, campaignUnitRow{c.ID, u.UnitType, 0})
			}
			if casualties > 0 {
				casMap[casKey{c.KingdomID, u.UnitType}] += casualties
			}
			totalAtkCasualties += casualties

			r := atkByType[u.UnitType]
			if r == nil {
				r = &CombatUnitRecord{UnitType: u.UnitType}
				atkByType[u.UnitType] = r
			}
			r.Count += u.Count
			r.Power += totalUnitPower(u.UnitType, u.Count)
			r.Casualties += casualties
		}
	}

	defByType := make(map[string]*CombatUnitRecord)
	var totalDefCasualties int

	// Reserve defenders — casualties only deducted from kingdom_units.
	for _, u := range targetAvailableUnits {
		unitPow := totalUnitPower(u.UnitType, u.Count)
		casualties := 0
		if defLoss > 0 && defPow > 0 {
			casualties = int(float64(u.Count) * defLoss * float64(unitPow) / float64(defPow))
		}
		if casualties > 0 {
			casMap[casKey{targetKingdom.ID, u.UnitType}] += casualties
		}
		totalDefCasualties += casualties

		r := defByType[u.UnitType]
		if r == nil {
			r = &CombatUnitRecord{UnitType: u.UnitType}
			defByType[u.UnitType] = r
		}
		r.Count += u.Count
		r.Power += unitPow
		r.Casualties += casualties
	}

	// At-home legion defenders — casualties deducted from both kingdom_legion_units and kingdom_units.
	type legionUnitRow struct {
		legionID int
		unitType string
		count    int
	}
	var legionUpdates []legionUnitRow
	var legionZeros []legionUnitRow

	for _, u := range atHomeLegionUnits {
		unitPow := totalUnitPower(u.UnitType, u.Count)
		casualties := 0
		if defLoss > 0 && defPow > 0 {
			casualties = int(float64(u.Count) * defLoss * float64(unitPow) / float64(defPow))
		}
		survivors := max(0, u.Count-casualties)
		if survivors > 0 {
			legionUpdates = append(legionUpdates, legionUnitRow{u.LegionID, u.UnitType, survivors})
		} else {
			legionZeros = append(legionZeros, legionUnitRow{u.LegionID, u.UnitType, 0})
		}
		if casualties > 0 {
			casMap[casKey{targetKingdom.ID, u.UnitType}] += casualties
		}
		totalDefCasualties += casualties

		r := defByType[u.UnitType]
		if r == nil {
			r = &CombatUnitRecord{UnitType: u.UnitType}
			defByType[u.UnitType] = r
		}
		r.Count += u.Count
		r.Power += unitPow
		r.Casualties += casualties
	}

	// Campaign defenders — casualties deducted from both kingdom_campaign_units and kingdom_units.
	for _, c := range defenders {
		for _, u := range campaignUnitsByID[c.ID] {
			campHadUnits[c.ID] = true
			unitPow := totalUnitPower(u.UnitType, u.Count)
			casualties := 0
			if defLoss > 0 && defPow > 0 {
				casualties = int(float64(u.Count) * defLoss * float64(unitPow) / float64(defPow))
			}
			survivors := max(0, u.Count-casualties)
			if survivors > 0 {
				campUpdates = append(campUpdates, campaignUnitRow{c.ID, u.UnitType, survivors})
				campHasSurvivors[c.ID] = true
			} else {
				campZeros = append(campZeros, campaignUnitRow{c.ID, u.UnitType, 0})
			}
			if casualties > 0 {
				casMap[casKey{c.KingdomID, u.UnitType}] += casualties
			}
			totalDefCasualties += casualties

			r := defByType[u.UnitType]
			if r == nil {
				r = &CombatUnitRecord{UnitType: u.UnitType}
				defByType[u.UnitType] = r
			}
			r.Count += u.Count
			r.Power += unitPow
			r.Casualties += casualties
		}
	}

	// Population stolen and per-attacker-kingdom distribution.
	var popStolen int
	gainByKingdom := make(map[int]int)
	if atkPow > defPow {
		ratio := float64(atkPow-defPow) / float64(atkPow+defPow)
		calculated := max(1, int(float64(targetKingdom.Population)*ratio*0.1))
		popStolen = min(calculated, max(0, targetKingdom.Population-100))
		for _, c := range attackers {
			for _, u := range campaignUnitsByID[c.ID] {
				share := int(float64(popStolen) * float64(totalUnitPower(u.UnitType, u.Count)) / float64(atkPow))
				if share > 0 {
					gainByKingdom[c.KingdomID] += share
				}
			}
		}
	}

	// ── Phase 2: writes ───────────────────────────────────────────────────────

	// Identify fully depleted campaigns (had units, none survived).
	var depleted []int
	for _, c := range attackers {
		if campHadUnits[c.ID] && !campHasSurvivors[c.ID] {
			depleted = append(depleted, c.ID)
		}
	}
	for _, c := range defenders {
		if campHadUnits[c.ID] && !campHasSurvivors[c.ID] {
			depleted = append(depleted, c.ID)
		}
	}
	depletedSet := make(map[int]bool, len(depleted))
	for _, id := range depleted {
		depletedSet[id] = true
	}

	// Update non-zero surviving campaign unit rows (skip depleted — cascade handles them).
	if len(campUpdates) > 0 {
		p := db.BulkUpdateCampaignUnitCountsParams{
			CampaignIds: make([]int, 0, len(campUpdates)),
			UnitTypes:   make([]string, 0, len(campUpdates)),
			Counts:      make([]int, 0, len(campUpdates)),
		}
		for _, u := range campUpdates {
			if !depletedSet[u.campaignID] {
				p.CampaignIds = append(p.CampaignIds, u.campaignID)
				p.UnitTypes = append(p.UnitTypes, u.unitType)
				p.Counts = append(p.Counts, u.count)
			}
		}
		if len(p.CampaignIds) > 0 {
			if err := q.BulkUpdateCampaignUnitCounts(ctx, p); err != nil {
				return nil, fmt.Errorf("bulk update campaign unit counts: %w", err)
			}
		}
	}

	// Delete zero-survivor rows for non-depleted campaigns.
	if len(campZeros) > 0 {
		p := db.BulkDeleteCampaignUnitsZeroParams{
			CampaignIds: make([]int, 0, len(campZeros)),
			UnitTypes:   make([]string, 0, len(campZeros)),
		}
		for _, u := range campZeros {
			if !depletedSet[u.campaignID] {
				p.CampaignIds = append(p.CampaignIds, u.campaignID)
				p.UnitTypes = append(p.UnitTypes, u.unitType)
			}
		}
		if len(p.CampaignIds) > 0 {
			if err := q.BulkDeleteCampaignUnitsZero(ctx, p); err != nil {
				return nil, fmt.Errorf("bulk delete zero campaign units: %w", err)
			}
		}
	}

	// Delete depleted campaigns; cascade removes their kingdom_campaign_units rows.
	if len(depleted) > 0 {
		if err := q.BulkDeleteCampaigns(ctx, depleted); err != nil {
			return nil, fmt.Errorf("delete depleted campaigns: %w", err)
		}
	}

	// Update non-zero surviving at-home legion unit rows.
	if len(legionUpdates) > 0 {
		p := db.BulkUpdateLegionUnitCountsParams{
			LegionIds: make([]int, len(legionUpdates)),
			UnitTypes: make([]string, len(legionUpdates)),
			Counts:    make([]int, len(legionUpdates)),
		}
		for i, u := range legionUpdates {
			p.LegionIds[i] = u.legionID
			p.UnitTypes[i] = u.unitType
			p.Counts[i] = u.count
		}
		if err := q.BulkUpdateLegionUnitCounts(ctx, p); err != nil {
			return nil, fmt.Errorf("bulk update legion unit counts: %w", err)
		}
	}

	// Delete zero-survivor at-home legion unit rows.
	if len(legionZeros) > 0 {
		p := db.BulkDeleteLegionUnitsZeroParams{
			LegionIds: make([]int, len(legionZeros)),
			UnitTypes: make([]string, len(legionZeros)),
		}
		for i, u := range legionZeros {
			p.LegionIds[i] = u.legionID
			p.UnitTypes[i] = u.unitType
		}
		if err := q.BulkDeleteLegionUnitsZero(ctx, p); err != nil {
			return nil, fmt.Errorf("bulk delete zero legion units: %w", err)
		}
	}

	// Deduct all casualties from kingdom_units.
	if len(casMap) > 0 {
		p := db.BulkDeductKingdomUnitsCasualtiesParams{
			KingdomIds: make([]int, 0, len(casMap)),
			UnitTypes:  make([]string, 0, len(casMap)),
			Casualties: make([]int, 0, len(casMap)),
		}
		for k, cas := range casMap {
			p.KingdomIds = append(p.KingdomIds, k.kingdomID)
			p.UnitTypes = append(p.UnitTypes, k.unitType)
			p.Casualties = append(p.Casualties, cas)
		}
		if err := q.BulkDeductKingdomUnitsCasualties(ctx, p); err != nil {
			return nil, fmt.Errorf("bulk deduct casualties: %w", err)
		}
	}

	if popStolen > 0 {
		gainIDs := make([]int, 0, len(gainByKingdom))
		gainAmounts := make([]int, 0, len(gainByKingdom))
		for kingdomID, gain := range gainByKingdom {
			gainIDs = append(gainIDs, kingdomID)
			gainAmounts = append(gainAmounts, gain)
		}
		if err := q.BulkGainKingdomPopulation(ctx, db.BulkGainKingdomPopulationParams{
			Ids:   gainIDs,
			Gains: gainAmounts,
		}); err != nil {
			return nil, fmt.Errorf("bulk gain population: %w", err)
		}
		if err := q.StealKingdomPopulation(ctx, db.StealKingdomPopulationParams{
			ID:         targetKingdom.ID,
			Population: popStolen,
		}); err != nil {
			return nil, fmt.Errorf("steal population from kingdom %d: %w", targetKingdom.ID, err)
		}
	}

	// ── Phase 3: build result ─────────────────────────────────────────────────

	attackerUnits := make([]CombatUnitRecord, 0, len(atkByType))
	for _, r := range atkByType {
		attackerUnits = append(attackerUnits, *r)
	}
	defenderUnits := make([]CombatUnitRecord, 0, len(defByType))
	for _, r := range defByType {
		defenderUnits = append(defenderUnits, *r)
	}

	winner := "defender"
	if atkPow > defPow {
		winner = "attacker"
	}

	seen := map[int]bool{targetKingdom.ID: true}
	participants := []CombatParticipant{{KingdomID: targetKingdom.ID, Role: "defender"}}
	for _, c := range attackers {
		if !seen[c.KingdomID] {
			seen[c.KingdomID] = true
			participants = append(participants, CombatParticipant{
				KingdomID:        c.KingdomID,
				Role:             "attacker",
				PopulationGained: gainByKingdom[c.KingdomID],
			})
		}
	}
	for _, c := range defenders {
		if !seen[c.KingdomID] {
			seen[c.KingdomID] = true
			participants = append(participants, CombatParticipant{
				KingdomID: c.KingdomID,
				Role:      "defender",
			})
		}
	}

	return &CombatResult{
		TargetKingdomID:    targetKingdom.ID,
		AttackerUnits:      attackerUnits,
		DefenderUnits:      defenderUnits,
		AttackerPower:      atkPow,
		DefenderPower:      defPow,
		Winner:             winner,
		AttackerCasualties: totalAtkCasualties,
		DefenderCasualties: totalDefCasualties,
		PopulationStolen:   popStolen,
		Participants:       participants,
	}, nil
}

// ResolveCombat fires one combat round per target kingdom with active campaigns.
// Each round runs in its own transaction — a failure rolls back that kingdom only.
func ResolveCombat(ctx context.Context, pool *pgxpool.Pool, q db.Querier, tickID int) error {
	combatReady, err := q.GetActiveCampaignsReadyForCombat(ctx)
	if err != nil {
		return fmt.Errorf("combat: get combat-ready: %w", err)
	}
	if len(combatReady) == 0 {
		return nil
	}

	grouped := make(map[int][]db.KingdomCampaign)
	for _, c := range combatReady {
		grouped[c.TargetKingdomID] = append(grouped[c.TargetKingdomID], c)
	}
	targetIDs := make([]int, 0, len(grouped))
	for id := range grouped {
		targetIDs = append(targetIDs, id)
	}

	kingdoms, err := q.GetKingdomsByIDs(ctx, targetIDs)
	if err != nil {
		return fmt.Errorf("combat: bulk fetch kingdoms: %w", err)
	}
	kingdomByID := make(map[int]db.Kingdom, len(kingdoms))
	for _, k := range kingdoms {
		kingdomByID[k.ID] = k
	}

	allAvailable, err := q.GetAvailableKingdomUnitsByIDs(ctx, targetIDs)
	if err != nil {
		return fmt.Errorf("combat: bulk fetch available units: %w", err)
	}
	availableByKingdom := make(map[int][]db.KingdomAvailableUnit, len(targetIDs))
	for _, u := range allAvailable {
		availableByKingdom[u.KingdomID] = append(availableByKingdom[u.KingdomID], u)
	}

	allAtHomeLegionUnits, err := q.GetAtHomeLegionUnitsByKingdomIDs(ctx, targetIDs)
	if err != nil {
		return fmt.Errorf("combat: bulk fetch at-home legion units: %w", err)
	}
	legionUnitsByKingdom := make(map[int][]db.GetAtHomeLegionUnitsByKingdomIDsRow, len(targetIDs))
	for _, u := range allAtHomeLegionUnits {
		legionUnitsByKingdom[u.KingdomID] = append(legionUnitsByKingdom[u.KingdomID], u)
	}

	campaignIDs := make([]int, len(combatReady))
	for i, c := range combatReady {
		campaignIDs[i] = c.ID
	}
	allCampaignUnits, err := q.GetCampaignUnitsByCampaignIDs(ctx, campaignIDs)
	if err != nil {
		return fmt.Errorf("combat: bulk fetch campaign units: %w", err)
	}
	campaignUnitsByID := make(map[int][]db.KingdomCampaignUnit, len(campaignIDs))
	for _, u := range allCampaignUnits {
		campaignUnitsByID[u.CampaignID] = append(campaignUnitsByID[u.CampaignID], u)
	}

	for targetID, group := range grouped {
		var attackers, campaignDefenders []db.KingdomCampaign
		for _, c := range group {
			if c.Action == "attack" {
				attackers = append(attackers, c)
			} else {
				campaignDefenders = append(campaignDefenders, c)
			}
		}
		if len(attackers) == 0 {
			continue // only defenders present — no combat
		}
		if err := runCombatRound(
			ctx, pool, tickID,
			kingdomByID[targetID],
			availableByKingdom[targetID],
			legionUnitsByKingdom[targetID],
			attackers,
			campaignDefenders,
			campaignUnitsByID,
		); err != nil {
			log.Printf("combat: kingdom %d: %v", targetID, err)
		}
	}
	return nil
}

// runCombatRound executes and commits one combat round for a single target kingdom.
func runCombatRound(
	ctx context.Context,
	pool *pgxpool.Pool,
	tickID int,
	targetKingdom db.Kingdom,
	availableUnits []db.KingdomAvailableUnit,
	atHomeLegionUnits []db.GetAtHomeLegionUnitsByKingdomIDsRow,
	attackers []db.KingdomCampaign,
	campaignDefenders []db.KingdomCampaign,
	campaignUnitsByID map[int][]db.KingdomCampaignUnit,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txq := db.New(tx)
	result, err := resolveCombatAtKingdom(ctx, txq, targetKingdom, availableUnits, atHomeLegionUnits, attackers, campaignDefenders, campaignUnitsByID)
	if err != nil {
		return err
	}
	if result == nil {
		// No power on either side.
		return tx.Commit(ctx)
	}

	atkJSON, err := json.Marshal(result.AttackerUnits)
	if err != nil {
		return fmt.Errorf("marshal attacker units: %w", err)
	}
	defJSON, err := json.Marshal(result.DefenderUnits)
	if err != nil {
		return fmt.Errorf("marshal defender units: %w", err)
	}
	logParams := db.InsertCombatLogParams{
		TickID:             tickID,
		TargetKingdomID:    result.TargetKingdomID,
		AttackerUnits:      atkJSON,
		DefenderUnits:      defJSON,
		AttackerPower:      result.AttackerPower,
		DefenderPower:      result.DefenderPower,
		Winner:             result.Winner,
		AttackerCasualties: result.AttackerCasualties,
		DefenderCasualties: result.DefenderCasualties,
		PopulationStolen:   result.PopulationStolen,
	}
	logID, err := txq.InsertCombatLog(ctx, logParams)
	if err != nil {
		return fmt.Errorf("insert combat log: %w", err)
	}

	kingdomIDs := make([]int, len(result.Participants))
	roles := make([]string, len(result.Participants))
	popGained := make([]int, len(result.Participants))
	for i, p := range result.Participants {
		kingdomIDs[i] = p.KingdomID
		roles[i] = p.Role
		popGained[i] = p.PopulationGained
	}
	if err := txq.BulkInsertCombatLogParticipants(ctx, db.BulkInsertCombatLogParticipantsParams{
		CombatLogID:      logID,
		KingdomIds:       kingdomIDs,
		Roles:            roles,
		PopulationGained: popGained,
	}); err != nil {
		return fmt.Errorf("insert combat log participants: %w", err)
	}

	return tx.Commit(ctx)
}
