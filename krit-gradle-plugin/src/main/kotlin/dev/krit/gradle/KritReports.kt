package dev.krit.gradle

import org.gradle.api.file.RegularFileProperty
import org.gradle.api.model.ObjectFactory
import org.gradle.api.provider.Property
import javax.inject.Inject

/**
 * DSL for configuring krit report outputs.
 *
 * Usage:
 * ```
 * krit {
 *     reports {
 *         sarif {
 *             required.set(true)
 *             outputLocation.set(file("build/reports/krit/krit.sarif"))
 *         }
 *         json {
 *             required.set(true)
 *         }
 *     }
 * }
 * ```
 */
interface KritReports {
    val sarif: KritReport
    val json: KritReport
    val plain: KritReport
    val checkstyle: KritReport
}

interface KritReport {
    /** Whether this report format is enabled. Default: false (except sarif which defaults to true). */
    val required: Property<Boolean>
    /** Output file location. Has a sensible default based on format. */
    val outputLocation: RegularFileProperty
}

/**
 * Default implementation created via ObjectFactory so Gradle can inject managed properties.
 */
abstract class DefaultKritReports @Inject constructor(
    objects: ObjectFactory,
) : KritReports {
    override val sarif: KritReport = objects.newInstance(DefaultKritReport::class.java)
    override val json: KritReport = objects.newInstance(DefaultKritReport::class.java)
    override val plain: KritReport = objects.newInstance(DefaultKritReport::class.java)
    override val checkstyle: KritReport = objects.newInstance(DefaultKritReport::class.java)
}

abstract class DefaultKritReport : KritReport
