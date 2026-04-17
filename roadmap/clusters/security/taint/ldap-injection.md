# LdapInjection

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

Untrusted source reaching a JNDI LDAP lookup string or
`DirContext.search(...)` filter argument.

## Shape

```kotlin
val name = request.queryParameter("user")
ctx.search("ou=users", "(cn=$name)", controls)
```

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
