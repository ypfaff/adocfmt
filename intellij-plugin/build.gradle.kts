import groovy.json.JsonSlurper
import org.jetbrains.intellij.platform.gradle.IntelliJPlatformType
import org.jetbrains.intellij.platform.gradle.TestFrameworkType
import org.jetbrains.intellij.platform.gradle.models.ProductRelease
import org.jetbrains.intellij.platform.gradle.tasks.PrepareSandboxTask

plugins {
    java
    id("org.jetbrains.intellij.platform") version "2.19.0"
    id("com.diffplug.spotless") version "8.10.4"
}

// The release sets the adocfmt version.
val releaseVersion = providers.gradleProperty("pluginVersion")
version = releaseVersion.getOrElse("0.0.0-dev")

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(21)
    }
}

repositories {
    mavenCentral()
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    intellijPlatform {
        intellijIdeaCommunity("2024.3")
        testFramework(TestFrameworkType.Platform)
    }
    testImplementation("junit:junit:4.13.2")
}

intellijPlatform {
    pluginConfiguration {
        ideaVersion {
            sinceBuild = "243"
            // Unset, because the default 243.* would disable the plugin in
            // every newer IDE until the next release.
            untilBuild = provider { null }
        }
        // What's New on the Marketplace links the release notes on GitHub. A
        // build without a release version has none.
        changeNotes = releaseVersion.map {
            """<a href="https://github.com/ypfaff/adocfmt/releases/tag/v$it">Release notes of adocfmt $it</a>"""
        }
    }
    // The release signs the plugin; a build without these leaves it unsigned.
    signing {
        certificateChain = providers.environmentVariable("INTELLIJ_CERTIFICATE_CHAIN")
        privateKey = providers.environmentVariable("INTELLIJ_PRIVATE_KEY")
        password = providers.environmentVariable("INTELLIJ_PRIVATE_KEY_PASSWORD")
    }
    pluginVerification {
        ides {
            current()
            // IntelliJ IDEA is one product since 2025.3; without the channel,
            // the latest would be an EAP.
            latest {
                types = listOf(IntelliJPlatformType.IntellijIdea)
                channels = listOf(ProductRelease.Channel.RELEASE)
            }
        }
    }
}

val repository = layout.projectDirectory.dir("..")

// adocfmt for this machine alone, in bin/<GOOS>-<GOARCH>/.
val buildAdocfmt = tasks.register<Exec>("buildAdocfmt") {
    val binaries = layout.buildDirectory.dir("adocfmt")
    val platform = providers.exec { commandLine("go", "env", "GOOS", "GOARCH") }
        .standardOutput.asText.get().trim().lines().joinToString("-")
    workingDir = repository.asFile
    environment("CGO_ENABLED", "0")
    // The trailing slash makes go build name the file, adocfmt.exe on Windows.
    commandLine("go", "build", "-trimpath", "-o", "${binaries.get().dir(platform).asFile}/", "./cmd/adocfmt")
    outputs.dir(binaries)
    doNotTrackState("go build keeps its own cache")
}

// The release passes GoReleaser's dist/ to bundle its binaries for all six
// platforms instead.
val goreleaserDist = providers.gradleProperty("goreleaserDist")

// The binaries in dist/ by GOOS-GOARCH. Their directories carry suffixes such as
// _v1, so the paths come from artifacts.json, which gives them from the
// directory GoReleaser ran in.
fun goreleaserBinaries(dist: File): Map<String, File> {
    @Suppress("UNCHECKED_CAST")
    val artifacts = JsonSlurper().parse(dist.resolve("artifacts.json")) as List<Map<String, Any>>
    return artifacts.filter { it["type"] == "Binary" }.associate {
        "${it["goos"]}-${it["goarch"]}" to dist.parentFile.resolve(it["path"] as String)
    }
}

tasks.withType<PrepareSandboxTask>().configureEach {
    into(pluginName.map { "$it/bin" }) {
        if (goreleaserDist.isPresent) {
            goreleaserBinaries(file(goreleaserDist.get())).forEach { (platform, binary) ->
                from(binary) { into(platform) }
            }
        } else {
            from(buildAdocfmt)
        }
    }
    // The Apache License section 4 requires both with the binaries.
    from(repository.files("LICENSE", "NOTICE")) {
        into(pluginName)
    }
}

tasks.buildPlugin {
    // An archive gets every file as 644, without the executable bit.
    filesMatching("bin/**") {
        permissions { unix("755") }
    }
}

spotless {
    java {
        // No version, so google-java-format moves with each Spotless update,
        // which Renovate proposes; it would not find a version written here.
        googleJavaFormat()
    }
}
