          package slack.test;

          import org.mockito.Mock;
          import org.mockito.Spy;
          import static org.mockito.Mockito.mock;
          import static org.mockito.Mockito.spy;

          class MyTests {
            @Mock TestClass fieldMock;
            @Spy TestClass fieldSpy;

            public void example() {
              TestClass localMock = mock(TestClass.class);
              TestClass localSpy = spy(localMock);
              Class<TestClass> classRef = TestClass.class;
              TestClass localMock2 = mock(classRef);
              TestClass fake = new TestClass("this is fine");
            }

              // TODO uncomment once above Java record support is fixed
//            @Mock TestJavaClass fieldMock2;
//            @Spy TestJavaClass fieldSpy2;
//
//            public void example2() {
//              TestJavaClass localMock = mock(TestJavaClass.class);
//              TestJavaClass localSpy = spy(localMock);
//              Class<TestJavaClass> classRef = TestJavaClass.class;
//              TestJavaClass localMock2 = mock(classRef);
//              TestJavaClass fake = new TestJavaClass("this is fine");
//            }
          }
