// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Classes that declare serialVersionUID, are exempt, or are not Serializable.
package test

import java.io.Serializable

class CompanionConst : Serializable {
    companion object {
        private const val serialVersionUID: Long = 1L
    }
}

class NamedCompanion : Serializable {
    companion object Versions {
        private const val serialVersionUID = 2L
    }
}

// Enum classes serialize by name, so Go and FIR exempt them.
enum class Status : Serializable {
    ACTIVE,
    INACTIVE,
}

@Suppress("unused")
enum class AnnotatedStatus : Serializable {
    ON,
    OFF,
}

private enum class PrivateStatus : Serializable {
    ON,
}

enum class Priority {
    LOW {
        override fun weight() = 1
    },
    HIGH {
        override fun weight() = 2
    };

    abstract fun weight(): Int
}

class NotSerializable {
    val name = "plain"
}

open class PlainBase

class PlainDerived : PlainBase(), Comparable<PlainDerived> {
    override fun compareTo(other: PlainDerived): Int = 0
}

// A companion object's own Serializable does not make it a Serializable
// class here, and Go does not visit it either.
class WithSerializableCompanion {
    companion object : Serializable
}

// Anonymous objects are not class declarations, for Go and FIR.
val anonymous: Serializable = object : Serializable {}
