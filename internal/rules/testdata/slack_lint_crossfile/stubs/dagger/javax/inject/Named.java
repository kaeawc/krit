package javax.inject;
import java.lang.annotation.*;
@Qualifier @Documented @Retention(RetentionPolicy.RUNTIME)
public @interface Named { String value() default ""; }
