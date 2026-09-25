// Compiler-test source stubs; never packaged in the production artifact.
// TLS types come from the JDK's javax.net.ssl; they are never stubbed here.
package okhttp3

import java.io.Closeable
import java.io.IOException
import java.util.concurrent.TimeUnit
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.SSLSocketFactory
import javax.net.ssl.X509TrustManager

open class OkHttpClient internal constructor(builder: Builder) : Call.Factory {
    constructor() : this(Builder())

    val hostnameVerifier: HostnameVerifier
        get() = TODO()

    val sslSocketFactory: SSLSocketFactory
        get() = TODO()

    val interceptors: List<Interceptor>
        get() = TODO()

    override fun newCall(request: Request): Call = TODO()

    open fun newBuilder(): Builder = TODO()

    class Builder {
        fun connectTimeout(timeout: Long, unit: TimeUnit): Builder = TODO()

        fun connectTimeout(duration: java.time.Duration): Builder = TODO()

        fun readTimeout(timeout: Long, unit: TimeUnit): Builder = TODO()

        fun writeTimeout(timeout: Long, unit: TimeUnit): Builder = TODO()

        fun callTimeout(timeout: Long, unit: TimeUnit): Builder = TODO()

        fun addInterceptor(interceptor: Interceptor): Builder = TODO()

        fun addNetworkInterceptor(interceptor: Interceptor): Builder = TODO()

        fun hostnameVerifier(hostnameVerifier: HostnameVerifier): Builder = TODO()

        @Deprecated(
            "Use the sslSocketFactory overload that accepts a X509TrustManager.",
            level = DeprecationLevel.ERROR,
        )
        fun sslSocketFactory(sslSocketFactory: SSLSocketFactory): Builder = TODO()

        fun sslSocketFactory(sslSocketFactory: SSLSocketFactory, trustManager: X509TrustManager): Builder = TODO()

        fun certificatePinner(certificatePinner: CertificatePinner): Builder = TODO()

        fun retryOnConnectionFailure(retryOnConnectionFailure: Boolean): Builder = TODO()

        fun followRedirects(followRedirects: Boolean): Builder = TODO()

        fun build(): OkHttpClient = TODO()
    }
}

class Request internal constructor() {
    val method: String
        get() = TODO()

    fun header(name: String): String? = TODO()

    fun newBuilder(): Builder = TODO()

    open class Builder {
        open fun url(url: String): Builder = TODO()

        open fun header(name: String, value: String): Builder = TODO()

        open fun addHeader(name: String, value: String): Builder = TODO()

        open fun get(): Builder = TODO()

        open fun post(body: RequestBody): Builder = TODO()

        open fun put(body: RequestBody): Builder = TODO()

        open fun delete(body: RequestBody? = null): Builder = TODO()

        open fun build(): Request = TODO()
    }
}

class MediaType private constructor() {
    companion object {
        @JvmStatic
        @JvmName("get")
        fun String.toMediaType(): MediaType = TODO()

        @JvmStatic
        @JvmName("parse")
        fun String.toMediaTypeOrNull(): MediaType? = TODO()
    }
}

abstract class RequestBody {
    abstract fun contentType(): MediaType?

    companion object {
        @JvmStatic
        @JvmName("create")
        fun String.toRequestBody(contentType: MediaType? = null): RequestBody = TODO()
    }
}

abstract class ResponseBody : Closeable {
    abstract fun contentType(): MediaType?

    abstract fun contentLength(): Long

    fun string(): String = TODO()

    fun bytes(): ByteArray = TODO()

    override fun close() {
        TODO()
    }
}

class Response internal constructor() : Closeable {
    val code: Int
        get() = TODO()

    val message: String
        get() = TODO()

    val body: ResponseBody?
        get() = TODO()

    val isSuccessful: Boolean
        get() = TODO()

    val request: Request
        get() = TODO()

    fun header(name: String, defaultValue: String? = null): String? = TODO()

    override fun close() {
        TODO()
    }
}

interface Call : Cloneable {
    fun request(): Request

    @Throws(IOException::class)
    fun execute(): Response

    fun enqueue(responseCallback: Callback)

    fun cancel()

    fun isExecuted(): Boolean

    fun isCanceled(): Boolean

    public override fun clone(): Call

    fun interface Factory {
        fun newCall(request: Request): Call
    }
}

interface Callback {
    fun onFailure(call: Call, e: IOException)

    @Throws(IOException::class)
    fun onResponse(call: Call, response: Response)
}

fun interface Interceptor {
    @Throws(IOException::class)
    fun intercept(chain: Chain): Response

    interface Chain {
        fun request(): Request

        @Throws(IOException::class)
        fun proceed(request: Request): Response
    }
}

class CertificatePinner private constructor() {
    class Builder {
        fun add(pattern: String, vararg pins: String): Builder = TODO()

        fun build(): CertificatePinner = TODO()
    }
}
