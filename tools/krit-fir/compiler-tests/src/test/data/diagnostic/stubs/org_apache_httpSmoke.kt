// Smoke: read the status of an Apache HttpResponse.
package stubs

import org.apache.http.HttpResponse

fun statusOf(response: HttpResponse): Int = response.statusLine.statusCode
