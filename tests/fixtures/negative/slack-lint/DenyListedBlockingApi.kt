package fixtures.negative.slacklint

import io.reactivex.rxjava3.core.Single
import io.reactivex.rxjava3.disposables.Disposable

fun load(user: Single<String>, onUser: (String) -> Unit): Disposable = user.subscribe(onUser)
