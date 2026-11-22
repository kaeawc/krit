package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.support.walkWithParents
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall

internal object ClickableTouchBodies {
    fun callsClick(root: FirElement): Boolean {
        var found = false
        walkWithParents(root) { element, _ ->
            if (element is FirFunctionCall) {
                if (element.calleeReference.name.asString() == "performClick") found = true
            }
        }
        return found
    }
}
