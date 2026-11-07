package test

import android.app.Activity
import android.view.View

class SafeTagUsage {
    fun bindData(view: View, label: String, other: View) {
        view.setTag(label)
        view.setTag(other)
        view.setTag(1, label)
    }

    fun keyedActivity(view: View, activity: Activity) {
        view.setTag(1, activity)
    }

    fun unresolvedArg(view: View) {
        view.setTag(1, activity)
    }
}

class TagStore {
    fun setTag(key: Int, value: View) {}
}

fun unrelated(store: TagStore, other: View) {
    store.setTag(1, other)
}

fun commentsAndStrings() {
    // view.setTag(1, other)
    val sample = "view.setTag(1, other)"
}
