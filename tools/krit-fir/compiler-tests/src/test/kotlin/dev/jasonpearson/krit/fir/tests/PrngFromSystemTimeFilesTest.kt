package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// PrngFromSystemTime cases that need a source path or extra Java sources:
// the Go rule's own test-path markers, and the clock owners the shared stubs
// do not declare (android.icu.util.Calendar, org.threeten.bp.Instant).
class PrngFromSystemTimeFilesTest {

    private fun findings(
        sources: Map<String, String>,
        testFiles: Set<String> = emptySet(),
        scanPaths: Map<String, String> = emptyMap(),
    ): Map<String, Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("PrngFromSystemTime")),
            testFiles = testFiles,
            scanPaths = scanPaths,
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "PrngFromSystemTime" }.groupingBy { it.file }.eachCount()
    }

    private fun seeded(pkg: String) = """
        package $pkg

        import java.util.Random
        import javax.crypto.Cipher

        class Keys {
            fun rng(): Random = Random(System.currentTimeMillis())

            fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
        }
    """.trimIndent()

    // Go skips a lower-cased path holding /src/test/ or /src/androidtest/,
    // unless it also holds /tests/fixtures/, on the scan spelling of the path.
    // A leading `src/test/` has no slash before `src`, so Go checks it.
    @Test fun goTestPathMarkersAreMatchedOnTheScanSpelling() {
        val sources = mapOf(
            "app/src/main/kotlin/MainKeys.kt" to seeded("main"),
            "app/src/test/kotlin/UnitKeys.kt" to seeded("unit"),
            "app/src/androidTest/kotlin/DeviceKeys.kt" to seeded("device"),
            "repo/tests/fixtures/app/src/test/FixtureKeys.kt" to seeded("fixture"),
            "src/test/RootKeys.kt" to seeded("root"),
        )
        val relative = sources.keys.associateWith { it }
        assertEquals(
            mapOf("MainKeys.kt" to 1, "FixtureKeys.kt" to 1, "RootKeys.kt" to 1),
            findings(sources, scanPaths = relative),
        )
    }

    // The Go rule does not consult scanner.IsTestFile, so a request-listed
    // test file on a production path is still checked.
    @Test fun requestListedTestFileIsStillChecked() {
        val sources = mapOf("Listed.kt" to seeded("listed"))
        assertEquals(mapOf("Listed.kt" to 1), findings(sources, testFiles = setOf("Listed.kt")))
    }

    // Go matches `Calendar.getInstance().timeInMillis` and
    // `Instant.now().toEpochMilli()` whatever the owner's package, so the ICU
    // Calendar and the ThreeTen backport Instant, both system clock reads,
    // must be reported. The Java sources stand in for the platform/library.
    @Test fun icuCalendarAndThreeTenInstantAreClockReads() {
        val sources = mapOf(
            "android/icu/util/Calendar.java" to """
                package android.icu.util;

                public abstract class Calendar {
                    public static Calendar getInstance() { return null; }
                    public long getTimeInMillis() { return 0L; }
                }
            """.trimIndent(),
            "org/threeten/bp/Instant.java" to """
                package org.threeten.bp;

                public final class Instant {
                    public static Instant now() { return null; }
                    public long toEpochMilli() { return 0L; }
                }
            """.trimIndent(),
            "Clocks.kt" to """
                package clocks

                import android.icu.util.Calendar
                import java.security.SecureRandom
                import java.util.Random
                import org.threeten.bp.Instant

                fun icu(): Random = Random(Calendar.getInstance().timeInMillis)

                fun icuGetter(): Random = Random(Calendar.getInstance().getTimeInMillis())

                fun threeTen(): Random = Random(Instant.now().toEpochMilli())

                fun secure(): SecureRandom = SecureRandom()
            """.trimIndent(),
        )
        assertEquals(mapOf("Clocks.kt" to 3), findings(sources))
    }

    // Lines of [file] that report the rule.
    private fun lines(
        sources: Map<String, String>,
        file: String,
    ): List<Int> {
        val result = KritFirProbe.compile(
            sources,
            FirRuleCompileContext(enabledRuleIds = setOf("PrngFromSystemTime")),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "PrngFromSystemTime" && it.file == file }.map { it.line }.sorted()
    }

    // Go matches `Calendar.getInstance().timeInMillis` whatever the owner's
    // package, so the ICU4J Calendar (and a subclass, through the inherited
    // static) is a system clock read. The Java sources stand in for ICU4J.
    @Test fun icu4jCalendarIsAClockRead() {
        val sources = mapOf(
            "com/ibm/icu/util/Calendar.java" to """
                package com.ibm.icu.util;

                public abstract class Calendar {
                    public static Calendar getInstance() { return null; }
                    public static Calendar getInstance(TimeZone zone) { return null; }
                    public long getTimeInMillis() { return 0L; }
                }
            """.trimIndent(),
            "com/ibm/icu/util/GregorianCalendar.java" to """
                package com.ibm.icu.util;

                public class GregorianCalendar extends Calendar {
                }
            """.trimIndent(),
            "com/ibm/icu/util/TimeZone.java" to """
                package com.ibm.icu.util;

                public abstract class TimeZone {
                    public static TimeZone getDefault() { return null; }
                }
            """.trimIndent(),
            "Icu.kt" to """
                package clocks

                import com.ibm.icu.util.Calendar
                import com.ibm.icu.util.GregorianCalendar
                import com.ibm.icu.util.TimeZone
                import java.util.Random
                import javax.crypto.Cipher

                fun icu(): Random = Random(Calendar.getInstance().timeInMillis)

                fun icuGetter(): Random = Random(Calendar.getInstance().getTimeInMillis())

                fun qualified(): Random = Random(com.ibm.icu.util.Calendar.getInstance().timeInMillis)

                fun gregorian(): Random = Random(GregorianCalendar.getInstance().timeInMillis)

                fun zoned(): Random = Random(Calendar.getInstance(TimeZone.getDefault()).timeInMillis)

                fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
            """.trimIndent(),
        )
        // Line 17 (a zoned Calendar) is reported by neither: Go's text is
        // `Calendar.getInstance(TimeZone.getDefault()).timeInMillis`.
        assertEquals(listOf(9, 11, 13, 15), lines(sources, "Icu.kt"))
    }

    private val jodaSources = mapOf(
        "org/joda/time/base/AbstractInstant.java" to """
            package org.joda.time.base;

            public abstract class AbstractInstant {
                public java.util.Date toDate() { return null; }
            }
        """.trimIndent(),
        "org/joda/time/DateTimeZone.java" to """
            package org.joda.time;

            public abstract class DateTimeZone {
                public static final DateTimeZone UTC = null;
            }
        """.trimIndent(),
        "org/joda/time/DateTime.java" to """
            package org.joda.time;

            public final class DateTime extends org.joda.time.base.AbstractInstant {
                public DateTime() {}
                public DateTime(DateTimeZone zone) {}
                public DateTime(long instant) {}
                public static DateTime now() { return null; }
                public static DateTime now(DateTimeZone zone) { return null; }
            }
        """.trimIndent(),
        "org/joda/time/LocalDateTime.java" to """
            package org.joda.time;

            public final class LocalDateTime {
                public LocalDateTime() {}
                public static LocalDateTime now() { return null; }
                public java.util.Date toDate() { return null; }
            }
        """.trimIndent(),
    )

    // Go's `Date().time` / `Date().getTime()` seed substrings also match a
    // Joda-Time `toDate().time`. Built by a no-argument constructor or now()
    // (or one taking only a time zone), the value is the current instant, so
    // those are system clock reads and FIR reports them as Go does.
    @Test fun jodaToDateOfTheCurrentInstantIsAClockRead() {
        val sources = jodaSources + mapOf(
            "Joda.kt" to """
                package clocks

                import java.util.Random
                import javax.crypto.Cipher
                import org.joda.time.DateTime
                import org.joda.time.DateTimeZone
                import org.joda.time.LocalDateTime

                fun constructed(): Random = Random(DateTime().toDate().time)

                fun now(): Random = Random(DateTime.now().toDate().time)

                fun getter(): Random = Random(DateTime().toDate().getTime())

                fun zoned(): Random = Random(DateTime(DateTimeZone.UTC).toDate().time)

                fun nowZoned(): Random = Random(DateTime.now(DateTimeZone.UTC).toDate().getTime())

                fun local(): Random = Random(LocalDateTime().toDate().time)

                fun localNow(): Random = Random(LocalDateTime.now().toDate().time)

                fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
            """.trimIndent(),
        )
        assertEquals(listOf(9, 11, 13, 15, 17, 19, 21), lines(sources, "Joda.kt"))
    }

    // Go reports both calls below, because their seed text contains
    // `Date().time` (inside `toDate().time`). Neither seed is the system
    // time: one is a parameter's instant, the other the epoch. So FIR
    // reports neither (declared divergences).
    @Test fun jodaToDateOfAnotherInstantIsNotAClockRead() {
        val sources = jodaSources + mapOf(
            "JodaFixed.kt" to """
                package clocks

                import java.util.Random
                import javax.crypto.Cipher
                import org.joda.time.DateTime

                fun event(event: DateTime): Random = Random(event.toDate().time)

                fun epoch(): Random = Random(DateTime(0L).toDate().time)

                fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
            """.trimIndent(),
        )
        assertEquals(emptyList(), lines(sources, "JodaFixed.kt"))
    }

    // A Random subclass declared in another package: the calling file imports
    // util.Random and never mentions java.util.Random or kotlin.random.Random,
    // so Go's gate fails and Go reports nothing. The call still seeds a
    // java.util.Random from the system clock, so FIR reports it.
    @Test fun randomSubclassFromAnotherPackageIsReported() {
        val sources = mapOf(
            "util/Random.kt" to """
                package util

                class Random(seed: Long) : java.util.Random(seed)
            """.trimIndent(),
            "Use.kt" to """
                package app

                import javax.crypto.Cipher
                import util.Random

                fun seeded(): Random = Random(System.nanoTime())

                fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
            """.trimIndent(),
        )
        assertEquals(listOf(6), lines(sources, "Use.kt"))
    }

    // A same-package `typealias Random = java.util.Random` in another file:
    // the calling file never mentions java.util.Random, so Go reports
    // nothing. The call builds a java.util.Random seeded from the system
    // clock, so FIR reports it.
    @Test fun samePackageTypeAliasInAnotherFileIsReported() {
        val sources = mapOf(
            "Alias.kt" to """
                package app

                typealias Random = java.util.Random
            """.trimIndent(),
            "Use.kt" to """
                package app

                import javax.crypto.Cipher

                fun seeded(): Random = Random(System.nanoTime())

                fun cipher(): Cipher = Cipher.getInstance("AES/GCM/NoPadding")
            """.trimIndent(),
        )
        assertEquals(listOf(5), lines(sources, "Use.kt"))
    }
}
