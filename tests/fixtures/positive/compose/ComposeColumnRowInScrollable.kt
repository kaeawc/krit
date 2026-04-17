package fixtures.positive.compose

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun ComposeColumnRowInScrollablePositive(items: List<String>) {
	Column(Modifier.verticalScroll(rememberScrollState())) {
		Header()
		LazyColumn {
			items(items) { item ->
				RowItem(item)
			}
		}
	}

	Row(Modifier.horizontalScroll(rememberScrollState())) {
		Sidebar()
		LazyRow {
			items(items) { item ->
				Chip(item)
			}
		}
	}
}

@Composable
fun Header() {}

@Composable
fun Sidebar() {}

@Composable
fun RowItem(item: String) {}

@Composable
fun Chip(item: String) {}
