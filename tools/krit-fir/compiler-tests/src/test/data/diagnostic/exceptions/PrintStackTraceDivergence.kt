// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27, 38, 48, 58, 69
// Go artifacts: Go matches the receiver's spelling against the nearest
// enclosing catch's variable, so it reports these calls inside a catch whose
// variable is `e`. None of them is printStackTrace() on a Throwable.
package test

class Printer {
    fun printStackTrace() {}
}

class Holder(val e: Printer)

fun Throwable.printStackTrace(tag: String) {
    println("$tag: $message")
}

class Service(private val e: Printer) {
    // Go reports this because the receiver is spelled `e`; the local `e`
    // shadows the caught exception and is a Printer.
    fun shadowingLocal() {
        try {
            doWork()
        } catch (e: Exception) {
            run {
                val e = Printer()
                e.printStackTrace()
            }
        }
    }

    // Go reports this because the lambda parameter is spelled `e`; it is a
    // Printer.
    fun shadowingLambdaParameter(printers: List<Printer>) {
        try {
            doWork()
        } catch (e: Exception) {
            printers.forEach { e -> e.printStackTrace() }
        }
    }

    // Go reports this because the receiver's last identifier is `e`; the
    // property `this.e` is a Printer.
    fun thisProperty() {
        try {
            doWork()
        } catch (e: Exception) {
            this.e.printStackTrace()
        }
    }

    // Go reports this because the receiver's last identifier is `e`;
    // `holder.e` is a Printer.
    fun otherProperty(holder: Holder) {
        try {
            doWork()
        } catch (e: Exception) {
            holder.e.printStackTrace()
        }
    }

    // Go reports this because the receiver is the caught exception; the call
    // resolves to the extension above, which prints a line through println and
    // is not the stack-trace printer.
    fun projectExtension() {
        try {
            doWork()
        } catch (e: Exception) {
            e.printStackTrace("service")
        }
    }

    private fun doWork() {}
}
