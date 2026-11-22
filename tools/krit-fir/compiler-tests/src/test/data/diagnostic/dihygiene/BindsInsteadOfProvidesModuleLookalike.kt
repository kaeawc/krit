// go-lines: 13
package bindsinstead.modulelookalike

import dagger.Provides

annotation class Module
interface ModuleLookalikeService
class ModuleLookalikeImplementation : ModuleLookalikeService

// Go reports this resolved Dagger provider in a lookalike module; FIR requires dagger.Module.
@Module
abstract class ModuleLookalikeHost {
    @Provides fun fakeModule(implementation: ModuleLookalikeImplementation): ModuleLookalikeService = implementation
}
