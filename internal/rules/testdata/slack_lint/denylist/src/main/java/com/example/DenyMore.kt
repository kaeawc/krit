package com.example

import io.reactivex.rxjava3.core.Completable
import io.reactivex.rxjava3.core.Flowable
import io.reactivex.rxjava3.core.Maybe
import io.reactivex.rxjava3.subjects.PublishSubject
import java.text.DateFormat
import java.util.*

class DenyMore(private val subject: PublishSubject<Int>) {
  fun subjects(maybe: Maybe<Int>, flowable: Flowable<Int>, completable: Completable) {
    subject.hide()
    maybe.blockingGet()
    flowable.blockingSubscribe()
    completable.blockingAwait()
    (maybe).hide()
  }

  fun starImported() {
    val v = Vector<Int>()
    val cal: Calendar = Calendar.getInstance()
    cal.timeInMillis
    cal.add(Calendar.DAY_OF_MONTH, 1)
    val format = DateFormat.getDateInstance(DateFormat.SHORT)
    format.parse("x")
    java.util.Calendar.MONTH
    Date().getTime()
  }

  fun members(date: Date?, format: java.text.SimpleDateFormat, cal: Calendar) {
    date?.let { it }
    format.isLenient
    format.javaClass
    format.toPattern()
    cal.time.time
    date.toString()
  }

  fun notListed(list: MutableList<Int>) {
    // Date() and Calendar.getInstance() in a comment
    val s = "LinkedList() Date() Calendar.YEAR"
    list.add(1)
    val text = s.length
  }
}
