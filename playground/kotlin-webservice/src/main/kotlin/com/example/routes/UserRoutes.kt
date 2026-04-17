package com.example.routes

// WildcardImport violations
import io.ktor.server.application.*
import io.ktor.server.request.*
import io.ktor.server.response.*
import io.ktor.server.routing.*
import io.ktor.http.*
import com.example.models.*
import com.example.services.*
import java.util.*

fun Route.configureUserRoutes() {
    val service = UserService()

    route("/users") {
        get {
            val users = service.getAllUsers()
            call.respond(ApiResponse(data = users, success = true, message = "OK"))
        }

        get("/{id}") {
            val id = call.parameters["id"] ?: return@get call.respond(
                HttpStatusCode.BadRequest,
                ApiResponse<User>(data = null, success = false, message = "Missing id")
            )
            val user = service.getUserById(id)
            if (user != null) {
                call.respond(ApiResponse(data = user, success = true, message = "OK"))
            } else {
                call.respond(
                    HttpStatusCode.NotFound,
                    ApiResponse<User>(data = null, success = false, message = "Not found")
                )
            }
        }

        post {
            val request = call.receive<User>()
            val created = service.createUser(request)
            call.respond(HttpStatusCode.Created, ApiResponse(data = created, success = true, message = "Created"))
        }

        delete("/{id}") {
            val id = call.parameters["id"] ?: return@delete call.respond(
                HttpStatusCode.BadRequest,
                ApiResponse<Unit>(data = null, success = false, message = "Missing id")
            )
            service.deleteUser(id)
            call.respond(ApiResponse(data = Unit, success = true, message = "Deleted"))
        }
    }
}
