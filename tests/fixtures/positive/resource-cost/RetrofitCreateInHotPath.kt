package fixtures.positive.resourcecost

import retrofit2.Retrofit

class RetrofitCreateInHotPath {
    fun getApi(): Any {
        return Retrofit.Builder()
            .baseUrl("https://api.example.com/")
            .build()
            .create(ApiService::class.java)
    }
}
