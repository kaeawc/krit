// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 39, 52
// Project-declared printStackTrace overloads called on the caught exception.
// Each call is printStackTrace() on a Throwable writing to the console instead
// of a logger, so it is reported like Go, whatever the overload's body does and
// wherever it is declared: a top-level extension, a member extension of a
// class that is not a Throwable, or a member of a Throwable subclass.
package test

fun Throwable.printStackTrace(tag: String) {
    println("$tag: $message")
}

class Overload : Exception() {
    fun printStackTrace(tag: Int) {
        println(tag)
    }
}

class Service {
    fun topLevelExtension() {
        try {
            doWork()
        } catch (e: Exception) {
            <!PrintStackTrace!>e.printStackTrace("service")<!>
        }
    }

    // A member extension: the receiver it is bound to is the Throwable, not
    // this class.
    fun Throwable.printStackTrace(level: Long) {
        println("$level $message")
    }

    fun memberExtension() {
        try {
            doWork()
        } catch (e: Exception) {
            <!PrintStackTrace!>e.printStackTrace(3L)<!>
        }
    }

    // A String receiver is not a Throwable, in Go or here.
    fun String.printStackTrace() {
        println(this)
    }

    fun throwableSubclassMember() {
        try {
            doWork()
        } catch (e: Overload) {
            <!PrintStackTrace!>e.printStackTrace(1)<!>
            "text".printStackTrace()
        }
    }

    // Lookalikes owned by a local class and an object expression that are not
    // Throwables: no finding, and the owner lookup does not throw.
    fun localLookalikes() {
        try {
            doWork()
        } catch (e: Exception) {
            object {
                fun printStackTrace() {}
            }.printStackTrace()

            class LocalP {
                fun printStackTrace() {}
            }
            LocalP().printStackTrace()
        }
    }

    private fun doWork() {}
}
