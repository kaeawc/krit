# DI graph validation cluster

Whole-graph validation of Dagger / Hilt / Metro / Anvil binding
graphs, orthogonal to the per-file [`../di-hygiene/`](../di-hygiene/)
cluster. These rules resolve transitively — they need the cross-file
index plus annotation-driven binding resolution.

No parent overview doc — scoped from the assistant's architecture
survey.

## Concepts

- [`whole-graph-binding-completeness.md`](whole-graph-binding-completeness.md)
- [`cross-module-scope-consistency.md`](cross-module-scope-consistency.md)
- [`di-cycle-detection.md`](di-cycle-detection.md)
- [`dead-bindings.md`](dead-bindings.md)
- [`di-graph-export.md`](di-graph-export.md)
