package fixtures.negative.resourcecost

object RetrofitCreateInHotPath {
    private val api = Retrofit.Builder()
        .baseUrl("https://api.example.com/")
        .build()
        .create(ApiService::class.java)

    fun getApi(): Any = api
}
