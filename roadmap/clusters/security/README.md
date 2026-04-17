# Security cluster

Parent docs:
- [`roadmap/49-security-rules-syntactic.md`](../../49-security-rules-syntactic.md)
- [`roadmap/50-security-rules-call-shape.md`](../../50-security-rules-call-shape.md)
- [`roadmap/51-security-rules-taint.md`](../../51-security-rules-taint.md)

## Sub-clusters

- [`syntactic/`](syntactic/) — ship-now rules keyed on a single call,
  annotation, or property shape. No data flow.
- [`call-shape/`](call-shape/) — opt-in advisors keyed on argument
  *shape* (literal vs concat vs interpolation).
- [`taint/`](taint/) — deferred until a taint substrate is built.
  See [`../../51-security-rules-taint.md`](../../51-security-rules-taint.md).

## Already shipping (do not re-scope)

- `AddJavascriptInterface`, `SetJavaScriptEnabled`, `GetInstance`
  (ECB/DES), `SecureRandom`, `TrustedServer`, `WorldReadableFiles`,
  `WorldWriteableFiles`, `AllowBackupManifest`, `DebuggableManifest`,
  `CleartextTraffic`, `ExportedWithoutPermission`,
  `ExportedContentProvider`, `ExportedReceiver`, `GrantAllUris`,
  `InsecureBaseConfigurationManifest`, `HandlerLeak`,
  `PackagedPrivateKey`, `ByteOrderMark`.
- See `internal/rules/android_security.go` and
  `internal/rules/android_manifest_security.go`.
