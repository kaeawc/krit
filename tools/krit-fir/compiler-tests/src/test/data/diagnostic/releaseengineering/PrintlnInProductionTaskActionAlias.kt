// RENDER_DIAGNOSTICS_FULL_TEXT
// Go exempts a function whose annotation is spelled TaskAction, so it reports
// this call: the annotation is spelled Action. The annotation is TaskAction
// through a typealias, so the function is a task action and FIR does not
// report it.
package test

annotation class TaskAction

typealias Action = TaskAction

abstract class AliasedTask {
    @Action
    fun execute() {
        println("aliased annotation")
    }
}
