# Ideas
Raw thoughts. No commitment, no structure. May never happen.

- Find a name
- [skills] Design skills
- [events] Add a Journal of events — could have...
- [timers] safe timers
- [winning] conquering zones for winning condition
- [army] prevent attacking if 75% of the tick has passed
- [refactor] Service layer in `internal/<feature>/` for multi-step actions — needs design pass: which features, what shape
- [refactor] Replace SSE re-query pattern with richer tick events — architectural, affects every refresh handler
- [refactor] Tick stress test via extended `cmd/seed/` — open-ended; how many kingdoms, what to measure

# TODO
Decided to do. One line is fine — enough to act on.

- [ ] [army] legion for armie
- [ ] [game] Fix startvation formula
- [ ] [game] Fix resource forumala with a power formula and building
- [ ] [map] add distance calculator
- [ ] [refactor] Split `handlers/guild/` by responsibility (membership / officers / invitations / lifecycle)
- [ ] [refactor] Extract `sse.Alert(err)` helper to kill `NewSSE().PatchElementGostar(errComponent(...))` boilerplate
- [ ] [refactor] Decide: graduate or remove `handlers/chat/`
- [ ] [refactor] Add `statement_timeout` to DB connection string (or wrap handler DB calls in `context.WithTimeout`)

# In Progress
What you're touching right now. Delete entries when done.

_(none)_
