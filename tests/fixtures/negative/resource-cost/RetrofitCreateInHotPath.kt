package fixtures.negative.resourcecost

import retrofit2.Retrofit

object RetrofitCreateInHotPath {
    private val api = Retrofit.Builder()
        .baseUrl("https://api.example.com/")
        .build()
        .create(ApiService::class.java)

    fun getApi(): Any = api
}
