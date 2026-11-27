package com.example

import dagger.Binds
import dagger.Module
import com.example.other.Remote

interface Repo
interface Cache
open class BaseRepo : Repo
class RepoImpl : BaseRepo()
class CacheImpl : Cache
class Unrelated
class Box<T> : Repo

@Module
interface RepoModule {
  @Binds fun bindRepo(impl: RepoImpl): Repo
  @Binds fun bindWrong(impl: CacheImpl): Repo
  @Binds fun bindUnrelated(impl: Unrelated): Cache
  @Binds fun bindSame(impl: Repo): Repo
  @Binds fun bindRemote(impl: com.example.other.RemoteImpl): Remote
  @Binds fun bindRemoteWrong(impl: Unrelated): Remote
  @Binds fun bindBox(impl: Box<String>): Repo
  @Binds fun bindNullable(impl: CacheImpl?): Repo
  @Binds fun bindString(impl: String): CharSequence
  @Binds fun bindStringWrong(impl: String): Repo
}
