// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 23, 27, 35, 73, 77, 80, 122, 140, 148
package test

import kotlin.OptIn as Acknowledge
import test.MarkedBase as RenamedBase

@kotlin.RequiresOptIn(level = kotlin.RequiresOptIn.Level.WARNING)
annotation class ExperimentalCoroutinesApi

// Go-only: FIR skips this marker in its own package tree; Go recognizes the well-known name.
@ExperimentalCoroutinesApi // FIR abstains: FIR skips this marker in its own package tree; Go recognizes the well-known name.
fun sameModuleMarkerIsSkipped() = Unit

annotation class RequiresOptIn

@RequiresOptIn
annotation class ExperimentalStdlibApi

annotation class ExperimentalTime

// Go-only: this same-named project annotation is not an opt-in marker.
@ExperimentalTime // FIR abstains: this same-named project annotation is not an opt-in marker.
fun ordinaryAnnotation() = Unit

// Go-only: RequiresOptIn resolves to a project lookalike, not kotlin.RequiresOptIn.
@ExperimentalStdlibApi // FIR abstains: RequiresOptIn resolves to a project lookalike, not kotlin.RequiresOptIn.
fun falseMetaAnnotation() = Unit

@kotlin.OptIn(ExperimentalCoroutinesApi::class)
fun consumed() = Unit

// Go-only: Go does not recognize the aliased kotlin.OptIn acknowledgment.
@Acknowledge(ExperimentalCoroutinesApi::class)
@ExperimentalCoroutinesApi // FIR abstains: resolved kotlin.OptIn consumes the requirement.
fun acknowledgedAlias() = Unit

@ExperimentalCoroutinesApi
private fun privateApi() = Unit

@ExperimentalCoroutinesApi
internal fun internalApi() = Unit

internal class HiddenOwner {
    @ExperimentalCoroutinesApi
    fun hiddenMember() = Unit
}

private class PrivateOwner {
    @ExperimentalCoroutinesApi
    fun hiddenMember() = Unit
}

@kotlin.OptIn(ExperimentalCoroutinesApi::class)
class AcknowledgedOwner {
    @ExperimentalCoroutinesApi
    fun member() = Unit
}

/** Callers must opt-in because this API may change. */
@kotlin.ExperimentalStdlibApi
fun documentedWarning() = Unit

/** Requires ExperimentalStdlibApi. */
@kotlin.ExperimentalStdlibApi
fun documentedMarker() = Unit

@kotlin.OptIn(kotlin.ExperimentalStdlibApi::class)
@kotlin.ExperimentalStdlibApi
interface MarkedBase

// Go-only: Go cannot connect the qualified supertype to its external marker acknowledgment.
@kotlin.ExperimentalStdlibApi // FIR abstains: the qualified supertype acknowledges this marker.
class Inherited : test.MarkedBase

// Go-only: Go cannot connect the aliased supertype to its external marker acknowledgment.
@kotlin.ExperimentalStdlibApi // FIR abstains: the aliased supertype acknowledges this marker.
class AliasedInherited : RenamedBase {
    // Go-only too: Go misses the external-marker acknowledgment on the aliased supertype.
    @kotlin.ExperimentalStdlibApi // FIR abstains: Go misses the external-marker acknowledgment on the aliased supertype.
    fun deliberatelyMarkedMember() = Unit
}

interface Contract {
    fun member()
}

class Implementation : Contract {
    @ExperimentalCoroutinesApi
    override fun member() = Unit
}

fun localScope() {
    @ExperimentalCoroutinesApi
    fun local() = Unit
    class LocalOwner {
        @ExperimentalCoroutinesApi
        fun member() = Unit
    }
}

class FinalOwner {
    @ExperimentalCoroutinesApi
    protected fun helper() = Unit

    protected class HiddenNested {
        @ExperimentalCoroutinesApi
        fun helper() = Unit
    }
}

enum class Mode {
    A;
    @ExperimentalCoroutinesApi
    protected fun helper() = Unit
}

sealed class Shape {
    @ExperimentalCoroutinesApi
    protected fun helper() = Unit
    // Go-only: Go treats this implicitly protected sealed constructor as public.
    @ExperimentalCoroutinesApi // FIR abstains: the sealed constructor is implicitly protected.
    constructor(id: Int)
}

open class ProtectedOpenOwner {
    @ExperimentalCoroutinesApi
    protected fun helper() = Unit
}

open class ProtectedCompanionOwner {
    companion object {
        @ExperimentalCoroutinesApi
        protected fun helper() = Unit
    }
}

val runner = object : Runnable {
    // Go-only: Go walks through object_literal; this member is absent from Runnable.
    @ExperimentalCoroutinesApi // FIR abstains: this anonymous object's extra member is not public API.
    fun helper() = Unit
    override fun run() = Unit
}

enum class Kind {
    A {
        // Go-only: Go walks through enum_entry; this member is absent from Kind.
        @ExperimentalCoroutinesApi // FIR abstains: this enum entry's extra member is not public API.
        fun extra() = Unit
    }
}
