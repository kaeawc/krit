// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.ui.semantics

import androidx.compose.runtime.Immutable
import androidx.compose.ui.Modifier

interface SemanticsPropertyReceiver {
    operator fun <T> set(key: SemanticsPropertyKey<T>, value: T)
}

class SemanticsPropertyKey<T>(val name: String)

// A value class with companion constants, not an enum: `Role.Button` resolves
// to the companion property Role.Companion.Button.
@Immutable
@JvmInline
value class Role private constructor(private val value: Int) {
    companion object {
        val Button: Role
            get() = TODO()

        val Checkbox: Role
            get() = TODO()

        val Switch: Role
            get() = TODO()

        val RadioButton: Role
            get() = TODO()

        val Tab: Role
            get() = TODO()

        val Image: Role
            get() = TODO()

        val DropdownList: Role
            get() = TODO()
    }
}

fun Modifier.semantics(mergeDescendants: Boolean = false, properties: SemanticsPropertyReceiver.() -> Unit): Modifier =
    TODO()

fun Modifier.clearAndSetSemantics(properties: SemanticsPropertyReceiver.() -> Unit): Modifier = TODO()

var SemanticsPropertyReceiver.contentDescription: String
    get() = TODO()
    set(value) = TODO()

var SemanticsPropertyReceiver.stateDescription: String
    get() = TODO()
    set(value) = TODO()

var SemanticsPropertyReceiver.role: Role
    get() = TODO()
    set(value) = TODO()

var SemanticsPropertyReceiver.testTag: String
    get() = TODO()
    set(value) = TODO()

fun SemanticsPropertyReceiver.heading() {
    TODO()
}

fun SemanticsPropertyReceiver.invisibleToUser() {
    TODO()
}

fun SemanticsPropertyReceiver.onClick(label: String? = null, action: (() -> Boolean)?) {
    TODO()
}
