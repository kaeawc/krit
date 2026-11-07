// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Like detekt, only a declared supertype entry whose text is exactly
// `Serializable` matches, whatever it resolves to. Everything else in a
// Serializable family is not reported.
package test

import java.io.Externalizable
import java.io.Serializable

class QualifiedMarker : java.io.Serializable

abstract class ExternalMarker : Externalizable

open class Base : Serializable {
    companion object {
        private const val serialVersionUID = 1L
    }
}

class Derived : Base()

class Failure : Exception()

class Names : ArrayList<String>()

typealias Wire = java.io.Serializable

class Aliased : Wire

class Delegating(impl: Serializable) : Serializable by impl
