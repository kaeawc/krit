package dihygiene

annotation class Inject

class Dep

abstract class BaseUseCase(val dep: Dep)

class ConcreteUseCase @Inject constructor(dep: Dep) : BaseUseCase(dep)

// Abstract but no @Inject; not flagged.
abstract class OtherBase(val dep: Dep)
