package test;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.jsontype.impl.LaissezFaireSubTypeValidator;

class MapperFactory {
    ObjectMapper unsafe() {
        new ObjectMapper().enableDefaultTyping();
        return new ObjectMapper().activateDefaultTyping(LaissezFaireSubTypeValidator.instance);
    }
}
