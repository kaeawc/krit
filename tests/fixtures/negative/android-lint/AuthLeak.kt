package com.example

class Endpoints {
    val noPassword = "https://user@example.com/path"
    val formatted = "ftp://deploy:%s@mirror.example.com"
    fun interpolated(user: String) = "http://$user@example.com"
    fun split(password: String) = "http://admin:${password}@db.example.com"
}
