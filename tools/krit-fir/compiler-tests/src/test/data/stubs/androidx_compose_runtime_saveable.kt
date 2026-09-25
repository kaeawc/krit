// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.runtime.saveable

import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState

interface Saver<Original, Saveable : Any> {
    fun SaverScope.save(value: Original): Saveable?

    fun restore(value: Saveable): Original?
}

fun interface SaverScope {
    fun canBeSaved(value: Any): Boolean
}

fun <Original, Saveable : Any> Saver(
    save: SaverScope.(value: Original) -> Saveable?,
    restore: (value: Saveable) -> Original?,
): Saver<Original, Saveable> = TODO()

fun <Original, Saveable> listSaver(
    save: SaverScope.(value: Original) -> List<Saveable>,
    restore: (list: List<Saveable>) -> Original?,
): Saver<Original, Any> = TODO()

fun <T> autoSaver(): Saver<T, Any> = TODO()

// The calculation parameter is named `init` and T is non-null.
@Composable
fun <T : Any> rememberSaveable(
    vararg inputs: Any?,
    saver: Saver<T, out Any> = autoSaver(),
    key: String? = null,
    init: () -> T,
): T = TODO()

@Composable
fun <T> rememberSaveable(
    vararg inputs: Any?,
    stateSaver: Saver<T, out Any>,
    key: String? = null,
    init: () -> MutableState<T>,
): MutableState<T> = TODO()
