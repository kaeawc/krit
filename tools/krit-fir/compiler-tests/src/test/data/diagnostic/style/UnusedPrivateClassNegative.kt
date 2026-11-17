// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives Go also leaves alone: every private class is referred to, or not judged.
package test.unusedprivateclassnegative

import kotlin.reflect.KClass

annotation class Tag(val value: KClass<*>)

private class Constructed
private class Typed
private class Parameter
private class ReturnType
private class Property
private open class Supertype
private class Generic<T>
private class Argument
private class Cast
private class Check
private class Literal
private class Annotated
private class Receiver
private class WithCompanion {
    companion object {
        const val VALUE = 1
    }
}
private class WithStatic {
    companion object Named {
        fun make() = 2
    }
}
private enum class Mode { A, B }
private enum class Entries { ONE, TWO }
private interface Contract
private annotation class Marker
private class Thrown : Exception()
private class InFunctionType
private class InLambdaReceiver
private class AsTypeArgument
private class InWhere

private open class Parent

private class Child : Parent()

private class Sub : Supertype()

private class Impl : Contract

private class Nested {
    private class Deep

    fun make(): Any = Deep()
}

private fun construct() = Constructed()

private fun typed(value: Typed?) = value

private fun parameter(p: Parameter) = p

private fun returnType(): ReturnType? = null

private val property: Property? = null

private fun child() = Child()

private fun sub() = Sub()

private fun impl(): Contract = Impl()

private fun generic() = Generic<Argument>()

private fun cast(value: Any) = value as Cast

private fun check(value: Any) = value is Check

private fun literal() = Literal::class

@Tag(Marker::class)
class Tagged

@Marker
class Marked

private fun annotated(value: Annotated) = value

private fun Receiver.extension() = 1

private fun companionConstant() = WithCompanion.VALUE

private fun companionMake() = WithStatic.make()

private fun modes() = Mode.A

private fun entries() = Entries.entries

private fun thrower() { throw Thrown() }

private fun catcher() {
    try {
        thrower()
    } catch (e: Thrown) {
        println(e)
    }
}

private fun functionType(block: (InFunctionType) -> Unit) = block

private fun lambdaReceiver(block: InLambdaReceiver.() -> Unit) = block

private fun typeArgument(list: List<AsTypeArgument>) = list

private fun <T> where(value: T) where T : InWhere = value

private fun nested() = Nested()

// A nested class reached only through its private enclosing class keeps the enclosing class used.
private class ConstructedThrough {
    class Inner
}

private class TypedThrough {
    class Inner
}

private fun constructedThrough() = ConstructedThrough.Inner()

private fun typedThrough(value: TypedThrough.Inner) = value

// Not judged: objects and companion objects are separate declarations in Go.
private object Singleton

private class Outer {
    private companion object
}

private fun outer() = Outer()

// Not private: a public, internal, protected or local class is never judged.
class PublicUnused

internal class InternalUnused

fun local() {
    class LocalUnused
}

open class Container {
    protected class ProtectedUnused
}
