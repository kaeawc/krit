package com.example

import android.app.Activity
import android.app.Service

class SplashScreenHelper

class SplashService : Service() {
    override fun onBind(intent: android.content.Intent?) = null
}

class MainActivity : Activity()

object SplashScreenConfig

interface LaunchScreen

fun splashActivity() = Unit
