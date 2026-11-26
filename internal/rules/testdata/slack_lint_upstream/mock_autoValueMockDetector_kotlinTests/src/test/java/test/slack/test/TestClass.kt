package slack.test

import org.mockito.Mock
import org.mockito.Spy
import slack.test.mockito.mock

class MyTests {
  @Mock lateinit var fieldMock: TestClass
  @Spy lateinit var fieldSpy: TestClass
  @Mock lateinit var fieldBuilderMock: TestClass.Builder
  @Spy lateinit var fieldBuilderSpy: TestClass.Builder

  fun example() {
    val localMock1 = org.mockito.Mockito.mock(TestClass::class.java)
    val localSpy1 = org.mockito.Mockito.spy(localMock1)
    val localMock2 = mock<TestClass>()
    val classRef = TestClass::class.java
    val localMock3 = org.mockito.Mockito.mock(classRef)

    val builderLocalMock1 = org.mockito.Mockito.mock(TestClass.Builder::class.java)
    val builderLocalSpy1 = org.mockito.Mockito.spy(builderLocalMock1)
    val builderLocalMock2 = mock<TestClass.Builder>()
    val builderClassRef = TestClass.Builder::class.java
    val builderLocalMock3 = org.mockito.Mockito.mock(classRef)
    val fake = TestClass.builder().build()
  }
}
