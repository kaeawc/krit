package com.example.prod

import com.example.fixtures.FakeUser

class Prod {
    val user: FakeUser = FakeUser()
    val users: List<FakeUser> = listOf(FakeUser.sample())
}

