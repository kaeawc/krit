# SingletonOnMutableClass

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@Singleton class Foo` where the class has any `var` property or any
`val` initialised to a mutable collection.

## Triggers

```kotlin
@Singleton
class UserCache @Inject constructor() {
    var currentUser: User? = null
    val entries = mutableListOf<Entry>()
}
```

## Does not trigger

```kotlin
@Singleton
class UserCache @Inject constructor() {
    private val _state = MutableStateFlow<User?>(null)
    val state: StateFlow<User?> = _state
}
```

## Dispatch

`class_declaration` annotated with a singleton scope (`@Singleton`,
`@ApplicationScoped`, etc.) whose body contains unprotected mutable
state.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
