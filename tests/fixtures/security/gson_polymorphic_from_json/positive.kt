package test

import com.google.gson.Gson
import com.google.gson.GsonBuilder

fun unsafeGson(raw: String) {
    Gson().fromJson(raw, Any::class.java)
    GsonBuilder().create().fromJson(raw, Object::class.java)
}
