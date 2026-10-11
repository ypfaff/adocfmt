import org.jetbrains.intellij.platform.gradle.IntelliJPlatformType
import org.jetbrains.intellij.platform.gradle.models.ProductRelease

plugins {
    java
    id("org.jetbrains.intellij.platform") version "2.19.0"
    id("com.diffplug.spotless") version "8.10.3"
}

// The release sets the adocfmt version.
version = providers.gradleProperty("pluginVersion").getOrElse("0.0.0-dev")

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
    }
}

intellijPlatform {
    pluginConfiguration {
        ideaVersion {
            sinceBuild = "243"
            // Unset, because the default 243.* would disable the plugin in
            // every newer IDE until the next release.
            untilBuild = provider { null }
        }
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

spotless {
    java {
        // No version, so google-java-format moves with each Spotless update,
        // which Renovate proposes; it would not find a version written here.
        googleJavaFormat()
    }
}
