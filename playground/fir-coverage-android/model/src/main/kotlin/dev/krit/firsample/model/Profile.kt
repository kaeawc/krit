package dev.krit.firsample.model

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

@Serializable
data class Profile(
    val id: String,
    val displayName: String,
    val tags: List<String> = emptyList(),
)

private val json = Json { ignoreUnknownKeys = true }

// `Profile.serializer()` exists only after the kotlinx-serialization compiler
// plugin runs; without the plugin FIR reports it as unresolved.
fun decodeProfile(text: String): Profile = json.decodeFromString(Profile.serializer(), text)

fun encodeProfile(profile: Profile): String = json.encodeToString(Profile.serializer(), profile)

// Needs the module-wide `-opt-in=dev.krit.firsample.model.ExperimentalSampleApi`.
@ExperimentalSampleApi
fun Profile.initials(): String =
    displayName.split(' ').filter { it.isNotEmpty() }.joinToString("") { it.first().uppercase() }

fun Profile.label(): String = "${initials()} ($id)"
