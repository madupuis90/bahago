# Legions are persistent named unit containers, not ephemeral campaign bundles

Before this change, a Campaign carried a single unit type and count directly in the `kingdom_campaigns` table (`unit_type TEXT`, `count INT`). Sending an army meant selecting a unit type, a quantity, and a target — the campaign was the army. That worked for single-type engagements but broke down as soon as we wanted multi-type compositions: you would need one campaign row per unit type, and the travelling-together semantics become implicit rather than enforced.

The refactor introduces **Legions** — persistent named containers (`kingdom_legions`) that each hold a composition of units (`kingdom_legion_units`). A Kingdom can have at most three Legions at a time. Campaigns now reference a Legion by foreign key; the legion's composition is snapshotted into `kingdom_campaign_units` at departure and restored (minus casualties) on return.

Unit **availability** is derived from a view rather than computed ad hoc:

```
kingdom_available_units = kingdom_units
                        − units assigned to any Legion
                        − units in campaign snapshots
```

This makes the "can I assign X units?" check a simple read against the view, and the reserve (units not in any legion) is an implicit third source of truth.

## Considered Options

**Keep ephemeral per-campaign unit bundles** — add a `kingdom_campaign_units` join table to the old design so a campaign can carry multiple unit types without persistent legions. Rejected because it provides no way for a player to pre-arrange an army composition they intend to reuse. Each send would require re-picking unit types from scratch, and there is no place in the UI to show "your standing armies".

**Single atomic CTE (`CreateCampaignIfAvailable`)** — the original design used a single CTE that checked availability and created the campaign in one query, preventing TOCTOU races at the database level. The legion model requires a multi-step transaction (lock legion, snapshot units, clear legion, create campaign) that is orchestrated in application code. The race window is acceptable because the campaign send path acquires a `FOR UPDATE` lock on the legion row, preventing concurrent sends of the same legion.

**Unlimited legions** — no cap on how many legions a Kingdom can create. Rejected for gameplay reasons: legions are a strategic decision, not a UI convenience. Three is the current limit (`MaxLegionsPerKingdom = 3`), enforced at the application layer via `CountLegionsForKingdom` rather than in the schema so the constant can be adjusted without a migration.

## Consequences

Campaigns now carry a snapshot of the legion's composition at departure, so the returning unit count can differ from the departing one (survivors only). The snapshot lives in `kingdom_campaign_units`; `BulkRestoreLegionUnits` copies survivors back into the legion on return.

The `kingdom_available_units` VIEW means unit availability is always consistent but requires joining three tables. Tick-path queries that need at-home unit totals use `GetAtHomeLegionUnitsByKingdomIDs`, which filters out deployed legions via `NOT EXISTS (SELECT 1 FROM kingdom_campaigns WHERE legion_id = ...)`.

Auto-naming (`'Legion ' || number`) is done in the `CreateLegion` query using `generate_series` to find the lowest unused slot ≤ Cap. Numbers are stable per kingdom — deleting Legion 2 and creating a new one produces "Legion 2" again.
