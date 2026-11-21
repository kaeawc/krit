package androidx.compose.runtime.rxjava3

import androidx.compose.runtime.Composable
import androidx.compose.runtime.State
import io.reactivex.rxjava3.core.Observable

@Composable
fun <T : R, R> Observable<T>.subscribeAsState(initial: R): State<R> = TODO()
