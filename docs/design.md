# Summary

Bahago (name to be changed) is a game inspired by Bahagon. 
It is a real time strategy game where you manage a kingdom, gather resouce, construct buildings, train armies and fight with your neighbords.

# Mechanics

## Population

Population is the base multiplier for resource gathering.

Percentage of population can be allocated to:
- Woodcutter: Gathers wood
- Miner: Gathers stone
- Farmers: Gathers food
- Clergy: Gathers devotion
- Disciple: Gathers mana, Prepare spells
- Scholar: Gathers knowledge
- Idle (unallocated population): Increase the growth of population per tick

The allocation of population to different role will change your resource rates:
- Production: positive rate per tick e.g. +30 wood per tick
- Upkeep: negative rate per tick e.g. -40 food per tick

## Resources

There are 3 material resources: 
- Wood: Required for buildings, army, technological advancement
- Stone: Required for buildings, army, technological advancement
- Food: Upkeep for population and army 

There are 3 incorporeal resources:
- Devotion: Base multipler for Divine interaction, required to sustain prayers
- Mana/aether/arcane: Required for rituals (spells), upkeep for summuned units
- Knowledge - Required for the advancements tree (technology, religon and magic)


TODO (design): Is knowledge a flat cost to unlock skills some kind of threshold with the resource never going down? 

## Doctrines/Discipline/Pillars

There are 3 doctrines that your kingdom can expand in:
- The Doctrine of Science
- The Doctrine of Faith
- The Doctrine of Arcane

A kingdom can invest in all 3 or specialize in one. 

TODO (design): Ideas of perks: Edict, decree, innovation, principle, tradition, discoveries

## The Compass/Wheel/Disc/Atlas

This is the advancement tree. It is split in 3 section like a pie. Each section is for a different doctrine.

## Alignments

Alignments give access unique perks that can be unlocked in the advancement tree. They are located at the outer edges and point towards a doctrine.

Here are some examples:

Science Doctrine:
  - Atheism: Can not get devotion; Bonus to knowledge/material resources
  - Tinker/Engineer: Unlocks mechanial units like siege engines and towers (no food upkeep)
Science/Faith Doctrine:
  - Crusader: During combat, convert enemy army to their rank based on devotion
Faith Doctrine:
  - Devotee: Can have 2 prayers at the same time/lower devotion upkeep
  - Altruism: Can pray for another player
  - Demonic: Can curse another player; Curses are like prayers but with negative effects
Faith/Arcana Doctrine:
  - Necromancy: Units lost in battle are transformed into undead based on devotion
  - Shaman: ??
Arcana Doctrine:
  - Conjurer: Conjured units have no/reduce mana upkeep
  - Arcane warden: Mana shield, mana counts as defence
Arcana/Science:
  - Alchemist: Can use mana to transform material resources into another


## Creating your kingdom

Kingdoms have name, this is how players identify each other e.g. "Kingdom of Bob"
TODO (design): Could we incorporate titles based on some mechanic? e.g. "Kingdom of duke bob" "Kingdom of queen bee"

Kingdom choose their alignment
TODO (design): Is the alignment unlocked a bit later?


## Constructions

## Perks


## Unit Attributes

- Worshipper - boosted by devotion
- Summun - does not have food upkeep
- Pacifism - can't attack
- Raiders - can't block
- Flying - bonus vs melee
- Archer - bonus vs Flying 20%
- Melee - no bonus
- Siege Engine - survives until the end of the round
- Undead - take 30% less damage from non-worshipper
- deathtouch - 50% more damage to non-summun
- enrage - 30% more power when outnumbered
- shields - 40% less damage from archer
- Gluttony - 30% more food upkeep


# Army

## Movement
 - You can send your army to attack or defend another player
 - Each army movements require a certain amount of ticks based on the distance of the other player
 - Multiple players can attack or defend the same player at the same time 

## Combat

- During combat, both sides will potentially lose army based on the difference in power of their armies
- A successful combat outcome is determined based on who has the strongest army
- The winner of a successful attack steals population from the loser

# Game world

## Map

- The map is a 2d grid
- Players have x and y position on the grid

---------------------------------------------------------------------------------------------------------------------------------------------------

# Misc + Brain Dump

- Rituals: Take time to prepare, based on disciples
- Prayers: Positive buffs that consumes devotion over time
- Constructions: Require worker, advance the player, camp, village, town, kingdom
- Night/Day cycle, slower attacks at night

### Science
 - Weapon upgrade I, II, III
 - Armor upgrade I, II, III
 - Tactical formation
 - Wood, mineral processors


### Religion
- Chantry I, II, III
- Religious rites I, II, III

### Magic 
- ritual site I, II, III
- summuning circle I, II, III

### Tier 1

- Dwelling - increase population growth rate
- Walls - protect population from getting stolen during raid
- Farms - increase food growth rate
- lumbermill - increase wood growth rate
- quarry - increase mineral growth
- schools - increase


### Knowledge/Experience
- school
- academy
- university

### Clergy
- Monastery
- Temple
- Shrine

### Magic
- ritual stones
- mana well
- arcane nexus
- obelisk

### Technology
- Forge
- Workshop
- Laboratory




## Necro

tier 1 ghosts - 20 power - apparation
tier 2 zombie - 80 power - apparation ghoulish
tier 3 Lich - 150 power - apparation ghoulish deathtouch


