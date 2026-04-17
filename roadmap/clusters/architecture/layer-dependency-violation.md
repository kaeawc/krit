# LayerDependencyViolation

**Cluster:** [architecture](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

Module A depends on module B in violation of a configured layer
matrix. E.g. `ui` imports from `data-internal`.

## Configuration

```yaml
architecture:
  layers:
    - name: ui
      may_depend_on: [domain]
    - name: domain
      may_depend_on: [data-api]
    - name: data-api
      may_depend_on: []
    - name: data-internal
      may_depend_on: [data-api]
  match:
    ui: "**/ui/**"
    domain: "**/domain/**"
    data-api: "**/data/api/**"
    data-internal: "**/data/internal/**"
```

## Triggers

`ui` module imports a symbol from `data-internal`.

## Does not trigger

`ui` imports only from `domain`.

## Dispatch

Module graph walk; resolve each cross-module reference to its
layer, compare against the configured allow matrix.

## Links

- Parent: [`../README.md`](../README.md)
- Related: [`package-naming-convention-drift.md`](package-naming-convention-drift.md)
