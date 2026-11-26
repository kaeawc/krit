package dagger.producers;
import java.lang.annotation.*;
@Documented @Target(ElementType.TYPE) @Retention(RetentionPolicy.RUNTIME)
public @interface ProducerModule { Class<?>[] includes() default {}; }
