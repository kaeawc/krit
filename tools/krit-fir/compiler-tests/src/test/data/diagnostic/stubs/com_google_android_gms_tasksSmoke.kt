// Smoke: Play services Task state getters read as synthetic properties.
package stubs

import com.google.android.gms.tasks.Task

fun describePlayServicesTask(task: Task<String>): String = when {
    !task.isComplete -> "pending"
    task.isCanceled -> "canceled"
    task.isSuccessful -> task.result
    else -> task.exception?.message ?: "failed"
}
