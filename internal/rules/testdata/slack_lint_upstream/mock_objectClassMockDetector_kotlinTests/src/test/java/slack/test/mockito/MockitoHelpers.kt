package slack.test.mockito

import org.mockito.Mockito

inline fun <reified T : Any> mock(): T = Mockito.mock(T::class.java)

inline fun <reified T : Any> mock(block: T.() -> Unit): T {
  return Mockito.mock(T::class.java).apply(block)
}
