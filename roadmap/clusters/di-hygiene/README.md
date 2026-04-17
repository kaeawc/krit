# DI hygiene cluster

Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)

Per-rule checks for Dagger / Hilt / Metro / Anvil. For whole-graph
binding validation see [`../di-graph/`](../di-graph/).

## Scope and lifecycle

- [`singleton-on-mutable-class.md`](singleton-on-mutable-class.md)
- [`scope-on-parameterized-class.md`](scope-on-parameterized-class.md)
- [`inject-on-abstract-class.md`](inject-on-abstract-class.md)
- [`hilt-singleton-with-activity-dep.md`](hilt-singleton-with-activity-dep.md)

## Bindings graph shape

- [`provider-instead-of-lazy.md`](provider-instead-of-lazy.md)
- [`lazy-instead-of-direct.md`](lazy-instead-of-direct.md)
- [`binds-instead-of-provides.md`](binds-instead-of-provides.md)
- [`binds-mismatched-arity.md`](binds-mismatched-arity.md)
- [`binds-return-type-matches-param.md`](binds-return-type-matches-param.md)
- [`module-with-non-static-provides.md`](module-with-non-static-provides.md)
- [`into-set-on-non-set-return.md`](into-set-on-non-set-return.md)

## Multibindings

- [`into-map-missing-key.md`](into-map-missing-key.md)
- [`into-map-duplicate-key.md`](into-map-duplicate-key.md)
- [`into-set-duplicate-type.md`](into-set-duplicate-type.md)
- [`missing-jvm-suppress-wildcards.md`](missing-jvm-suppress-wildcards.md)

## Component / graph wiring

- [`component-missing-module.md`](component-missing-module.md)
- [`subcomponent-not-installed.md`](subcomponent-not-installed.md)
- [`hilt-entry-point-on-non-interface.md`](hilt-entry-point-on-non-interface.md)
- [`hilt-install-in-mismatch.md`](hilt-install-in-mismatch.md)

## Metro / Anvil

- [`metro-graph-factory-missing-abstract.md`](metro-graph-factory-missing-abstract.md)
- [`anvil-contributes-binding-without-scope.md`](anvil-contributes-binding-without-scope.md)
- [`anvil-merge-component-empty-scope.md`](anvil-merge-component-empty-scope.md)
