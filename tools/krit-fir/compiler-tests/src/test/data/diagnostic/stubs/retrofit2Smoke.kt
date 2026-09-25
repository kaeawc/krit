// Smoke: a Retrofit service interface, the builder, and the reified create().
package stubs

import okhttp3.OkHttpClient
import retrofit2.Call
import retrofit2.Callback
import retrofit2.Response
import retrofit2.Retrofit
import retrofit2.create
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Header
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

data class RetrofitUser(val id: String)

interface UserApi {
    @GET("users/{id}")
    suspend fun user(@Path("id") id: String, @Query("expand") expand: Boolean = false): RetrofitUser

    @POST("users")
    fun create(@Body user: RetrofitUser, @Header("Authorization") auth: String): Call<RetrofitUser>

    @GET("users")
    suspend fun list(): Response<List<RetrofitUser>>
}

fun retrofitSmoke(client: OkHttpClient) {
    val retrofit = Retrofit.Builder()
        .baseUrl("https://example.com/")
        .client(client)
        .build()
    val api: UserApi = retrofit.create(UserApi::class.java)
    val reified: UserApi = retrofit.create()
    api.create(RetrofitUser("1"), "token").enqueue(object : Callback<RetrofitUser> {
        override fun onResponse(call: Call<RetrofitUser>, response: Response<RetrofitUser>) {
            if (response.isSuccessful) <!PrintlnInProduction!>println<!>(response.body() ?: response.code())
        }

        override fun onFailure(call: Call<RetrofitUser>, t: Throwable) {}
    })
    <!PrintlnInProduction!>println<!>(reified)
}
