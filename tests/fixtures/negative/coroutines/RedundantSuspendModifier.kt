package fixtures.negative.coroutines

import kotlinx.coroutines.delay

suspend fun real() {
    delay(100)
}

open class Service {
    open suspend fun projectSuspend() = Unit
}

suspend fun neededForProjectSuspend(service: Service) {
    service.projectSuspend()
}

open suspend fun overridable() {}

abstract class Base {
    abstract suspend fun required()
}

class Child : Base() {
    override suspend fun required() {}
}

// Should NOT flag: calls a project-defined suspend function that can't be resolved
interface BackupRepository {
    suspend fun getBackupsType(): String
}

suspend fun checkBackups(repo: BackupRepository) {
    val type = repo.getBackupsType()
    println(type)
}

// Should NOT flag: calls an unresolved method on an injected dependency
interface BillingApi {
    suspend fun queryProduct(id: String): Any
}

suspend fun loadProduct(api: BillingApi, id: String) {
    val product = api.queryProduct(id)
    println(product)
}

// actual / expect functions are part of a multiplatform contract; their
// modifier is locked by the declaration on the other side and must not be
// reported as redundant even when the body lacks suspend calls.
actual suspend fun actualPlatformContract() {
    println("platform impl with no suspend calls in this body")
}

expect suspend fun expectPlatformContract()

// external suspend declarations have no body we can inspect.
external suspend fun externalDeclaration()

// Lambda passed to an inline builder must still count: withContext is the
// most common case and its lambda body's delay() proves the outer modifier
// is necessary even though our walk also catches withContext itself.
suspend fun lambdaInsideInlineBuilder() {
    kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
        delay(50)
    }
}

// runCatching is a non-coroutine inline builder. Its lambda runs in the
// enclosing suspend context, so a delay() inside it justifies the modifier.
suspend fun lambdaInsideRunCatching() {
    runCatching {
        delay(10)
    }
}

// Suspend calls inside `apply`/`let`/`also`/`run` lambdas execute in the
// enclosing suspend context too.
class Repo {
    var count: Int = 0
}

suspend fun lambdaInsideApply(repo: Repo) {
    repo.apply {
        delay(1)
        count++
    }
}
