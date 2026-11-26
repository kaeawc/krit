package fixtures.negative.androidlint

import androidx.work.Constraints

fun onlyIdle(): Constraints {
    val idle = Constraints.Builder()
    val charging = Constraints.Builder()
    idle.setRequiresDeviceIdle(true)
    charging.setRequiresCharging(true)
    idle.setRequiresCharging(false)
    return idle.build()
}

class LocalBuilder {
    fun setRequiresDeviceIdle(value: Boolean) = this
    fun setRequiresCharging(value: Boolean) = this
}

fun lookalike() {
    val builder = LocalBuilder()
    builder.setRequiresDeviceIdle(true)
    builder.setRequiresCharging(true)
}
