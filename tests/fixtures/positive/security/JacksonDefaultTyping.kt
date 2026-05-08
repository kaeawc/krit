package test

import com.fasterxml.jackson.databind.ObjectMapper

class MapperFactory {
    fun unsafe(): ObjectMapper {
        return ObjectMapper().enableDefaultTyping()
    }
}
