// go-lines: 13
package bindsinstead.lookalike

annotation class Module
annotation class Provides

interface LookalikeService
class LookalikeImplementation : LookalikeService

// Go reports simple-name annotation lookalikes; FIR requires dagger.Provides and dagger.Module.
@Module
abstract class LookalikeModule {
    @Provides fun fakeProvider(implementation: LookalikeImplementation): LookalikeService = implementation
    @dagger.Provides fun realProviderFakeModule(implementation: LookalikeImplementation): LookalikeService = implementation
}
