package stubs

import androidx.navigation.NavType
import androidx.navigation.navArgument

fun NavigationSmoke() = navArgument("id") { type = NavType.IntType }
