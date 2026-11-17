# Shift Command

A 3-faction, StarCraft-style neon RTS built in Go + [Ebitengine](https://ebitengine.org/) for **[GitHub Game Off 2026](https://itch.io/jam/game-off-2026)** (theme: **SHIFT**). It ships as an HTML5/WASM build on itch.io.

Each faction is built around a different meaning of SHIFT, and the Shift key is the faction's SHIFT ability:

- **The Forge**: *gear shift*. Units and buildings switch operating modes.
- **The Brood**: *shapeshift*. Everything morphs from a base form, and can re-morph.
- **The Phasebound**: *phase shift*. Units slip out of phase, warp in, and fuse.

The full design is in [docs/GDD.md](docs/GDD.md).

## Status

- **M0 (skeleton)**: window, neon renderer with a glow pass, fixed 20 ticks/s simulation, and CI that builds the WASM version and deploys it to GitHub Pages on every push to `main`.
- **M1 (RTS core)**: a sandbox skirmish on the map *Neon Crossing*. Both sides get a mixed squad of Forge units (Trooper, Outrider, Strider), and the enemy has a forward group to fight. It includes a camera with pan and zoom, a minimap, box/click/double-click selection, control groups, A* pathfinding and flow fields for group moves, separation steering, combat with attack/armor bonuses, and fog of war. That sandbox is still available with `-demo`.
- **M2a (economy, base building, production)**: the match is now a real skirmish. Each side starts with a Command Hub and 6 Riggers. Riggers mine Crystals on their own; you build Generators (supply), Extractors on flux vents (Flux), a Drill Yard and a Foundry, then train Troopers, Outriders, Menders, Striders and Bastions. Destroy every enemy building to win. All stats are placeholders.
- **M2b (enemy AI)**: the enemy Forge now plays. It follows a data-driven build order, mines and takes Flux, adds production, expands to its natural at around 4 minutes, and keeps its supply ahead. It defends its base, gathers an army at a staging point, attacks when the wave is big enough (each wave is bigger than the last), and retreats when a fight turns against it. It scouts expansions to find your buildings. There is one difficulty for now (Easy/Normal/Hard come in M5). When the enemy hits something of yours off-screen, you get an **Under attack!** message and a red ping on the minimap; press **Space** to jump there.
- **M2c (Forge SHIFT modes)**: press **Shift** (or the SHIFT slot on the command card) to switch the selection's gear-shift mode; see the table below. The enemy AI uses them too: it sieges Bastions and plants Striders near enemies, overdrives Troopers into fights, boosts Outriders while travelling, and packs everything up before retreating. The enemy never relocates its buildings.

The title screen still has the M0 demo: press **Shift** to morph between the faction emblems, then press **Enter** or click to deploy.

## How to play

1. Your Riggers start mining. Select the **Command Hub** and press **Q** to train more Riggers.
2. Select a Rigger and use the command card (**Q W E R T**) to build: Command Hub, Generator (+8 supply), Extractor (on a purple flux vent), Drill Yard, Foundry. Click to place; **Esc** or right click cancels.
3. Build an Extractor and send 3 Riggers to it (right click the vent) to collect Flux.
4. Select a Drill Yard or Foundry and train an army. Right click the ground with the building selected to set its rally point.
5. The enemy builds up and attacks with waves that grow over time, so build a defense first. Then attack-move into the enemy base and destroy every enemy building.

The resource bar (top right) shows Crystals, Flux and Supply. Hover a command-card button to see its cost, requirements, and why it is disabled.

### Forge SHIFT modes

Shift toggles every selected unit type separately: if any unit of a type is in its normal mode, all of that type switch; otherwise they all switch back. Overdrive always re-fires.

| Unit | SHIFT mode | Effect |
|---|---|---|
| Trooper | **Overdrive** | +50% move and attack speed for 8 s; costs 10 HP per use (needs more than 10 HP) |
| Outrider | **Cruise / Boost** | Boost doubles speed but can't attack |
| Strider | **Walk / Planted** | Planted: +3 range, can't move (1 s to switch) |
| Mender | **Mobile / Field Post** | Field Post: heals every infantry unit within 4 at once, can't move (1 s to switch) |
| Bastion | **Tank / Artillery** | Artillery: range 13 splash shells (no friendly fire), minimum range 2, can't move (2 s to switch) |
| Command Hub, Drill Yard, Foundry | **Landed / Packed** | Packed: the building lifts off (1.5 s), can't train or accept resources, and flies over walls when you right click. Press Shift again to land on a clear site nearby. It still provides supply and can be attacked. You must empty the production queue first |

## Controls (so far)

| Input | Action |
|---|---|
| **Left click / drag** | Select a unit / box-select your units |
| **Double-click** | Select all units of that type on screen |
| **Tab** or **F** + click | Add to/toggle selection; with an order, queue it |
| **Right click** | Move, attack the enemy under the cursor, gather from crystals/vents, or help build/repair your building |
| **Right click** (building selected) | Set the rally point; a packed building flies there instead |
| **Q W E R T** | Command card: build (worker selected) or train (building selected) |
| **A**, then left click | Attack-move (click an enemy to attack it) |
| **S** / **H** | Stop / hold position |
| **Ctrl** or **Alt** + **0–9** | Set control group (use Alt in the browser, where Ctrl+digit switches tabs) |
| **0–9** | Select control group; press twice to center the camera on it |
| **Arrow keys**, **middle-drag** | Scroll the camera (screen-edge scrolling is native-only) |
| **Mouse wheel** | Zoom |
| **Minimap** | Left click/drag: jump camera; right click: move |
| **Space** | Jump the camera to the last "Under attack!" alert |
| **Shift** | Faction SHIFT ability: switch the selection's gear-shift modes (see above) |
| **Esc** | Cancel placement or targeting, then cancel the last queued unit / construction, then deselect |
| **Click a queue slot** | Cancel that queued unit (refunded) |
| **F1** | Toggle the controls panel |
| **F2** / **F3** | Debug: spawn your / an enemy squad at the cursor |
| **F4** | Debug: toggle fog of war |

In the browser, click the game once to give it keyboard focus (required on itch.io, where the game runs in an iframe). The game shows a "click to focus" hint whenever it is not receiving keyboard input.

## Running

Requires Go (see `go.mod` for the version). On Linux, install the [Ebitengine dependencies](https://ebitengine.org/en/documents/install.html) first.

```sh
make run        # or: go run ./cmd/game
make test       # go test ./...
make vet        # go vet ./...
```

## Web build

```sh
make web        # builds dist/web/{index.html,wasm_exec.js,game.wasm}
make serve      # builds, then serves dist/web at http://localhost:8080
make zip        # builds dist/shift-command-web.zip for itch.io upload
```

For itch.io, upload `dist/shift-command-web.zip` as an HTML project; `index.html` sits at the zip root.

## Debug flags

```sh
go run ./cmd/game -updates 120 -screenshot shot.png   # run 2s, save the last frame, exit
go run ./cmd/game -autoshift 20                       # title screen: press Shift every 20 updates
go run ./cmd/game -match                              # skip the title, start the skirmish
go run ./cmd/game -demo -updates 640 -screenshot fight.png   # M1 sandbox: army attack-moves into the enemy
go run ./cmd/game -match -skip 200 -reveal                    # fast-forward 200s of AI play, fog off
```

## Layout

```
cmd/game/          entry point
internal/engine/   main loop, scenes, fixed-step clock, input → commands
internal/sim/      deterministic simulation (20 ticks/s): units, orders, combat
internal/faction/  data-driven faction definitions
internal/geom/     2D vector/rect math shared by sim, world, and ui
internal/ui/       neon renderer, glow pass, camera, effects, HUD, minimap
internal/assets/   code-generated/embedded resources (fonts; synthesized audio later)
internal/world/    tile grid, map, fog of war, A* and flow-field pathfinding
internal/ai/       AI opponent: build order, economy, army control
web/               index.html for the WASM build
scripts/           build scripts
docs/GDD.md        game design document
```

## License

[MIT](LICENSE)
