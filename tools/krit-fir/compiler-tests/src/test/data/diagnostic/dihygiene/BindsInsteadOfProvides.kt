// go-lines: 13, 15, 33, 39, 46
package bindsinstead.basic

import dagger.Module
import dagger.Provides

interface BasicService
class BasicImplementation : BasicService

@Module
abstract class BasicModule {
    @Provides
    fun <!BindsInsteadOfProvides!>plain<!>(implementation: BasicImplementation): BasicService = implementation

    @Provides fun <!BindsInsteadOfProvides!>inferred<!>(implementation: BasicImplementation) = implementation

    @Provides fun block(implementation: BasicImplementation): BasicService { return implementation }
    @Provides fun cast(implementation: BasicImplementation): BasicService = implementation as BasicService
    @Provides fun parentheses(implementation: BasicImplementation): BasicService = (implementation)
    @Provides fun conversion(implementation: BasicImplementation): BasicService = implementation.let { it }
    @Provides fun nullableParameter(implementation: BasicImplementation?): BasicService? = implementation
    @Provides fun nullableReturn(implementation: BasicImplementation): BasicService? = implementation
    @Provides fun multiple(first: BasicImplementation, second: BasicImplementation): BasicService = first
    @Provides fun empty(): BasicService = BasicImplementation()

    // Both engines abstain: Go stops at the enclosing function; FIR rejects the concrete anonymous host.
    fun anonymousInside() = object {
        @Provides fun anonymousInsideMember(implementation: BasicImplementation): BasicService = implementation
    }

    companion object {
        @Provides
        fun <!BindsInsteadOfProvides!>companion<!>(implementation: BasicImplementation): BasicService = implementation
    }

    @Module
    abstract class NestedModule {
        @Provides
        fun <!BindsInsteadOfProvides!>nested<!>(implementation: BasicImplementation): BasicService = implementation
    }
}

@Module
interface BasicInterfaceModule {
    @Provides
    fun <!BindsInsteadOfProvides!>member<!>(implementation: BasicImplementation): BasicService = implementation
}

@Module
class ConcreteModule {
    @Provides fun concrete(implementation: BasicImplementation): BasicService = implementation
    companion object {
        @Provides fun concreteCompanion(implementation: BasicImplementation): BasicService = implementation
    }
}

@Module
object ObjectModule {
    @Provides fun objectMember(implementation: BasicImplementation): BasicService = implementation
}

fun anonymousModule() = object {
    // Both engines abstain: Go stops at the enclosing function; FIR rejects the concrete anonymous host.
    @Provides fun anonymous(implementation: BasicImplementation): BasicService = implementation
}
