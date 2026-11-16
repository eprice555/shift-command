# Shift Command

A 3-faction, StarCraft-style neon RTS built in Go + [Ebitengine](https://ebitengine.org/) for **[GitHub Game Off 2026](https://itch.io/jam/game-off-2026)** (theme: **SHIFT**). It ships as an HTML5/WASM build on itch.io.

Each faction is built around a different meaning of SHIFT, and the Shift key is the faction's SHIFT ability:

- **The Forge**: *gear shift*. Units and buildings switch operating modes.
- **The Brood**: *shapeshift*. Everything morphs from a base form, and can re-morph.
- **The Phasebound**: *phase shift*. Units slip out of phase, warp in, and fuse.

The full design is in [docs/GDD.md](docs/GDD.md).

## Status

**M0 (skeleton)**: window, neon renderer with a glow pass, fixed 20 ticks/s simulation, and a Shift-key demo that morphs between the three faction emblems. CI builds the WASM version and deploys it to GitHub Pages on every push to `main`.

## Controls (so far)

| Input | Action |
|---|---|
| **Shift** | Shift to the next faction form (M0 demo) |

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
go run ./cmd/game -autoshift 20                       # press Shift every 20 sim ticks
```

## Layout

```
cmd/game/          entry point
internal/engine/   main loop, fixed-step clock, input
internal/sim/      deterministic simulation (20 ticks/s)
internal/faction/  data-driven faction definitions
internal/ui/       neon renderer, glow pass, HUD
internal/assets/   code-generated/embedded resources (fonts; synthesized audio later)
internal/world/    map, fog, pathfinding (M1)
internal/ai/       AI opponent (M2)
web/               index.html for the WASM build
scripts/           build scripts
docs/GDD.md        game design document
```

## License

[MIT](LICENSE)
