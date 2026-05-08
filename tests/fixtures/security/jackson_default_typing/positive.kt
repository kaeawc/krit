package test

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.jsontype.impl.LaissezFaireSubTypeValidator

class MapperFactory {
    fun unsafe(): ObjectMapper {
        ObjectMapper().enableDefaultTyping()
        return ObjectMapper().activateDefaultTyping(LaissezFaireSubTypeValidator.instance)
    }
}
