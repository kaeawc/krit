// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 29, 35, 36
package test

sealed class Device {
    object Speaker : Device()
    object Ear : Device()
    class Bluetooth(val name: String) : Device()
}

// A mutable property is not stable, so a null check or `when` subject check does not smart cast it and its type
// stays nullable. The compiler's smart-cast node has a synthetic source, so the report goes on the written part.
class Session {
    var current: Device? = null
    private var hidden: Device? = null

    fun log(text: String) = println(text)

    fun whenSubjectElse() {
        when (current) {
            Device.Ear -> log("ear")
            else -> log("else <!NullableToStringCall!>$current<!>")
        }
    }

    fun whenIs() {
        when (current) {
            is Device.Bluetooth -> log("bt <!NullableToStringCall!>$current<!>")
            else -> log("else <!NullableToStringCall!>$current<!>")
        }
    }

    fun whenEqualsStable(value: Device?) {
        when (value) {
            Device.Ear -> log("ear $value")
            else -> log("else <!NullableToStringCall!>$value<!>")
        }
    }

    fun stableNullCheck(value: Device?) {
        if (value != null) {
            log("value $value")
        }
    }

    fun nullChecked() {
        if (hidden != null) {
            log("hidden <!NullableToStringCall!>$hidden<!>")
        }
    }

    fun nullCheckedPublic() {
        if (current != null) {
            log("current <!NullableToStringCall!>$current<!>")
        }
    }
}
