# Hermetic and build-system runs

This page collects the settings that matter when krit runs inside a build
system action (Bazel, Buck, a sandboxed CI step): no shared state between runs,
no processes that outlive the action, and the same output for the same inputs.

## JDK selection

Krit looks for `java` and `javac` under `$JAVA_HOME/bin` when `JAVA_HOME` is
set, and on `PATH` otherwise. This covers every JVM krit launches (the type
oracle, the FIR checker, the krit-fir daemons, the Java facts helper and its
`javac` compile) and the checks that decide whether to download the krit-fir
jar. A `JAVA_HOME` that lacks the tool is an error, not a fall back to `PATH`,
so an action that pins a JDK through `JAVA_HOME` never runs a different one.
`krit --doctor` prints the launchers it found.
