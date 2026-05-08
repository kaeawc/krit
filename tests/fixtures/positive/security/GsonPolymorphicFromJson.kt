package test

import com.google.gson.Gson

fun unsafeGson(raw: String) {
    Gson().fromJson(raw, Any::class.java)
}
