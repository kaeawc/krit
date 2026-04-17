# Security — taint-dependent (tier 3, deferred)

Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)

**Do not implement** any rule in this folder until the taint substrate
(source labelling, intra-procedural flow, sink registry) lands. See
the parent doc for the substrate design.

## Injection sinks

- [`sql-injection.md`](sql-injection.md)
- [`command-injection.md`](command-injection.md)
- [`path-traversal.md`](path-traversal.md)
- [`ldap-injection.md`](ldap-injection.md)
- [`xpath-injection.md`](xpath-injection.md)
- [`log-injection.md`](log-injection.md)

## Redirection / open-redirect

- [`open-redirect.md`](open-redirect.md)
- [`intent-redirection.md`](intent-redirection.md)
- [`unsafe-intent-launch.md`](unsafe-intent-launch.md)

## Crypto with flow

- [`encrypt-without-authentication.md`](encrypt-without-authentication.md)
- [`signature-verification-bypass.md`](signature-verification-bypass.md)

## Deserialization with flow

- [`unsafe-deserialization.md`](unsafe-deserialization.md)
- [`json-polymorphic-unsafe-type.md`](json-polymorphic-unsafe-type.md)
