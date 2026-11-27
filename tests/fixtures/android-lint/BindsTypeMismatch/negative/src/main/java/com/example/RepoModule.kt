package com.example

import dagger.Binds
import dagger.Module

interface Repo
class RepoImpl : Repo

@Module
interface RepoModule {
    @Binds
    fun bindRepo(impl: RepoImpl): Repo
}
