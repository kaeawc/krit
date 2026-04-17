# Concurrency / coroutines cluster

Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)

## Dispatcher / context

- [`flow-without-flow-on.md`](flow-without-flow-on.md)
- [`with-context-in-suspend-function-noop.md`](with-context-in-suspend-function-noop.md)
- [`main-dispatcher-in-library-code.md`](main-dispatcher-in-library-code.md)
- [`supervisor-scope-in-event-handler.md`](supervisor-scope-in-event-handler.md)
- [`global-scope-launch-in-view-model.md`](global-scope-launch-in-view-model.md)

## Lifecycle / cancellation

- [`collect-in-on-create-without-lifecycle.md`](collect-in-on-create-without-lifecycle.md)
- [`launch-without-coroutine-exception-handler.md`](launch-without-coroutine-exception-handler.md)
- [`channel-receive-without-close.md`](channel-receive-without-close.md)
- [`deferred-await-in-finally.md`](deferred-await-in-finally.md)
- [`coroutine-scope-created-but-never-cancelled.md`](coroutine-scope-created-but-never-cancelled.md)

## JVM primitives

- [`synchronized-on-string.md`](synchronized-on-string.md)
- [`synchronized-on-boxed-primitive.md`](synchronized-on-boxed-primitive.md)
- [`synchronized-on-non-final.md`](synchronized-on-non-final.md)
- [`volatile-missing-on-dcl.md`](volatile-missing-on-dcl.md)
- [`mutable-state-in-object.md`](mutable-state-in-object.md)
- [`collections-synchronized-list-iteration.md`](collections-synchronized-list-iteration.md)
- [`concurrent-modification-iteration.md`](concurrent-modification-iteration.md)

## Flow-specific

- [`state-flow-mutable-leak.md`](state-flow-mutable-leak.md)
- [`shared-flow-without-replay.md`](shared-flow-without-replay.md)
- [`state-flow-compare-by-reference.md`](state-flow-compare-by-reference.md)
