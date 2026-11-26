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
            }

              // TODO uncomment once above Java record support is fixed
//            @Mock lateinit var fieldMock2: TestJavaClass
//            @Spy lateinit var fieldSpy2: TestJavaClass
//
//            fun example2() {
//              val localMock1 = org.mockito.Mockito.mock(TestJavaClass::class.java)
//              val localSpy1 = org.mockito.Mockito.spy(localMock1)
//              val localMock2 = mock<TestJavaClass>()
//              val classRef = TestJavaClass::class.java
//              val localMock3 = org.mockito.Mockito.mock(classRef)
//
//              val dynamicMock = mock<TestJavaClass> {
//
//              }
//              val assigned: TestJavaClass = mock()
//              val fake = TestJavaClass("this is fine")
//            }
          }
