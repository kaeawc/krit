// Smoke: suspending on JDK CompletionStage/CompletableFuture with kotlinx.coroutines.future.await.
package stubs

import java.util.concurrent.CompletableFuture
import java.util.concurrent.CompletionStage
import kotlinx.coroutines.future.await

suspend fun awaitCompletionStages(stage: CompletionStage<String>): Int {
    val future: CompletableFuture<String> = CompletableFuture.supplyAsync { "smoke" }
    return stage.await().length + future.await().length
}
