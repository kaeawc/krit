package licensing

import kotlinx.coroutines.ExperimentalCoroutinesApi

/** Safe because the experimental flag is stable in our Kotlin version. */
@OptIn(ExperimentalCoroutinesApi::class)
fun useApi() {
    /* ... */
}
