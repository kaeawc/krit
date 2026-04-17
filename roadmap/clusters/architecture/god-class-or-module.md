# GodClassOrModule

**Cluster:** [architecture](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

A single class or module that references > N distinct packages —
the "knows about everything" pattern.

## Triggers

`class AppCoordinator` importing from 40 different packages.

## Does not trigger

Class imports stay within a bounded set.

## Dispatch

Per-class or per-module import-breadth count.

## Links

- Parent: [`../README.md`](../README.md)
- Related: [`fan-in-fan-out-hotspot.md`](fan-in-fan-out-hotspot.md)
