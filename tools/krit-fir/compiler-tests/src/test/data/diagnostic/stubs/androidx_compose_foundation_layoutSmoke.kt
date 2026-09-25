// Smoke: Column/Row/Box scoped content lambdas and the layout modifier overloads.
package stubs

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun LayoutSmoke(padding: PaddingValues) {
    Column(
        modifier = Modifier.fillMaxSize().padding(padding),
        verticalArrangement = Arrangement.spacedBy(8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text("title", modifier = Modifier.align(Alignment.Start))
        Row(
            modifier = Modifier.fillMaxWidth().height(48.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Spacer(modifier = Modifier.weight(1f))
            Box(modifier = Modifier.size(24.dp).width(24.dp), contentAlignment = Alignment.Center) {
                Spacer(Modifier.align(Alignment.BottomEnd).size(4.dp, 4.dp))
            }
        }
        Spacer(
            Modifier
                .fillMaxHeight(0.5f)
                .padding(16.dp)
                .padding(horizontal = 16.dp, vertical = 8.dp)
                .padding(start = 4.dp, top = 2.dp),
        )
    }
    val uniform = PaddingValues(16.dp)
    val mixed = PaddingValues(horizontal = 8.dp)
    val sides = PaddingValues(start = 1.dp, top = 2.dp, end = 3.dp, bottom = 4.dp)
    println("$uniform $mixed $sides")
}
