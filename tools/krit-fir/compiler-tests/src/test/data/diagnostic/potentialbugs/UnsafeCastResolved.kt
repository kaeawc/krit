// go-lines: 10, 12, 58, 59, 66
package unsafecastresolved

class ResolvedLeft
class ResolvedRight
typealias ResolvedAlias = ResolvedRight
typealias ResolvedNullableAlias = ResolvedRight?

// Both engines resolve this non-null alias to an unrelated final class.
fun aliasTarget(value: ResolvedLeft) = <!UnsafeCast!>value as ResolvedAlias<!>
// Pinned false negative: Go reports this alias, but FIR abstains on all resolved nullable targets.
fun nullableAliasTarget(value: ResolvedLeft) = value as ResolvedNullableAlias

fun produceLeft(): ResolvedLeft = ResolvedLeft()
// FIR-only: Go's fallback excludes calls; the resolved return type is final and disjoint.
fun resolvedCall() = <!UnsafeCast!>produceLeft() as ResolvedRight<!>
// FIR-only: Go's fallback excludes constructor calls; the constructed class is final and disjoint.
fun resolvedConstructor() = <!UnsafeCast!>ResolvedLeft() as ResolvedRight<!>

fun shadowedLocal(value: ResolvedLeft) {
    run {
        val value = ResolvedRight()
        value as ResolvedRight
    }
}

// FIR-only: Go exempts when-subject scaffolding; resolution proves an impossible cast even in this branch.
fun discriminated(value: ResolvedLeft, name: String) = when (name) {
    "left" -> <!UnsafeCast!>value as ResolvedRight<!>
    else -> null
}

// FIR-only: Go excludes Java constructor calls; the resolved final URI and String classes are disjoint.
fun javaFinalConstructor() = <!UnsafeCast!>java.net.URI("https://example.com") as String<!>

fun boxedInteger(value: java.lang.Integer) = value as Int
fun boxedString(value: java.lang.String) = value as String
fun boxingInteger(value: Int) = value as java.lang.Integer
fun boxingString(value: String) = value as java.lang.String
fun boxedBoolean(value: java.lang.Boolean) = value as Boolean
fun boxedCharacter(value: java.lang.Character) = value as Char

// FIR abstains because the JDK hierarchy exposes these JVM interfaces beyond its mapped Kotlin supertypes.
fun stringConstable(value: String) = value as java.lang.constant.Constable
fun intConstantDesc(value: Int) = value as java.lang.constant.ConstantDesc

class Z
// FIR-only: Go exempts single-letter targets as presumed type parameters; Z is a resolved final class.
fun singleLetterTarget(value: ResolvedLeft) = <!UnsafeCast!>value as Z<!>

class CastHolder(val value: ResolvedLeft)
// FIR-only: Go excludes navigation operands; this property has a resolved disjoint final type.
fun resolvedProperty(holder: CastHolder) = <!UnsafeCast!>holder.value as ResolvedRight<!>

enum class Fruit { APPLE }
enum class Color { RED }
// Resolved enum classes are final and unrelated.
fun enumToString(value: Fruit) = <!UnsafeCast!>value as String<!>
fun enumToEnum(value: Fruit) = <!UnsafeCast!>value as Color<!>
// FIR abstains because enum runtime supertypes include JVM interfaces absent from its mapped hierarchy.
fun enumConstable(value: Fruit) = value as java.lang.constant.Constable

object LeftObject
object RightObject
// Distinct resolved object classes are final and unrelated.
fun objectToObject() = <!UnsafeCast!>LeftObject as RightObject<!>
