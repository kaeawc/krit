package fixtures.negative.kotlin24

class ExplicitBackingFieldNaming {
    // The exposed type is Boolean; the explicit backing field only narrows
    // the initializer.
    val isEnabled: Boolean
        field = true

    // Accessor `field` is the implicit backing field.
    var isVisible: Boolean = false
        set(value) {
            field = value
        }
}
