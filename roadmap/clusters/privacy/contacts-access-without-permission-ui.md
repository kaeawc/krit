# ContactsAccessWithoutPermissionUi

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

`ContactsContract.CommonDataKinds.Phone.CONTENT_URI` query without
a prior `ActivityResultContracts.RequestPermission` path.

## Triggers

```kotlin
resolver.query(ContactsContract.CommonDataKinds.Phone.CONTENT_URI, null, null, null, null)
```

## Does not trigger

The same call behind a permission-grant callback.

## Dispatch

`call_expression` on `query` with Contacts URI; walk enclosing
class for a permission-request path.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
