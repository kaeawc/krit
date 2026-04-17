# RoomDatabaseVersionNotBumped

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Catches

`@Database` version is unchanged but an `@Entity` changed since the
last git-tracked commit. CI-only rule — skip outside CI mode.

## Triggers

`@Database(version = 2)` unchanged while `User.kt` diff adds a
column.

## Does not trigger

Either the version is bumped, or no schema change is detected.

## Dispatch

Requires git integration (out-of-scope for most single-pass runs).
Enabled via a `--ci-mode` flag.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
