package dihygiene

import org.gradle.api.DefaultTask
import org.gradle.api.model.ObjectFactory
import org.gradle.process.ExecOperations

annotation class Inject

class Dep

abstract class BaseUseCase(val dep: Dep)

class ConcreteUseCase @Inject constructor(dep: Dep) : BaseUseCase(dep)

// Abstract but no @Inject; not flagged.
abstract class OtherBase(val dep: Dep)

// Gradle instantiates abstract tasks and extensions itself and injects its
// services; not flagged.
abstract class FormatTask @Inject constructor(
    private val execOps: ExecOperations,
) : DefaultTask()

abstract class PluginExtension @Inject constructor(objects: ObjectFactory)
