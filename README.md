# Shift Command

A 3-faction, StarCraft-style neon RTS built in Go + [Ebitengine](https://ebitengine.org/) for **[GitHub Game Off 2026](https://itch.io/jam/game-off-2026)** (theme: **SHIFT**). It ships as an HTML5/WASM build on itch.io.

Each faction is built around a different meaning of SHIFT, and the Shift key is the faction's SHIFT ability:

- **The Forge**: *gear shift*. Units and buildings switch operating modes.
- **The Brood**: *shapeshift*. Everything morphs from a base form, and can re-morph.
- **The Phasebound**: *phase shift*. Units slip out of phase, warp in, and fuse.

The full design is in [docs/GDD.md](docs/GDD.md).

## Status

- **M0 (skeleton)**: window, neon renderer with a glow pass, fixed 20 ticks/s simulation, and CI that builds the WASM version and deploys it to GitHub Pages on every push to `main`.
- **M1 (RTS core)**: a sandbox skirmish on the map *Neon Crossing*. Both sides get a mixed squad of Forge units (Trooper, Outrider, Strider), and the enemy has a forward group to fight. It includes a camera with pan and zoom, a minimap, box/click/double-click selection, control groups, A* pathfinding and flow fields for group moves, separation steering, combat with attack/armor bonuses, and fog of war. There is no economy, faction SHIFT abilities or AI yet (M2), and unit stats are placeholders.

The title screen still has the M0 demo: press **Shift** to morph between the faction emblems, then press **Enter** or click to deploy.

## Controls (so far)

| Input | Action |
|---|---|
| **Left click / drag** | Select a unit / box-select your units |
| **Double-click** | Select all units of that type on screen |
| **Tab** or **F** + click | Add to/toggle selection; with an order, queue it |
| **Right click** | Move, or attack the enemy under the cursor |
| **A**, then left click | Attack-move (click an enemy to attack it) |
| **S** / **H** | Stop / hold position |
| **Ctrl** or **Alt** + **0–9** | Set control group (use Alt in the browser, where Ctrl+digit switches tabs) |
| **0–9** | Select control group; press twice to center the camera on it |
| **Arrow keys**, **middle-drag** | Scroll the camera (screen-edge scrolling is native-only) |
| **Mouse wheel** | Zoom |
| **Minimap** | Left click/drag: jump camera; right click: move |
| **Shift** | Faction SHIFT ability (arrives in M2) |
| **Esc** | Cancel attack-move targeting, then deselect |
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
go run ./cmd/game -match                              # skip the title, start the sandbox match
go run ./cmd/game -demo -updates 640 -screenshot fight.png   # army attack-moves into the enemy
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
internal/ai/       AI opponent (M2)
web/               index.html for the WASM build
scripts/           build scripts
docs/GDD.md        game design document
```

## License

[MIT](LICENSE)
