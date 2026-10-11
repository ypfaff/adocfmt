package io.github.ypfaff.adocfmt.intellij;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNull;
import static org.junit.Assert.assertThrows;
import static org.junit.Assert.assertTrue;
import static org.junit.Assume.assumeTrue;

import com.intellij.execution.ExecutionException;
import com.intellij.util.system.CpuArch;
import com.intellij.util.system.OS;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import org.junit.Rule;
import org.junit.Test;
import org.junit.rules.TemporaryFolder;

public class BinaryTest {
  @Rule public TemporaryFolder pluginDirectory = new TemporaryFolder();

  @Test
  public void platformIsNamedTheGoWay() {
    assertEquals("darwin-arm64", Binary.platform(OS.macOS, CpuArch.ARM64));
    assertEquals("darwin-amd64", Binary.platform(OS.macOS, CpuArch.X86_64));
    assertEquals("linux-arm64", Binary.platform(OS.Linux, CpuArch.ARM64));
    assertEquals("linux-amd64", Binary.platform(OS.Linux, CpuArch.X86_64));
    assertEquals("windows-arm64", Binary.platform(OS.Windows, CpuArch.ARM64));
    assertEquals("windows-amd64", Binary.platform(OS.Windows, CpuArch.X86_64));
  }

  @Test
  public void platformWithoutABinaryIsNone() {
    assertNull(Binary.platform(OS.FreeBSD, CpuArch.X86_64));
    assertNull(Binary.platform(OS.Linux, CpuArch.X86));
  }

  @Test
  public void fileNameHasTheWindowsExtension() {
    assertEquals("adocfmt.exe", Binary.fileName(OS.Windows));
    assertEquals("adocfmt", Binary.fileName(OS.Linux));
  }

  @Test
  public void chooseTakesTheCustomPathOverTheBundledBinary() throws Exception {
    Path root = pluginDirectory.getRoot().toPath();
    createBinary(binaryIn(root));
    assertEquals("/opt/adocfmt", Binary.choose("/opt/adocfmt", root));
  }

  @Test
  public void chooseFallsBackToTheBundledBinaryWithoutACustomPath() throws Exception {
    Path root = pluginDirectory.getRoot().toPath();
    Path binary = binaryIn(root);
    createBinary(binary);
    assertEquals(binary.toString(), Binary.choose("", root));
  }

  @Test
  public void findReportsAMissingBinary() {
    ExecutionException missing =
        assertThrows(
            ExecutionException.class, () -> Binary.find(pluginDirectory.getRoot().toPath()));
    assertTrue(
        missing.getMessage(), missing.getMessage().startsWith("The adocfmt binary is missing"));
  }

  @Test
  public void findSetsTheExecutableBit() throws Exception {
    assumeTrue(OS.CURRENT != OS.Windows);
    Path binary = binaryIn(pluginDirectory.getRoot().toPath());
    Files.createDirectories(binary.getParent());
    Files.createFile(
        binary, PosixFilePermissions.asFileAttribute(PosixFilePermissions.fromString("rw-r--r--")));

    assertEquals(binary, Binary.find(pluginDirectory.getRoot().toPath()));
    assertTrue(Files.isExecutable(binary));
  }

  private static void createBinary(Path binary) throws Exception {
    Files.createDirectories(binary.getParent());
    Files.createFile(binary);
  }

  private static Path binaryIn(Path pluginDirectory) {
    String platform = Binary.platform(OS.CURRENT, CpuArch.CURRENT);
    return pluginDirectory.resolve("bin").resolve(platform).resolve(Binary.fileName(OS.CURRENT));
  }
}
