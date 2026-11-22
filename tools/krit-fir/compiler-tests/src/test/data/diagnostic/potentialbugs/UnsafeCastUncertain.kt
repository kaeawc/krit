// go-lines: none
package unsafecastuncertain

interface CastMarker
interface CastOtherMarker
open class CastOpen
class CastChild : CastOpen(), CastMarker
class CastFinal
class CastGeneric<Value>

fun possibleSubclass(value: CastOpen) = value as CastMarker
fun possibleIntersection(value: CastMarker) = value as CastOtherMarker
fun possibleDowncast(value: CastOpen) = value as CastChild
fun erasedList(value: List<Int>) = value as List<String>
fun erasedContainer(value: CastGeneric<Int>) = value as CastGeneric<String>
// FIR abstains on generic targets; Go's compiler-diagnostic oracle reports this when it runs.
fun genericTarget(value: CastFinal) = value as List<String>
fun <Target> erasedParameter(value: Any) = value as Target
fun <Target : Any> parameterTarget(value: CastFinal) = value as Target
fun <Target> nullableParameter() = null as Target
// FIR-only: the resolved T : Any bound proves null cannot inhabit this target; Go's subtype shortcut accepts Nothing?.
fun <Target : Any> nonNullableParameter() = <!UnsafeCast!>null as Target<!>
fun bottomOperand() = TODO() as CastFinal
fun platformOperand() = System.getProperty("cast.input") as CastFinal
fun smartCast(value: CastOpen): CastChild? = if (value is CastChild) value as CastChild else null

// FIR abstains unless both endpoints are final concrete classes; Go reports these through its compiler-diagnostic oracle.
fun openDisjoint(value: CastOpen) = value as CastFinal
fun interfaceDisjoint(value: CastMarker) = value as CastFinal

open class OpenA
abstract class OpenB
// FIR abstains on two open/abstract endpoints; Go reports the compiler diagnostic through its oracle.
fun openToOpen(value: OpenA) = value as OpenB

class Token
interface Session
// FIR abstains unless both endpoints are final concrete classes; Go reports this through its compiler-diagnostic oracle.
fun classToInterface(value: Token) = value as Session
