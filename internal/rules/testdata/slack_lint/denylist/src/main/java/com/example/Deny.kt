package com.example

import android.content.Context
import android.support.test.espresso.matcher.ViewMatchers.withId
import android.view.View
import androidx.core.content.ContextCompat
import androidx.core.view.ViewCompat
import androidx.viewpager2.widget.ViewPager2
import io.reactivex.rxjava3.core.Observable
import io.reactivex.rxjava3.core.Single
import io.reactivex.rxjava3.schedulers.Schedulers
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Calendar.YEAR
import java.util.Date
import java.util.LinkedList
import java.util.Stack
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.launchIn
import kotlinx.coroutines.rx3.rxCompletable
import kotlinx.coroutines.rx3.rxSingle
import kotlinx.coroutines.runBlocking

class Deny(private val ctx: Context, private val view: View, private val pager: ViewPager2) {
  fun rx(obs: Observable<String>, single: Single<Int>) {
    obs.hide()
    single.blockingGet()
    obs.blockingFirst()
    Schedulers.newThread()
    Schedulers.io()
  }

  fun android() {
    ContextCompat.getDrawable(ctx, 1)
    ContextCompat.getColor(ctx, 1)
    withId(1)
    view.setOnClickListener(null)
    view.setOnClickListener { }
    pager.setId(ViewCompat.generateViewId())
    pager.setId(View.generateViewId())
    pager.setId(3)
  }

  fun collections() {
    val a = LinkedList<String>()
    val b = Stack<Int>()
    val c = java.util.Vector<Int>()
    val d = ArrayList<Int>()
  }

  fun time() {
    val d = Date()
    d.getTime()
    val t = d.time
    val cal = Calendar.getInstance()
    cal.get(Calendar.YEAR)
    val y = YEAR
    val f = SimpleDateFormat("yyyy")
    f.format(d)
  }

  fun coroutines(flow: Flow<Int>, scope: CoroutineScope) {
    flow.launchIn(scope)
    rxCompletable { }
    rxCompletable(Dispatchers.IO) { }
    rxSingle { 1 }
    runBlocking { }
    runCatching { 1 }
    "x".runCatching { length }
  }
}
