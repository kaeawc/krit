package test

import android.app.Activity
import android.view.View

class SafeTagUsage {
    fun bindData(view: View, label: String, contextName: String) {
        view.setTag(label)
        view.setTag("activity")
        view.setTag(contextName)
    }

    fun keyedTag(view: View, activity: Activity) {
        view.setTag(1, activity)
    }

    fun unresolvedArg(view: View) {
        view.setTag(activity)
    }
}

class TagStore {
    fun setTag(value: Activity) {}
}

fun unrelated(store: TagStore, activity: Activity) {
    store.setTag(activity)
}

fun commentsAndStrings() {
    // view.setTag(activity)
    val sample = "view.setTag(activity)"
}
