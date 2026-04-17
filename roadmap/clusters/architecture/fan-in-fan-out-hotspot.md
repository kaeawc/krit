# FanInFanOutHotspot

**Cluster:** [architecture](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

Class whose fan-in exceeds a configured threshold, or whose
fan-in × complexity score is in the top N% of the project.

## Triggers

A utility class referenced by 300 other files in a 400-file project.

## Does not trigger

Fan-in within threshold, or class is clearly a framework entry
point (Activity, Fragment, etc.).

## Dispatch

Cross-file reference aggregation; report as a summary rule per
run rather than per-file.

## Links

- Parent: [`../README.md`](../README.md)
