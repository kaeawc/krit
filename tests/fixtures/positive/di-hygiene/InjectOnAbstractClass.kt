package dihygiene

annotation class Inject

class Dep

abstract class BaseUseCase @Inject constructor(val dep: Dep)
