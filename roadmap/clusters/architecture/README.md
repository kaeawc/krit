# Architecture cluster

No dedicated parent overview doc — the concepts here come from the
architecture survey in the assistant's response to
"what else could krit provide". These rules operate at the
module-graph and package-level rather than per-file.

All rules reuse the module graph (`internal/module/`) and cross-file
index (`internal/scanner/index.go`).

## Layer enforcement

- [`layer-dependency-violation.md`](layer-dependency-violation.md)
- [`package-naming-convention-drift.md`](package-naming-convention-drift.md)

## Structural metrics

- [`module-dependency-cycle.md`](module-dependency-cycle.md)
- [`package-dependency-cycle.md`](package-dependency-cycle.md)
- [`fan-in-fan-out-hotspot.md`](fan-in-fan-out-hotspot.md)
- [`god-class-or-module.md`](god-class-or-module.md)

## API surface

- [`public-api-surface-snapshot.md`](public-api-surface-snapshot.md)
- [`public-to-internal-leaky-abstraction.md`](public-to-internal-leaky-abstraction.md)
