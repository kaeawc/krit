package com.example

import dagger.Binds
import dagger.Module

interface Repo
class Cache

@Module
interface RepoModule {
    @Binds
    fun bindRepo(impl: Cache): Repo
}
