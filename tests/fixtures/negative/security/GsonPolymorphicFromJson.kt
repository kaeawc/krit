package test

import com.google.gson.Gson

data class User(val id: String)

fun safeGson(raw: String) {
    Gson().fromJson(raw, User::class.java)
}
