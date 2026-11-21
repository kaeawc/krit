package stubs

import androidx.compose.runtime.Composable
import androidx.compose.runtime.rxjava3.subscribeAsState
import io.reactivex.rxjava3.core.Observable

@Composable
fun RxStateSmoke(source: Observable<Int>) = source.subscribeAsState(initial = 0)
