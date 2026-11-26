package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

class Presenter(val viewModel: HomeScreenViewModel)

@Composable
fun Home(viewModel: HomeScreenViewModel) {
    Text(viewModel.title)
    LaunchedEffect(viewModel) {}
    val presenter = Presenter(viewModel)
    Text(presenter.toString())
}
