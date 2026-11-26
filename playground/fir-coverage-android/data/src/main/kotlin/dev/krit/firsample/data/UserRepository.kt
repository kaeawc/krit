package dev.krit.firsample.data

import com.squareup.moshi.Moshi
import dev.krit.firsample.model.Profile
import dev.krit.firsample.proto.UserRecord
import dev.krit.firsample.proto.userRecord

class UserRepository(moshi: Moshi = Moshi.Builder().build()) {
    // Refers to the KSP-generated adapter directly, so this file compiles
    // only when the generated sources are on FIR's source path.
    private val adapter = UserDtoJsonAdapter(moshi)

    fun parse(json: String): UserDto? = adapter.fromJson(json)

    fun toRecord(dto: UserDto): UserRecord = userRecord(dto.id, dto.name, dto.visits)

    fun toProfile(record: UserRecord): Profile = Profile(id = record.id, displayName = record.name)
}
