// go-lines: 11, 14, 25
package bindsinstead.scope

import dagger.Module
import dagger.Provides

interface ScopeService
class ScopeImplementation : ScopeService

// Go reports these aliases without a module. FIR abstains: a Dagger module host is not proven.
@Provides fun topLevel(implementation: ScopeImplementation): ScopeService = implementation

abstract class UnannotatedHost {
    @Provides fun unannotated(implementation: ScopeImplementation): ScopeService = implementation
}

@Module
abstract class OuterModule {
    fun localHost() {
        @Provides
        fun local(implementation: ScopeImplementation): ScopeService = implementation
    }
    abstract class NestedUnannotated {
        // Go reports; FIR does not borrow the outer module annotation for a different host.
        @Provides fun nestedUnannotated(implementation: ScopeImplementation): ScopeService = implementation
    }

    // Both engines abstain: local functions are not module members, even inside initializers or lambdas.
    val lambdaHost = {
        @Provides fun lambdaLocal(implementation: ScopeImplementation): ScopeService = implementation
    }
    val initializerHost = run {
        @Provides fun initializerLocal(implementation: ScopeImplementation): ScopeService = implementation
        0
    }
    val anonymousHost = fun() {
        @Provides fun anonymousLocal(implementation: ScopeImplementation): ScopeService = implementation
    }
    val getterHost: Int get() {
        @Provides fun getterLocal(implementation: ScopeImplementation): ScopeService = implementation
        return 0
    }
    constructor() {
        @Provides fun constructorLocal(implementation: ScopeImplementation): ScopeService = implementation
    }
    init {
        @Provides fun initLocal(implementation: ScopeImplementation): ScopeService = implementation
    }
}
