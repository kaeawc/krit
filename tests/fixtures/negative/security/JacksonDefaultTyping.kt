package test

import com.fasterxml.jackson.annotation.JsonTypeInfo
import com.fasterxml.jackson.databind.ObjectMapper

@JsonTypeInfo(use = JsonTypeInfo.Id.NAME)
sealed class Event

class MapperFactory {
    fun safe(): ObjectMapper {
        return ObjectMapper().findAndRegisterModules()
    }
}
