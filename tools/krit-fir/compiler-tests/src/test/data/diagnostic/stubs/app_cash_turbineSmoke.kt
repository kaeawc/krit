package stubs

import app.cash.turbine.test
import kotlinx.coroutines.flow.Flow

suspend fun turbineSmoke(flow: Flow<Int>) {
    flow.test {
        awaitItem()
        expectNoEvents()
        expectMostRecentItem()
        awaitComplete()
        awaitError()
        cancelAndIgnoreRemainingEvents()
    }
}
