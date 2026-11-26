package dagger;
import java.lang.annotation.*;
@Documented @Target(ElementType.TYPE) @Retention(RetentionPolicy.RUNTIME)
public @interface Module { Class<?>[] includes() default {}; Class<?>[] subcomponents() default {}; }
