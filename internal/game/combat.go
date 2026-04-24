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

// resolveCombatAtKingdom runs one combat round between all attackers and defenders
// (both campaign-based defenders and the target's home army) at the given kingdom.
// All data is pre-fetched by the caller; this function only performs writes.
// It returns a CombatResult for logging, or nil when there is no power on
// either side and combat is skipped.
func resolveCombatAtKingdom(
	ctx context.Context,
	q db.Querier,
	targetKingdom db.Kingdom,
	targetAvailableUnits []db.KingdomAvailableUnit,
	attackers []db.KingdomCampaign,
	defenders []db.KingdomCampaign,
) (*CombatResult, error) {
	// Compute attacker power
	// TODO: unit attributes (Pacifism, Raiders, Flying, Archer, Worshipper, etc.)
	var atkPow int
	for _, c := range attackers {
		atkPow += totalUnitPower(c.UnitType, c.Count)
	}

	// Compute defender power
	var defPow int
	for _, u := range targetAvailableUnits {
		defPow += totalUnitPower(u.UnitType, u.Count)
	}
	for _, c := range defenders {
		defPow += totalUnitPower(c.UnitType, c.Count)
	}

	if atkPow == 0 && defPow == 0 {
		return nil, nil
	}

	// Loss ratios: each side loses proportional to the opponent's power advantage.
	atkLoss, defLoss := computeLossRatios(atkPow, defPow)

	// ── Phase 1: compute all outcomes ────────────────────────────────────────

	// casKingdomIDs, casUnitTypes, and casCasualties accumulate kingdom_units deductions
	// for all sides: home defenders, campaign defenders, and attackers.
	// Home-defender entries come from the available-units view (already aggregated by
	// (kingdom_id, unit_type)). Campaign entries are collected in campaignCas and
	// flattened below to avoid duplicate rows for kingdoms with multiple campaigns
	// of the same unit type.
	casKingdomIDs := make([]int, 0)
	casUnitTypes := make([]string, 0)
	casCasualties := make([]int, 0)

	// Attacker outcomes — aggregate by unit type across all attacking kingdoms.
	atkByType := make(map[string]*CombatUnitRecord)
	updateIDs := make([]int, 0, len(attackers)+len(defenders))
	updateCounts := make([]int, 0, len(attackers)+len(defenders))
	type campaignCasKey struct {
		kingdomID int
		unitType  string
	}
	campaignCas := make(map[campaignCasKey]int)
	var totalAtkCasualties int

	for _, c := range attackers {
		survivors := max(0, int(float64(c.Count)*(1-atkLoss)))
		casualties := c.Count - survivors
		if casualties > 0 {
			updateIDs = append(updateIDs, c.ID)
			updateCounts = append(updateCounts, survivors)
			campaignCas[campaignCasKey{c.KingdomID, c.UnitType}] += casualties
		}
		totalAtkCasualties += casualties

		r := atkByType[c.UnitType]
		if r == nil {
			r = &CombatUnitRecord{UnitType: c.UnitType}
			atkByType[c.UnitType] = r
		}
		r.Count += c.Count
		r.Power += totalUnitPower(c.UnitType, c.Count)
		r.Casualties += casualties
	}

	// Defender outcomes — home available units and campaign reinforcements, aggregated by type.
	defByType := make(map[string]*CombatUnitRecord)
	var totalDefCasualties int

	for _, u := range targetAvailableUnits {
		unitPow := totalUnitPower(u.UnitType, u.Count)
		casualties := 0
		if defLoss > 0 && defPow > 0 {
			casualties = int(float64(u.Count) * defLoss * float64(unitPow) / float64(defPow))
		}
		if casualties > 0 {
			casKingdomIDs = append(casKingdomIDs, targetKingdom.ID)
			casUnitTypes = append(casUnitTypes, u.UnitType)
			casCasualties = append(casCasualties, casualties)
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

	for _, c := range defenders {
		unitPow := totalUnitPower(c.UnitType, c.Count)
		survivors := c.Count
		casualties := 0
		if defLoss > 0 && defPow > 0 {
			cDefLoss := defLoss * float64(unitPow) / float64(defPow)
			survivors = max(0, int(float64(c.Count)*(1-cDefLoss)))
			casualties = c.Count - survivors
		}
		if casualties > 0 {
			updateIDs = append(updateIDs, c.ID)
			updateCounts = append(updateCounts, survivors)
			campaignCas[campaignCasKey{c.KingdomID, c.UnitType}] += casualties
		}
		totalDefCasualties += casualties

		r := defByType[c.UnitType]
		if r == nil {
			r = &CombatUnitRecord{UnitType: c.UnitType}
			defByType[c.UnitType] = r
		}
		r.Count += c.Count
		r.Power += unitPow
		r.Casualties += casualties
	}

	// Flatten aggregated campaign casualties into the cas* slices so that
	// BulkDeductKingdomUnitsCasualties receives one row per (kingdom_id, unit_type).
	for k, cas := range campaignCas {
		casKingdomIDs = append(casKingdomIDs, k.kingdomID)
		casUnitTypes = append(casUnitTypes, k.unitType)
		casCasualties = append(casCasualties, cas)
	}

	// Population stolen and per-attacker-kingdom distribution.
	// popStolen is capped to what the target can actually lose (floor is 100),
	// so attackers can never gain more population than the target gives up.
	// Integer truncation in share distribution may leave some population
	// unclaimed, which is intentional — no population is ever created.
	var popStolen int
	gainByKingdom := make(map[int]int)
	if atkPow > defPow {
		ratio := float64(atkPow-defPow) / float64(atkPow+defPow)
		calculated := max(1, int(float64(targetKingdom.Population)*ratio*0.1))
		popStolen = min(calculated, max(0, targetKingdom.Population-100))
		for _, c := range attackers {
			share := int(float64(popStolen) * float64(totalUnitPower(c.UnitType, c.Count)) / float64(atkPow))
			if share > 0 {
				gainByKingdom[c.KingdomID] += share
			}
		}
	}

	// ── Phase 2: writes ──────────────────────────────────────────────────────

	if len(updateIDs) > 0 {
		if err := q.BulkUpdateCampaignCounts(ctx, db.BulkUpdateCampaignCountsParams{
			Ids:    updateIDs,
			Counts: updateCounts,
		}); err != nil {
			return nil, fmt.Errorf("bulk update campaign counts: %w", err)
		}
	}

	// Delete any campaigns that were wiped out (all units killed this round).
	var depleted []int
	for i, count := range updateCounts {
		if count == 0 {
			depleted = append(depleted, updateIDs[i])
		}
	}
	if len(depleted) > 0 {
		if err := q.BulkDeleteCampaigns(ctx, depleted); err != nil {
			return nil, fmt.Errorf("delete depleted campaigns: %w", err)
		}
	}

	if len(casKingdomIDs) > 0 {
		if err := q.BulkDeductKingdomUnitsCasualties(ctx, db.BulkDeductKingdomUnitsCasualtiesParams{
			KingdomIds: casKingdomIDs,
			UnitTypes:  casUnitTypes,
			Casualties: casCasualties,
		}); err != nil {
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

	// ── Phase 3: build result ────────────────────────────────────────────────

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
	// Ties (atkPow == defPow) fall through to "defender" — the defending side
	// holds when forces are equal.

	// Build participant list — every kingdom involved in this combat round.
	// Target kingdom is always a defender. Attackers and campaign defenders follow.
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

	result := &CombatResult{
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
	}

	return result, nil
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
			attackers,
			campaignDefenders,
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
	attackers []db.KingdomCampaign,
	campaignDefenders []db.KingdomCampaign,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txq := db.New(tx)
	result, err := resolveCombatAtKingdom(ctx, txq, targetKingdom, availableUnits, attackers, campaignDefenders)
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
	participantParams := db.BulkInsertCombatLogParticipantsParams{
		CombatLogID:      logID,
		KingdomIds:       kingdomIDs,
		Roles:            roles,
		PopulationGained: popGained,
	}
	if err := txq.BulkInsertCombatLogParticipants(ctx, participantParams); err != nil {
		return fmt.Errorf("insert combat log participants: %w", err)
	}

	return tx.Commit(ctx)
}
