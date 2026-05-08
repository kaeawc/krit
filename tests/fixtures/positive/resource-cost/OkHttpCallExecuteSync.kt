package fixtures.positive.resourcecost

class OkHttpCallExecuteSync {
    suspend fun fetchData(call: okhttp3.Call): String {
        val response = call.execute()
        return response.body?.string() ?: ""
    }
}
