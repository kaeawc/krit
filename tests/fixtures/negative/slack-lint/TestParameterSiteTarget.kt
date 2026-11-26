package fixtures.negative.slacklint

import com.google.testing.junit.testparameterinjector.TestParameter

class ColorTest(@param:TestParameter val color: String, @TestParameter plain: String)
