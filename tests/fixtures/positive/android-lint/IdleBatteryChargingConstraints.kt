package fixtures.positive.androidlint

import androidx.work.Constraints

fun idleAndCharging(): Constraints {
    val builder = Constraints.Builder()
    builder.setRequiresDeviceIdle(true)
    builder.setRequiresCharging(true)
    return builder.build()
}
