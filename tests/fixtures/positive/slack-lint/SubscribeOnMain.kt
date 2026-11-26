package fixtures.positive.slacklint

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers
import io.reactivex.rxjava3.core.Observable

class Loader(private val source: Observable<String>) {
    fun load() = source
        .subscribeOn(AndroidSchedulers.mainThread())
        .subscribe()
}
