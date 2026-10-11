package io.github.ypfaff.adocfmt.intellij;

import com.intellij.execution.ExecutionException;
import com.intellij.util.system.CpuArch;
import com.intellij.util.system.OS;
import java.nio.file.Files;
import java.nio.file.Path;
import org.jetbrains.annotations.Nullable;

/** Finds the adocfmt binary to run: the user's own, or the one the plugin carries. */
final class Binary {
  private Binary() {}

  // The plugin leaves the user's own binary as it is, executable bit included.
  static String choose(String customPath, Path pluginDirectory) throws ExecutionException {
    return customPath.isBlank() ? find(pluginDirectory).toString() : customPath;
  }

  static Path find(Path pluginDirectory) throws ExecutionException {
    String platform = platform(OS.CURRENT, CpuArch.CURRENT);
    if (platform == null) {
      throw new ExecutionException(
          "The plugin has no adocfmt binary for " + OS.CURRENT + " on " + CpuArch.CURRENT);
    }
    Path binary = pluginDirectory.resolve("bin").resolve(platform).resolve(fileName(OS.CURRENT));
    if (!Files.isRegularFile(binary)) {
      throw new ExecutionException("The adocfmt binary is missing: " + binary);
    }
    makeExecutable(binary);
    return binary;
  }

  // The directory in bin, named GOOS-GOARCH like the Go build that made it.
  static @Nullable String platform(OS os, CpuArch arch) {
    String goos =
        switch (os) {
          case macOS -> "darwin";
          case Linux -> "linux";
          case Windows -> "windows";
          default -> null;
        };
    String goarch =
        switch (arch) {
          case X86_64 -> "amd64";
          case ARM64 -> "arm64";
          default -> null;
        };
    return goos == null || goarch == null ? null : goos + "-" + goarch;
  }

  static String fileName(OS os) {
    return os == OS.Windows ? "adocfmt.exe" : "adocfmt";
  }

  // The installer keeps the executable bit, but a plugin unpacked some other
  // way may have lost it.
  private static void makeExecutable(Path binary) throws ExecutionException {
    if (!Files.isExecutable(binary) && !binary.toFile().setExecutable(true)) {
      throw new ExecutionException("The adocfmt binary is not executable: " + binary);
    }
  }
}
