package dihygiene

annotation class Component
annotation class Subcomponent

@Subcomponent
interface UserSubcomponent {
    interface Factory {
        fun create(): UserSubcomponent
    }
}

// Parent exposes the subcomponent's Factory — installed.
@Component
interface AppComponent {
    fun userSub(): UserSubcomponent.Factory
}
