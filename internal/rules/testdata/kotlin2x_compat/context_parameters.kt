// rules: UnusedParameter, UnusedImport, LongParameterList, FunctionOnlyReturningConstant
package compat

import compat.log.Logger
import compat.log.Metrics
import compat.log.Tracer
import compat.log.Unused // expect: UnusedImport

// A context parameter is supplied implicitly, so an unused name is not an unused parameter.
context(logger: Logger)
fun unusedContext() {
    println("hi")
}

context(_: Logger)
fun anonymousContext() {
    log("hello")
}

context(logger: Logger, tracer: Tracer)
fun usesBoth(a: Int, b: Int) {
    logger.info(a.toString())
    tracer.trace(b)
}

// Context parameters are not counted as value parameters: five value parameters stay under the threshold.
context(l1: Logger, l2: Tracer, l3: Metrics)
fun manyContexts(a: Int, b: Int, c: Int, d: Int, e: Int) {
    l1.info("$a $b $c $d $e")
    l2.trace(a)
    l3.count(b)
}

context(Logger)
fun legacyContextReceiver() {
    info("legacy")
}

context(logger: Logger)
val loggerName: String
    get() = logger.name

context(logger: Logger)
fun constant(): Int = 42 // expect: FunctionOnlyReturningConstant

fun shortlist(a: Int, b: Int) = a + b

fun realParameter(used: Int, unused: Int) = used // expect: UnusedParameter
