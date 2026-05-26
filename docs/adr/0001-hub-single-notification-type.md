# Keep the tick hub simple — one notification type, multiple publishers

The hub broadcasts a post-tick `db.Kingdom` struct to all connected SSE handlers for that kingdom. Handlers re-query only the domain data they need (buildings, combat logs, unread count, etc.) and patch their slice of the UI. We considered replacing this with a typed event that bundles all pre-computed domain data, but rejected it.

## Considered Options

**Richer tick events** — pass a `TickEvent{Kingdom, Buildings, Prayers, CombatLogs, …}` so handlers receive everything and do no re-queries. Rejected because: (1) the tick already bulk-fetches all kingdoms' data for game logic — adding UI-driven fetches bloats that loop with work disconnected from game correctness; (2) at our expected scale (≤1000 players, minority concurrent) the per-handler re-queries at tick time are negligible; (3) the re-query pattern is easy to reason about — each handler owns its data contract.

## Consequences

On-demand push (e.g. immediate notification when a message is received, without waiting up to 120 s for the next tick) is achieved by adding more **publishers**, not by changing the event shape. Any handler that mutates domain state that another SSE stream cares about should call `hub.Publish(affectedKingdom)` after the write. The messages handler is the first candidate.
