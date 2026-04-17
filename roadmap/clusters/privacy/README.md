# Privacy / data-handling cluster

Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)

All rules key on an **SDK vocabulary registry** (new infra,
config-driven). See the parent doc for the registry design.

## PII reaching analytics

- [`analytics-event-with-pii-param-name.md`](analytics-event-with-pii-param-name.md)
- [`analytics-user-id-from-pii.md`](analytics-user-id-from-pii.md)
- [`crashlytics-custom-key-with-pii.md`](crashlytics-custom-key-with-pii.md)

## Permission / data access

- [`location-background-without-rationale.md`](location-background-without-rationale.md)
- [`contacts-access-without-permission-ui.md`](contacts-access-without-permission-ui.md)
- [`clipboard-on-sensitive-input-type.md`](clipboard-on-sensitive-input-type.md)
- [`screenshot-not-blocked-on-login-screen.md`](screenshot-not-blocked-on-login-screen.md)
- [`biometric-auth-not-falling-back-to-device-credential.md`](biometric-auth-not-falling-back-to-device-credential.md)

## Consent gating

- [`analytics-call-without-consent-gate.md`](analytics-call-without-consent-gate.md)
- [`admob-initialized-before-consent.md`](admob-initialized-before-consent.md)
- [`firebase-remote-config-defaults-with-pii.md`](firebase-remote-config-defaults-with-pii.md)

## Storage

- [`shared-preferences-for-sensitive-key.md`](shared-preferences-for-sensitive-key.md)
- [`plain-file-write-of-sensitive.md`](plain-file-write-of-sensitive.md)
- [`log-of-shared-preference-read.md`](log-of-shared-preference-read.md)
