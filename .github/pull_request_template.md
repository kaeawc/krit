## Summary

<!-- What does this PR do? -->

## Type

- [ ] Bug fix
- [ ] New rule
- [ ] Rule improvement (reduced FPs, new config option)
- [ ] Performance
- [ ] Documentation
- [ ] CI/tooling

## Testing

- [ ] Added positive fixture (`tests/fixtures/positive/`)
- [ ] Added negative fixture (`tests/fixtures/negative/`)
- [ ] Added unit test
- [ ] `make ci` passes
- [ ] Tested on a real codebase

## Checklist

- [ ] No new `go vet` warnings
- [ ] No new stubs (all rules have real implementations)
- [ ] Auto-fixes declare a safety level (cosmetic/idiomatic/semantic)
- [ ] Rule uses `DispatchBase` (preferred) or `LineBase` (if line-based)
