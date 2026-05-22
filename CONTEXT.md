# Bahago

A browser-based multiplayer medieval kingdom strategy game. Players manage a single kingdom, gather resources, build structures, train armies, and compete or ally with other kingdoms.

## Language

**Kingdom**: A player's realm. Each user owns exactly one. Has a name, coordinates on the World Map, a Population, and stockpiles of the six Resources.
_Avoid_: Realm, Faction — use "Kingdom" when referring to the in-game entity, and "Player" only when referring to the person behind it.

**Population**: The kingdom's workforce and the base multiplier for all resource production. Grows each tick via Idle allocation. Declines from starvation. Stolen by successful attackers.
_Avoid_: Workers, Citizens, People.

**Resource**: One of six stockpiles — Wood, Stone, Food, Mana, Devotion, Knowledge. Each has a **Production Rate** (positive, driven by Allocation and Building bonuses) and an **Upkeep** (negative, driven by game mechanics — e.g. Prayers add devotion upkeep, Units add food upkeep). The **Net Rate** is Production minus Upkeep; a stockpile only declines when the net rate is negative. Starvation, for example, occurs when the current Food stockpile plus Food Production is less than Food Upkeep.
_Avoid_: Currency, Income — use "Production Rate" and "Upkeep" for the per-tick flows, "stockpile" for the accumulated amount, "Net Rate" for the combined effect.

**Allocation**: The distribution of a kingdom's Population across seven percentage slices — one per Resource plus Idle. All seven must sum to 100.
_Avoid_: Population Management, Distribution.

**Idle**: The Allocation slice not assigned to any Resource. Drives Population growth each tick. A kingdom that stops allocating to production grows its workforce faster.
_Avoid_: Unallocated, Free Population.

**Tick**: A single game clock pulse fired once per hour. All state changes occur at tick time: Resources are produced, Constructions and Training advance, Campaigns move, Prayers are charged, and Combat resolves.
_Avoid_: Turn, Round, Hour.

**Building**: A structure that provides a percentage production bonus to one or more Resources. Defined statically by type (name, max count, cost, ticks to build, bonus); runtime state is a count per Kingdom. Built via Construction.
_Avoid_: Structure, Improvement — use "Building" for the completed structure, "Construction" for the in-progress work.

**Construction**: An in-progress building project tracked by `ticks_remaining`. Completes when the countdown reaches zero, incrementing the Building count.
_Avoid_: Building (see above).

**Unit**: A standard military unit trained with Wood or Stone, with food upkeep. Examples: Recruit, Archer, Raider, Knight, Catapult.
_Avoid_: Soldier, Troop, Regular.

**Summon**: A magical military unit trained with Mana, with mana upkeep. Requires mana production to unlock. Examples: Shade, Dread Knight.
_Avoid_: Magical Unit, Mana Unit.

**Training**: An in-progress batch of Units or Summons. A single Training order produces a fixed count of one type; it completes in a fixed number of ticks regardless of batch size.
_Avoid_: Recruiting, Hiring.

**Attribute**: A conditional modifier on a Unit or Summon that affects combat behaviour or upkeep. Examples: Flying, Archer, Melee, Summon, Undead, Deathtouch, Enrage.
_Avoid_: Trait, Ability, Stat.

**Campaign**: A military expedition sent by one Kingdom to another, with an action of either attack or defend. Passes through three statuses:
- **en_route** — units are travelling to the target Kingdom.
- **active** — units are performing their action (attacking or defending) each tick.
- **returning** — units are travelling back to the home Kingdom.

_Avoid_: Mission, Raid, Quest.

**Combat**: The resolution of all active attack and defend Campaigns at a target Kingdom each tick. Produces a Combat Log entry.
_Avoid_: Battle, Fight, Conflict.

**Combat Log**: A record of a single Combat resolution for a given tick and target Kingdom. Records both sides' units, power totals, casualties, winner, and Population stolen.
_Avoid_: Battle Log, History.

**Prayer**: An ongoing devotion-powered blessing cast by a Kingdom. Costs devotion upkeep each tick; grants a resource production bonus to the target Kingdom. Currently all Prayers are self-targeted.
_Avoid_: Spell, Blessing, Buff.

**Guild**: A multi-Kingdom alliance. A Kingdom can belong to at most one active Guild.

Guild statuses:
- **Pending** — the Guild has been created by an Applicant but has not yet reached the 5-supporter threshold. Expires if the threshold is not met.
- **Active** — the Guild is fully formed and operational.

Guild membership roles:
- **Applicant** — the Kingdom that created a Pending Guild. Becomes the Leader when the Guild activates.
- **Supporter** — a Kingdom that pledged to back a Pending Guild. Becomes a Member when the Guild activates.
- **Member** — a full member of an Active Guild.
- **Officer** — a Member promoted by the Leader. Can approve join requests and send Invitations.
- **Leader** — the former Applicant; the founding Kingdom and owner of the Guild. Has full authority: can promote Members to Officer, manage membership, and disband the Guild entirely.
- **Pending Approval** — a Kingdom that has requested to join an Active Guild, awaiting Officer or Leader approval.
- **Invited** — a Kingdom that has received a Guild Invitation from an Officer, awaiting the Kingdom's accept or deny.

_Avoid_: Alliance, Faction, Clan.

**World Map**: A 64×64 tile grid divided into 8×8 pages of 8×8 tiles. Each Kingdom occupies a coordinate on the grid.
_Avoid_: Map, Grid.

---

## Example dialogue

> **Domain expert**: My kingdom is running low on food and my population is starting to drop.
>
> **Developer**: Your Food allocation is probably too low — try shifting a few percentage points from Wood to Food in your Allocation. If you've built Farms, those add a production bonus too. Keep in mind starvation causes Population loss scaled to the deficit, so even a small shortfall compounds over ticks.
>
> **Domain expert**: I also sent an attack campaign yesterday — will my units come back?
>
> **Developer**: Yes. Once the campaign's active phase finishes (after `action_ticks` ticks of combat), the status moves to returning and the units travel home. Check the Combat Log for what happened while they were there.
