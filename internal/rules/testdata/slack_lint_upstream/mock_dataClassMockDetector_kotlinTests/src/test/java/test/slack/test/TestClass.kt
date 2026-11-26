package slack.test

import org.mockito.Mock
import org.mockito.Spy
import slack.test.mockito.mock

class MyTests {
  @Mock lateinit var fieldMock: TestClass
  @Spy lateinit var fieldSpy: TestClass

  fun example() {
    val localMock1 = org.mockito.Mockito.mock(TestClass::class.java)
    val localSpy1 = org.mockito.Mockito.spy(localMock1)
    val localMock2 = mock<TestClass>()
    val classRef = TestClass::class.java
    val localMock3 = org.mockito.Mockito.mock(classRef)

    val dynamicMock = mock<TestClass> {

    }
    val assigned: TestClass = mock()
    val fake = TestClass("this is fine")

    // Extra tests for location reporting
    val unnecessaryMockedValues = TestClass(
      "This is fine",
      mock()
    )
    val unnecessaryNestedMockedValues = TestClass(
      "This is fine",
      listOf(mock())
    )
    val withNamedArgs = TestClass(
      foo = "This is fine",
      list = listOf(mock())
    )
  }
}
