# InlineCodelens

**Cluster:** [sdlc/lsp-integration](README.md) · **Status:** planned · **Severity:** n/a (LSP)

## Concept

Complexity, test coverage, last-touched commit, and risk score
shown above each function in the editor.

## Shape

```
// complexity=18 · 47 consumers · last touched 3 days ago
fun loadUsers(): List<User> { ... }
```

## Infra reuse

- Existing LSP server.
- Existing complexity rules + reference index.

## Links

- Parent: [`../README.md`](../README.md)
