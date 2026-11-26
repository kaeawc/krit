package fixtures.positive.slacklint

import io.reactivex.rxjava3.core.Single

fun load(user: Single<String>): String = user.blockingGet()
