package dev.krit.firsample.app

import dev.krit.firsample.data.UserRepository
import dev.krit.firsample.model.ExperimentalSampleApi
import dev.krit.firsample.model.initials

class ProfilePresenter(private val repository: UserRepository = UserRepository()) {
    // :model opts in for its own sources; callers in other modules still must.
    @OptIn(ExperimentalSampleApi::class)
    fun label(json: String): String? {
        val dto = repository.parse(json) ?: return null
        val profile = repository.toProfile(repository.toRecord(dto))
        return "${profile.initials()}: ${profile.displayName} (${dto.visits})"
    }
}
