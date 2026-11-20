package stubs

import org.koin.core.annotation.Singleton
import org.koin.core.annotation.Single

@Singleton
class KoinService

@Singleton
fun koinService(): KoinService = KoinService()

@Single
class SingleService

@Single
fun singleService(): SingleService = SingleService()
