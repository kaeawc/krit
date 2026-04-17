# ModuleTemplateConformance

**Cluster:** [sdlc/build-config](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Concept

"All feature modules must have `:ui`, `:data`, `:domain` submodules
with specific plugin applications" — convention-plugin conformance.

## Configuration

```yaml
module_template:
  feature_root: "feature:*"
  required_submodules: [ui, data, domain]
  required_plugins: [com.android.library, org.jetbrains.kotlin.android]
```

## Triggers

A `feature:x` module missing one of the required submodules or plugins.

## Does not trigger

Module matches the template.

## Dispatch

`BuildGraph` walk + template config.

## Links

- Parent: [`../README.md`](../README.md)
