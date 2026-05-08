package test

import com.fasterxml.jackson.annotation.JsonTypeInfo
import com.fasterxml.jackson.databind.ObjectMapper

class LocalMapper {
    fun activateDefaultTyping() = Unit
}

@JsonTypeInfo(use = JsonTypeInfo.Id.NAME)
sealed class Event

class MapperFactory {
    fun safe(local: LocalMapper): ObjectMapper {
        local.activateDefaultTyping()
        return ObjectMapper().findAndRegisterModules()
    }
}
