package fixtures.positive.resourcecost

class RetrofitCreateInHotPath {
    fun getApi(): Any {
        return Retrofit.Builder()
            .baseUrl("https://api.example.com/")
            .build()
            .create(ApiService::class.java)
    }
}
