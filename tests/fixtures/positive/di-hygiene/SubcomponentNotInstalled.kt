package dihygiene

annotation class Component
annotation class Subcomponent

@Subcomponent
interface UserSubcomponent {
    interface Factory {
        fun create(): UserSubcomponent
    }
}

// Parent component does NOT expose UserSubcomponent or its Factory.
@Component
interface AppComponent {
    fun something(): String
}
