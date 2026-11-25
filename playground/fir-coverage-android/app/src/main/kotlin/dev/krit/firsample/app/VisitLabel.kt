package dev.krit.firsample.app

// VisitCounter is a Java class in this module's src/main/java, so FIR
// resolves it only when it compiles the module's Java sources too.
fun visitLabel(counter: VisitCounter): String = greeting("you", counter.increment())
