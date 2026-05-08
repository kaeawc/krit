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
