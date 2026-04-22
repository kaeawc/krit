package com.example.prod

import com.example.prod.FakeUser as ProdFake

class Prod {
    // FakeUser should not be reported from a comment.
    val text = "FakeUser should not be reported from a string"
    val unresolved: MissingFixture? = null
    val local = ProdFake()
}

