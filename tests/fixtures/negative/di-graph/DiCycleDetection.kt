package digraph

annotation class Inject
annotation class Component

class Alpha @Inject constructor(val beta: Beta)

class Beta @Inject constructor(val gamma: Gamma)

class Gamma @Inject constructor()

@Component
interface AppComponent {
    fun alpha(): Alpha
}
