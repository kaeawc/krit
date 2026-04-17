# ScreenshotNotBlockedOnLoginScreen

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Activity` / `@Composable` whose class or function name contains
`Login`/`Password`/`Pin`/`Secure`/`Payment`/`Card` with no
`FLAG_SECURE` / `ScreenshotBlocker` modifier set.

## Triggers

```kotlin
class LoginActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.login)
    }
}
```

## Does not trigger

```kotlin
class LoginActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        window.setFlags(FLAG_SECURE, FLAG_SECURE)
        setContentView(R.layout.login)
    }
}
```

## Dispatch

`class_declaration` / `@Composable` whose name matches the pattern;
walk body for `FLAG_SECURE` reference.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
