# Noctiluca

A generative, terminal-native bioluminescent ecosystem written in Go. A single `main.go`, Go standard library only, GPLv3.

Noctiluca simulates mobile luminous organisms with toroidal boundaries, local alignment and repulsion, Kuramoto-like oscillator coupling, environmental vector fields, energy variation, mutation, and birth/death events. Each organism deposits light into a decaying two-dimensional field. The simulation is artistic rather than a biological model.

## Requirements

- Linux or another terminal-capable operating system
- Go 1.23 or newer (older Go versions may also work)
- ANSI truecolor terminal for full color
- A terminal large enough for the chosen width and height

## Build

```bash
go build -o noctiluca main.go
```

## Quick start

```bash
./noctiluca
./noctiluca -mode vortex -palette ember -agents 250
./noctiluca -mode river -palette aurora -width 100 -height 30
./noctiluca -mode constellation -render braille -palette violet
./noctiluca -mode eclipse -frames 500 -fps 30
```

Press Ctrl+C to stop continuous animation. The cursor is restored on normal exit and handled termination.

## Simulation modes

| Mode | Dynamics |
| --- | --- |
| `nebula` | Sinusoidal turbulence and drifting clouds |
| `vortex` | Rotation around the center |
| `river` | Directed current and meanders |
| `constellation` | Moving attractor |
| `eclipse` | Central repulsion |
| `swarm` | Interaction-only flocking |

## Rendering

`-render ascii` produces a detailed text luminance field. `-render braille` packs 2×4 simulation cells into each terminal character. Both support truecolor. `-no-color` produces monochrome output. Available palettes are `ocean`, `ember`, `aurora`, `violet`, and `mono`.

The terminal should be at least `width` columns by `height + 1` rows for ASCII mode. Braille uses approximately half the width and one quarter the height. The application does not automatically query terminal dimensions.

## Scientific and artistic exports

```bash
./noctiluca -frames 1000 -quiet -output state.json -csv metrics.csv -svg artwork.svg
./noctiluca -input state.json -frames 200 -output continued.json
./noctiluca -frames 500 -output checkpoint.json -save-every 100
```

- JSON snapshots contain the simulation configuration, agents, light field, tick counter, and birth/death totals.
- CSV records tick, simulated time, population, oscillator order parameter, mean energy, mean speed, luminosity, births, and deaths.
- SVG exports the final luminous field as an 8× scaled grid of colored rectangles, suitable for viewing or editing in a vector graphics application.
- `-quiet` disables terminal animation and prints final metrics as JSON. Useful for scripts, batch experiments, and CI.

Snapshot writes use a temporary file, `fsync`, and atomic rename within the destination directory. The file is created with mode 0600.

## Controls

| Flag | Default | Description |
| --- | --- | --- |
| `-mode` | `nebula` | Simulation mode |
| `-width` | `90` | Field width (8–400) |
| `-height` | `32` | Field height (8–160) |
| `-agents` | `180` | Initial population (1–2000) |
| `-seed` | `42` | Random seed |
| `-dt` | `0.65` | Simulation timestep |
| `-coupling` | `1.1` | Oscillator coupling |
| `-radius` | `9` | Local interaction radius |
| `-trail` | `0.89` | Persistence of light, 0–1 |
| `-flow` | `0.18` | Strength of environmental vector field |
| `-mutation` | `0.2` | Mutation magnitude |
| `-epoch` | `500` | Reproduction interval, 0 disables evolution |
| `-frames` | `0` | Number of frames; 0 runs continuously |
| `-fps` | `20` | Display frame rate cap |
| `-render` | `ascii` | `ascii` or `braille` |
| `-palette` | `ocean` | Color palette |
| `-no-color` | `false` | Disable truecolor |
| `-quiet` | `false` | No animation, output final metrics |
| `-input` | none | Load saved state |
| `-output` | none | Save final state |
| `-snapshot` | none | Alias for `-output` |
| `-save-every` | `0` | Periodic checkpoint interval |
| `-csv` | none | Export time-series metrics |
| `-svg` | none | Export final field to SVG |

## Mathematics and model

Agents have a position, velocity, oscillator phase and frequency, energy, hue, age, and four heritable genome parameters. Each step computes nearby agents within a toroidal Euclidean interaction radius. Alignment and inverse-square repulsion contribute to velocity; a mode-specific vector field adds large-scale structure.

The phase oscillator evolves approximately as:

```
dθᵢ/dt = ωᵢ + K · meanⱼ[sin(θⱼ − θᵢ) · (1 − dᵢⱼ/R)]
```

Only neighbors within the interaction radius contribute. Light intensity is modulated by oscillator phase and energy. The global order parameter is the magnitude of the mean unit phase vector, from 0 (incoherent) to 1 (synchronized).

The light field decays by the trail factor every simulation step and receives Gaussian-weighted emissions from organisms. Toroidal wrapping avoids edge accumulation. A periodic reproduction event copies and mutates an organism, while older organisms may die stochastically.

## Performance

Neighbor search is a direct all-pairs calculation, O(N²) per frame. Rendering is O(width × height). Keep population moderate on small VMs. For example, `-agents 120 -width 80 -height 24 -fps 15` is a conservative starting point. The simulator is single-threaded and does not require elevated privileges.

## Reproducibility and limitations

A fixed seed produces repeatable fresh runs with the same parameters and executable. Resuming from a saved snapshot reconstructs the random generator from the seed and tick rather than serializing its internal state; a resumed run therefore does **not** guarantee bit-for-bit continuity with an uninterrupted run. A snapshot preserves the current agents and light field. There is no automatic terminal resize handling, interactive key controls, physical light transport, or spatial indexing. Genome values are inherited and mutated, but do not yet directly influence behavior; evolution is exploratory rather than fitness-driven.

## Validation

```bash
gofmt -w main.go
go build -o noctiluca main.go
go vet main.go
./noctiluca -frames 20 -quiet -output /tmp/noctiluca.json -csv /tmp/noctiluca.csv -svg /tmp/noctiluca.svg
./noctiluca -input /tmp/noctiluca.json -frames 2 -quiet
```

## License

GNU General Public License version 3.0. See `LICENSE`.
