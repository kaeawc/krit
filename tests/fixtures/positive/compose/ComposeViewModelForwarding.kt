package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun Home(viewModel: HomeScreenViewModel) {
    HomeContent(viewModel)
}

@Composable
fun HomeContent(viewModel: HomeScreenViewModel) {
    Text(viewModel.toString())
}
