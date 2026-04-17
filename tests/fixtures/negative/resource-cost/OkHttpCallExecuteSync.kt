package fixtures.negative.resourcecost

class OkHttpCallExecuteSync {
    fun fetchData(call: okhttp3.Call): String {
        val response = call.execute()
        return response.body?.string() ?: ""
    }
}
