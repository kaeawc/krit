// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Receivers spelled like the caught variable `e` that are other values. None
// of them is printStackTrace() on a Throwable, so neither Go nor FIR reports
// them: Go requires the bare caught name, not hidden by a local or a lambda
// parameter declared inside the catch.
package test

class Printer {
    fun printStackTrace() {}
}

class Holder(val e: Printer)

class Service(private val e: Printer) {
    // The local `e` shadows the caught exception and is a Printer.
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

    // The lambda parameter `e` is a Printer.
    fun shadowingLambdaParameter(printers: List<Printer>) {
        try {
            doWork()
        } catch (e: Exception) {
            printers.forEach { e -> e.printStackTrace() }
        }
    }

    // The property `this.e` is a Printer.
    fun thisProperty() {
        try {
            doWork()
        } catch (e: Exception) {
            this.e.printStackTrace()
        }
    }

    // `holder.e` is a Printer.
    fun otherProperty(holder: Holder) {
        try {
            doWork()
        } catch (e: Exception) {
            holder.e.printStackTrace()
        }
    }

    private fun doWork() {}
}
