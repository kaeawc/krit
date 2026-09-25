// Smoke: remaining Retrofit HTTP annotations.
package stubs

import retrofit2.http.DELETE
import retrofit2.http.Headers
import retrofit2.http.PUT
import retrofit2.http.Path

interface AdminApi {
    @Headers("Accept: application/json", "X-Admin: 1")
    @PUT("items/{id}")
    suspend fun put(@Path("id") id: Long)

    @DELETE("items/{id}")
    suspend fun delete(@Path(value = "id", encoded = true) id: Long)
}
