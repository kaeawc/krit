// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 16, 21, 25, 30, 35, 42
// Local vars that Go and FIR both report: the var's initializer, a literal over
// 23 characters, can still be the tag the call passes. An assignment that runs
// before the call on every path replaces it (LongLogTagDivergence).
package test.longlogtag.localvar

import android.util.Log

fun localVars(flag: Boolean, items: List<Int>) {
    // Never reassigned.
    var unchanged = "UnchangedVarTagThatIsTooLong"
    <!LongLogTag!>Log.d(unchanged, "m")<!>
    // Assigned only after the call.
    var laterAssigned = "LaterAssignedTagThatIsTooLong"
    <!LongLogTag!>Log.d(laterAssigned, "m")<!>
    laterAssigned = "Short"
    // Assigned on one path only: when flag is false the long tag is passed.
    var conditional = "ConditionalTagThatIsTooLongX"
    if (flag) conditional = "Short"
    <!LongLogTag!>Log.d(conditional, "m")<!>
    // The first iteration passes the long tag before the assignment.
    var looped = "LoopedVarTagThatIsFarTooLong"
    for (item in items) {
        <!LongLogTag!>Log.d(looped, "m")<!>
        looped = "Short"
    }
    // forEach runs its lambda before the assignment that follows it.
    var captured = "CapturedVarTagThatIsTooLong"
    items.forEach { <!LongLogTag!>Log.d(captured, "m")<!> }
    captured = "Short"
    for (item in items) {
        // Declared inside the loop, so each iteration starts from the literal.
        var perIteration = "PerIterationVarTagIsTooLong"
        <!LongLogTag!>Log.d(perIteration, "m")<!>
        perIteration = "Short"
    }
    // Reassigned to another long literal before the call: FIR's message names
    // the literal passed, Go's names the initializer's; both report the line.
    var relabeled = "FirstLabelTagThatIsTooLongX"
    relabeled = "SecondLabelTagThatIsTooLong"
    <!LongLogTag!>Log.d(relabeled, "m")<!>
}
