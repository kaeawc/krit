package com.example.services

import com.example.models.User
import java.text.SimpleDateFormat
import java.util.Date
import java.util.UUID
import java.security.MessageDigest

class UserService {

    private val users = mutableMapOf<String, User>()

    // EmptyFunctionBlock
    fun initialize() {
    }

    // UnusedParameter: 'format' is never used
    fun getAllUsers(format: String = "json"): List<User> {
        return users.values.toList()
    }

    fun getUserById(id: String): User? {
        return users[id]
    }

    fun createUser(user: User): User {
        val newUser = user.copy(id = UUID.randomUUID().toString())
        users[newUser.id] = newUser
        return newUser
    }

    fun deleteUser(id: String) {
        users.remove(id)
    }

    // LongMethod (60+ lines) + MagicNumber + SimpleDateFormat without Locale
    fun generateReport(userIds: List<String>, includeStats: Boolean, outputPath: String): String {
        val sb = StringBuilder()
        val dateFormat = SimpleDateFormat("yyyy-MM-dd HH:mm:ss")
        sb.append("Report generated at: ${dateFormat.format(Date())}\n")
        sb.append("==========================================\n")

        var totalAge = 0
        var minAge = 999
        var maxAge = 0
        var activeCount = 0
        var inactiveCount = 0
        var adminCount = 0
        var userCount = 0
        var moderatorCount = 0

        for (id in userIds) {
            val user = users[id] ?: continue
            sb.append("User: ${user.name}\n")
            sb.append("  Email: ${user.email}\n")
            sb.append("  Age: ${user.age}\n")
            sb.append("  Role: ${user.role}\n")

            totalAge += user.age
            if (user.age < minAge) {
                minAge = user.age
            }
            if (user.age > maxAge) {
                maxAge = user.age
            }

            when (user.role) {
                "admin" -> adminCount++
                "user" -> userCount++
                "moderator" -> moderatorCount++
            }

            if (user.age >= 18) {
                activeCount++
            } else {
                inactiveCount++
            }

            sb.append("  Status: ${if (user.age >= 18) "active" else "inactive"}\n")
            sb.append("  Hash: ${hashEmail(user.email)}\n")
            sb.append("---\n")
        }

        if (includeStats) {
            sb.append("\n")
            sb.append("==========================================\n")
            sb.append("Statistics:\n")
            sb.append("  Total users: ${userIds.size}\n")
            sb.append("  Average age: ${if (userIds.isNotEmpty()) totalAge / userIds.size else 0}\n")
            sb.append("  Min age: $minAge\n")
            sb.append("  Max age: $maxAge\n")
            sb.append("  Active: $activeCount\n")
            sb.append("  Inactive: $inactiveCount\n")
            sb.append("  Admins: $adminCount\n")
            sb.append("  Users: $userCount\n")
            sb.append("  Moderators: $moderatorCount\n")
            sb.append("==========================================\n")
        }

        val result = sb.toString()
        return result
    }

    // SwallowedException
    private fun hashEmail(email: String): String {
        try {
            val digest = MessageDigest.getInstance("SHA-256")
            val bytes = digest.digest(email.toByteArray())
            return bytes.joinToString("") { "%02x".format(it) }
        } catch (e: Exception) {
            return ""
        }
    }

    // EmptyCatchBlock
    fun parseAge(input: String): Int {
        try {
            return input.toInt()
        } catch (e: NumberFormatException) {
        }
        return 0
    }

    // ReturnFromFinally
    fun loadConfig(): String {
        try {
            return "production"
        } finally {
            return "default"
        }
    }
}
