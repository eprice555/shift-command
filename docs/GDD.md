# Shift Command — Game Design Document (Game Off 2026)

**Jam:** https://itch.io/jam/game-off-2026 · **Theme:** SHIFT
**Deadline:** Dec 1, 2026 13:37 PST · **Target submit:** Nov 30
**Stack:** Go + Ebitengine → WASM (HTML5 on itch.io), source in a public GitHub repo
**Roles:** Copilot builds; Owner (eprice555) playtests/approves each milestone, approves all pushes/settings changes, owns itch submission

## 1. Pitch
A StarCraft-style 1v1 RTS skirmish with three asymmetric factions. Each faction is built around a different meaning of SHIFT:
- **The Forge** — *gear shift*: units and buildings switch operating modes.
- **The Brood** — *shapeshift*: everything morphs from a base form, and can re-morph.
- **The Phasebound** — *phase shift*: units slip out of phase, warp in, and fuse.

All names and art are original. Mechanics are inspired by StarCraft; no Blizzard IP.

## 2. Core RTS systems
| System | Design |
|---|---|
| Resources | **Crystals** (fields, 5 per trip) + **Flux** (vents, needs Extractor, 4 per trip) |
| Supply | Cap raised by supply source per faction; max 100 |
| Workers | Gather, construct, 50 crystals, 1 supply |
| Tech tree | HQ → Tier 1 building → Tier 2 building; requirements gate units |
| Combat | Ground only. Attack types: **Normal / Concussive / Explosive** vs armor **Light / Armored** (bonus damage) |
| Controls | Click/box select, double-click same-type, Ctrl+1–9 groups, right-click smart command, A-move, S-stop, H-hold, rally points, hotkey grid (QWER/ASDF) |
| Shift key | **Shift = the faction's SHIFT ability** on selected units (Tab/F = modifier for queueing). Theme hook: the Shift key literally shifts. |
| Fog of war | Unexplored (black) / explored (dim) / visible |
| Pathfinding | Grid A* + flow fields for group moves, separation steering |
| Win/lose | Destroy all enemy buildings |

## 3. Factions

### 3.1 The Forge (Terran-like) — Gear Shift
**Faction rules:** Buildings can **pack up and relocate** (Shift on a building). Workers repair. Supply via Generators.

| Building | Cost | Function | Requires |
|---|---|---|---|
| Command Hub | 400c | HQ, trains Rigger, drop-off. Shift: pack up & move | — |
| Generator | 100c | +8 supply | — |
| Extractor | 75c | Flux vent | — |
| Drill Yard | 150c | Trains Trooper, Outrider, Mender | Command Hub |
| Foundry | 150c/100f | Trains Strider, Bastion | Drill Yard |

| Unit | Cost | Supply | Role | SHIFT mode |
|---|---|---|---|---|
| Rigger | 50c | 1 | Worker, builds, repairs | — |
| Trooper | 50c | 1 | Basic ranged infantry (Light) | **Overdrive**: +50% attack/move speed, −10 HP |
| Outrider | 75c | 2 | Fast raider bike, bonus vs Light (Light) | **Cruise ↔ Boost**: boost = 2× speed, can't attack |
| Mender | 50c/25f | 1 | Support, heals infantry (Light) | **Mobile ↔ Field Post**: deploys, AoE heal aura, can't move |
| Strider | 100c/25f | 2 | Anti-armor walker (Armored) | **Walk ↔ Planted**: planted = +3 range, can't move |
| Bastion | 150c/100f | 3 | Siege tank (Armored) | **Tank ↔ Artillery**: artillery = range 13, splash, can't move |

### 3.2 The Brood (Zerg-like) — Shapeshift
**Faction rules:** The Hive passively spawns **Spawn** (max 3 per Hive). Every unit is morphed from Spawn. Workers morph into buildings (consumed). Buildings must be placed on **Creep**. Units regenerate HP. **Shift = re-morph** a unit into another unlocked form (pay the cost difference, 50% morph time).

| Building | Cost | Function | Requires |
|---|---|---|---|
| Hive | 300c | HQ, spawns Spawn, creep, drop-off | — |
| Nest Pod | 100c | +8 supply (building variant of Overlord) | — |
| Extractor | 25c | Flux vent | — |
| Spawning Pit | 200c | Unlocks Biter, Leaper, Spitter | Hive |
| Gestation Den | 150c/100f | Unlocks Behemoth, Broodmother | Spawning Pit |

| Unit | Cost | Supply | Role | SHIFT |
|---|---|---|---|---|
| Drone | 50c | 1 | Worker, morphs into buildings | Re-morph |
| Biter | 50c (×2) | 0.5 each | Fast melee swarm (Light) | Re-morph / **Burrow** |
| Leaper | 50c/25f | 1 | Raider, leaps onto targets from range 5 (Light) | Re-morph / **Pounce** (gap-closing leap, cooldown) |
| Spitter | 75c/25f | 2 | Ranged acid (Light) | Re-morph / **Root** (static, +2 range) |
| Broodmother | 100c/100f | 2 | Support caster: spawns timed Mites, transfuses HP (Armored) | Re-morph / **Brood Burst** (spawn 4 Mites, 10s lifetime) |
| Behemoth | 200c/100f | 4 | Heavy melee tank (Armored) | Re-morph / **Shell** (+4 armor, −50% speed) |

### 3.3 The Phasebound (Protoss-like) — Phase Shift
**Faction rules:** All units have **regenerating shields**. Buildings must be powered by an **Anchor** field. After researching Warp at a Resonator, newly trained units **warp in** at the Anchor nearest their rally point (or production building if they have no rally). **Shift = Phase**: the unit goes intangible for 2s (can't attack or be attacked, passes through units), 10s cooldown. Phasebound variants: Warden dashes toward a nearby enemy; Glint phases for 4s and crosses cliffs; Seer blinks forward; Conduit phases nearby allies for 3s on a longer cooldown.

| Building | Cost | Function | Requires |
|---|---|---|---|
| Spire | 400c | HQ, trains Drifter, drop-off | — |
| Anchor | 100c | +8 supply, power field | — |
| Extractor | 75c | Flux vent | — |
| Gate | 150c | Trains Warden, Glint, Seer | Anchor |
| Resonator | 150c/150f | Trains Conduits, Monoliths; researches Warp; Seer fusion | Gate |

| Unit | Cost | Supply | Role | SHIFT |
|---|---|---|---|---|
| Drifter | 50c | 1 | Worker, *summons* buildings (doesn't stay) | Phase |
| Warden | 100c | 2 | Melee, shields (Light) | **Phase Dash**: dash to a nearby enemy while phased |
| Glint | 100c/25f | 2 | Fast raider/scout, bonus vs workers (Light) | **Long Phase**: 4s phase, crosses cliffs |
| Seer | 125c/50f | 2 | Ranged, bonus vs Armored | **Phase Blink**: blink forward; two nearby Seers fuse into a Radiant |
| Conduit | 50c/150f | 2 | Support caster: restores shields (Armored) | **Phase Field**: nearby allies phase out together (3s, long cooldown) |
| Monolith | 300c/200f | 6 | Heavy ranged siege (Armored) | Phase |
| Radiant | — | 4 | Fused Seers; explosive splash and heavy shields | Phase |

Warp research costs 150c/150f and takes 60s (placeholder). Warped units spend 2s materializing and cannot be targeted during that time. Radiant stats and all Phasebound ability values are balance placeholders.

## 4. AI opponent
- Scripted **build orders** per faction (data-driven), with state machine: Expand economy → Tech → Build army → Attack at threshold → Retreat if losing.
- Uses its faction's SHIFT ability (sieges Bastions, re-morphs vs counters, phases out at low HP).
- Difficulty: **Easy / Normal / Hard** via reaction delay, build-order tightness, and income multiplier (Hard only).

## 5. Game flow
Title → Skirmish setup (pick your faction, AI faction, difficulty, map) → Match → Victory/Defeat stats screen.
Maps: 1 MVP map (2-player, mirrored, main + natural expansion), 2nd map stretch.

## 6. Technical architecture
- `cmd/game` entry; `internal/` packages: `engine` (loop, camera, input), `world` (map, fog, pathfinding), `sim` (units, combat, orders — fixed 20 ticks/s, deterministic), `faction` (data-driven defs), `ai`, `ui` (HUD, minimap, menus), `assets`.
- Unit/building stats in data tables so balance changes don't touch logic.
- GitHub Actions: build WASM on push → deploy to GitHub Pages for instant playtest links.
- Unit tests for sim logic (combat math, pathfinding, economy).

## 7. Art & audio
**Style: code-drawn geometric neon vector** (no external art assets). Dark background, glowing outlines (additive blending + blurred offscreen glow pass), subtle grid floor.

| Faction | Color | Shape language | Motion |
|---|---|---|---|
| The Forge | Amber/orange (#FFA630) | Squares, hexagons, hard angles, tread marks | Mechanical, snappy; mode shifts animate as parts sliding/locking |
| The Brood | Magenta + acid green (#FF3CAC / #7CFF4F) | Circles, blobs, tendrils | Wobbling/pulsing outlines; morphs tween between shapes |
| The Phasebound | Cyan/white (#3CF0FF) | Triangles, diamonds, rings | Floating, flickering; phase = translucent + scanline shimmer |

- Units are readable silhouettes at small size; team color = faction color, enemy tinted toward red outline.
- Effects: particle bursts on death, muzzle lines, shockwave rings for splash, creep as pulsing magenta noise field.
- **Audio: procedurally synthesized in Go** (no external assets), via Ebitengine `audio` package.
  - SFX: oscillator + envelope + noise synth (sfxr-style) — shots, explosions, morph/phase/mode-shift sounds, UI clicks, "unit ready" chirps. Generated once at startup into PCM buffers.
  - Music: short generated synthwave loops (bass arp + pad + drums) per faction, crossfade intensity when combat starts.
  - Audio starts after first click/keypress (browser autoplay rules).

## 8. Milestones
| # | Dates | Deliverable | Playtest gate (owner) |
|---|---|---|---|
| M0 | Nov 16–17 | Repo, Ebitengine skeleton, CI → GitHub Pages WASM build | Opens in browser |
| M1 | Nov 17–19 | RTS core: map, camera, minimap, selection, groups, A*/flow fields, combat, fog | Move/fight units feels RTS-like |
| M2 | Nov 19–21 | Economy + **The Forge complete** + basic AI (Forge mirror) | **Fallback-shippable game** |
| M3 | Nov 21–23 | **The Brood** | Brood playable vs AI |
| M4 | Nov 23–25 | **The Phasebound** | All 9 matchups work |
| M5 | Nov 25–27 | AI build orders for all factions, difficulty, balance pass | Hard AI is challenging |
| M6 | Nov 27–29 | Art/audio polish, menus, tooltips/how-to-play | Feels finished |
| M7 | Nov 29–30 | Bugfix, itch page, README, submit (owner approves) | Submitted |

## 9. Cut list (in order, if behind)
1. Fog of war → full vision
2. Phasebound fusion (Radiant)
3. Brood creep requirement
4. Forge building relocation
5. Support casters (Mender, Broodmother, Conduit) → back to 5 units/faction
6. Third faction entirely (ship 2 factions)

## 10. Stretch
Upgrades (attack/armor), spellcasters, air units, 2nd map, 2v2, replays.
