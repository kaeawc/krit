// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7, 10, 13, 16, 19, 23, 27, 33, 37, 51
package test

import kotlin.ExperimentalStdlibApi

<!OptInMarkerExposedPublicly!>@ExperimentalStdlibApi<!>
fun exposed() = Unit

<!OptInMarkerExposedPublicly!>@ExperimentalStdlibApi<!>
val exposedProperty = 1

<!OptInMarkerExposedPublicly!>@ExperimentalStdlibApi<!>
class ExposedClass

<!OptInMarkerExposedPublicly!>@ExperimentalStdlibApi<!>
object ExposedObject

<!OptInMarkerExposedPublicly!>@ExperimentalStdlibApi<!>
interface ExposedInterface

class PublicOwner {
    <!OptInMarkerExposedPublicly!>@ExperimentalStdlibApi<!>
    fun member() = Unit

    companion object {
        <!OptInMarkerExposedPublicly!>@ExperimentalStdlibApi<!>
        fun companionMember() = Unit
    }
}

class Constructed {
    <!OptInMarkerExposedPublicly!>@ExperimentalStdlibApi<!>
    constructor(value: Int) {}
}

<!OptInMarkerExposedPublicly!>@ExperimentalStdlibApi<!>
fun String.extension() = length

open class PublicBase {
    // Go and FIR both skip protected declarations, including those in open classes.
    @ExperimentalStdlibApi
    protected fun protectedApi() = Unit

    companion object {
        @ExperimentalStdlibApi
        protected fun protectedCompanionApi() = Unit
    }
}

<!OptInMarkerExposedPublicly!>@ExperimentalStdlibApi<!>
typealias ExposedAlias = String
