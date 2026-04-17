package test
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel

@Preview
@Composable
fun FooPreview() {
    val vm: FooViewModel = hiltViewModel()
    Foo(vm)
}
