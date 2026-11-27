package fixtures.androidlint

import androidx.lifecycle.Lifecycle
import androidx.lifecycle.testing.TestLifecycleOwner
import kotlinx.coroutines.test.runTest

fun resumes() = runTest {
    val owner = TestLifecycleOwner()
    owner.setCurrentState(Lifecycle.State.RESUMED)
}

fun blocking() {
    val owner = TestLifecycleOwner()
    owner.currentState = Lifecycle.State.RESUMED
}
