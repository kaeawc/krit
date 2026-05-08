package test

import com.google.gson.Gson

data class User(val id: String)
class LocalGson {
    fun fromJson(raw: String, type: Any) = Any()
}

fun safeGson(raw: String, local: LocalGson, type: Class<*>) {
    Gson().fromJson(raw, User::class.java)
    Gson().fromJson(raw, type)
    local.fromJson(raw, Any::class.java)
}
