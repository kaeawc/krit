// Compiler-test source stubs; never packaged in the production artifact.
package retrofit2

import okhttp3.OkHttpClient

class Retrofit private constructor() {
    fun <T> create(service: Class<T>): T = TODO()

    fun newBuilder(): Builder = TODO()

    class Builder {
        fun baseUrl(baseUrl: String): Builder = TODO()

        fun client(client: OkHttpClient): Builder = TODO()

        fun addConverterFactory(factory: Converter.Factory): Builder = TODO()

        fun build(): Retrofit = TODO()
    }
}

// retrofit2's Kotlin extension (KotlinExtensions.kt).
inline fun <reified T> Retrofit.create(): T = TODO()

interface Call<T> : Cloneable {
    fun execute(): Response<T>

    fun enqueue(callback: Callback<T>)

    fun isExecuted(): Boolean

    fun cancel()

    public override fun clone(): Call<T>
}

interface Callback<T> {
    fun onResponse(call: Call<T>, response: Response<T>)

    fun onFailure(call: Call<T>, t: Throwable)
}

// Java final class: body()/code() are plain methods; isSuccessful() is read
// as a synthetic property.
class Response<T> private constructor() {
    val isSuccessful: Boolean
        get() = TODO()

    fun body(): T? = TODO()

    fun code(): Int = TODO()

    fun message(): String = TODO()

    fun errorBody(): okhttp3.ResponseBody? = TODO()
}

class HttpException(response: Response<*>) : RuntimeException() {
    fun code(): Int = TODO()
}

interface Converter<F, T> {
    fun convert(value: F): T?

    abstract class Factory
}
