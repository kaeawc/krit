// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 25
// Positives: a public or protected function or property with no declared type
// whose inferred type is a Java platform type. The finding sits on the name.
// Go reports only an expression-body function whose body text contains a
// `java.`, `javax.`, `android.`, `Java` or `Javax` identifier (`fullyQualified` and `flexibleCollection`),
// so it misses every platform type written without that prefix and every
// property. detekt reports all of these, since it reads the inferred type.
package test

import java.io.File

class Positives {
    // Go reads the `java.` prefix of the expression text and reports this one too.
    fun <!HasPlatformType!>fullyQualified<!>() = java.io.File("a").name

    fun <!HasPlatformType!>imported<!>() = File("a").parent

    fun <!HasPlatformType!>environment<!>() = System.getenv("HOME")

    val <!HasPlatformType!>property<!> = System.getProperty("user.dir")

    fun <!HasPlatformType!>flexibleArray<!>() = File("a").list()

    fun <!HasPlatformType!>flexibleCollection<!>() = java.util.Collections.emptyList<String>()

    protected val <!HasPlatformType!>protectedProperty<!> = File("a").name

    fun <!HasPlatformType!>block<!>() = run { File("a").name }

    var <!HasPlatformType!>mutable<!> = System.getenv("HOME")

    companion object {
        fun <!HasPlatformType!>fromCompanion<!>() = File("a").name
    }
}

fun <!HasPlatformType!>topLevel<!>() = File("a").name

val <!HasPlatformType!>topLevelProperty<!> = System.getenv("HOME")

object Singleton {
    fun <!HasPlatformType!>member<!>() = File("a").name
}

// The visibility of the enclosing class does not matter, as in detekt.
private class PrivateOwner {
    fun <!HasPlatformType!>member<!>() = File("a").name
}

open class Overrider : Base() {
    // The inferred type is inherited, and it is the platform type of Base.name().
    override fun <!HasPlatformType!>name<!>() = super.name()
}

open class Base {
    open fun <!HasPlatformType!>name<!>() = File("a").name
}
