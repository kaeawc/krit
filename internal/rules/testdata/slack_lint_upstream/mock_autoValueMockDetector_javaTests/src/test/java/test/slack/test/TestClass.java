package slack.test;

import org.mockito.Mock;
import org.mockito.Spy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.spy;

class MyTests {
  @Mock TestClass fieldMock;
  @Spy TestClass fieldSpy;
  @Mock TestClass.Builder fieldMock;
  @Spy TestClass.Builder fieldSpy;

  public void example() {
    TestClass localMock = mock(TestClass.class);
    TestClass localSpy = spy(localMock);
    Class<TestClass> classRef = TestClass.class;
    TestClass localMock2 = mock(classRef);

    TestClass.Builder builderLocalMock = mock(TestClass.Builder.class);
    TestClass.Builder builderLocalSpy = spy(builderLocalMock);
    Class<TestClass.Builder> builderClassRef = TestClass.Builder.class;
    TestClass.Builder builderLocalMock2 = mock(builderClassRef);
    TestClass fake = TestClass.builder().build();
  }
}
