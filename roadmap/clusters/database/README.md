# Database / Room cluster

Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)

## Room @Query

- [`room-select-star-without-limit.md`](room-select-star-without-limit.md)
- [`room-query-missing-where-for-update.md`](room-query-missing-where-for-update.md)
- [`room-query-with-like-missing-escape.md`](room-query-with-like-missing-escape.md)
- [`room-suspend-query-in-transaction.md`](room-suspend-query-in-transaction.md)
- [`room-multiple-writes-missing-transaction.md`](room-multiple-writes-missing-transaction.md)
- [`room-conflict-strategy-replace-on-fk.md`](room-conflict-strategy-replace-on-fk.md)
- [`room-relation-without-index.md`](room-relation-without-index.md)
- [`room-return-type-flow-without-distinct.md`](room-return-type-flow-without-distinct.md)

## Schema / migration

- [`room-entity-changed-migration-missing.md`](room-entity-changed-migration-missing.md)
- [`room-database-version-not-bumped.md`](room-database-version-not-bumped.md)
- [`room-migration-uses-exec-sql-with-interpolation.md`](room-migration-uses-exec-sql-with-interpolation.md)
- [`room-export-schema-disabled.md`](room-export-schema-disabled.md)
- [`room-fallback-to-destructive-migration.md`](room-fallback-to-destructive-migration.md)

## Entities / DAOs

- [`entity-mutable-column.md`](entity-mutable-column.md)
- [`entity-primary-key-not-stable.md`](entity-primary-key-not-stable.md)
- [`dao-not-interface.md`](dao-not-interface.md)
- [`dao-without-annotations.md`](dao-without-annotations.md)
- [`foreign-key-without-on-delete.md`](foreign-key-without-on-delete.md)

## JDBC / raw SQLite

- [`jdbc-prepared-statement-not-closed.md`](jdbc-prepared-statement-not-closed.md)
- [`jdbc-result-set-leaked-from-function.md`](jdbc-result-set-leaked-from-function.md)
- [`sqlite-cursor-without-close.md`](sqlite-cursor-without-close.md)
