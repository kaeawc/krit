// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Suspend functions the modifier is needed or fixed for. Go and FIR report nothing.
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import kotlinx.coroutines.Dispatchers

open class Service {
    open suspend fun projectSuspend() = Unit
}

suspend fun neededForProjectSuspend(service: Service) {
    service.projectSuspend()
}

class Repository(private val service: Service) {
    suspend fun load() {
        service.projectSuspend()
    }

    suspend fun loadAll() {
        load()
        load()
    }
}

interface BackupRepository {
    suspend fun getBackupsType(): String
}

suspend fun checkBackups(repo: BackupRepository) {
    val type = repo.getBackupsType()
    println(type)
}

suspend fun lambdaInsideInlineBuilder() {
    withContext(Dispatchers.IO) {
        delay(50)
    }
}

suspend fun nestedInlineLambdas(items: List<Int>) {
    items.forEach { item ->
        run {
            delay(item.toLong())
        }
    }
}

class Local {
    suspend fun localCall() {
        suspend fun local() {
            delay(1)
        }
        local()
    }
}

val holder = object {
    suspend fun inObjectLiteral() {
        delay(1)
    }
}
