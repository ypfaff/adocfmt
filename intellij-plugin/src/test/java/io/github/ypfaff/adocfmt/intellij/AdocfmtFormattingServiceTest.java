package io.github.ypfaff.adocfmt.intellij;

import com.intellij.application.options.CodeStyle;
import com.intellij.application.options.codeStyle.excludedFiles.GlobPatternDescriptor;
import com.intellij.formatting.service.FormattingService;
import com.intellij.openapi.command.WriteCommandAction;
import com.intellij.psi.PsiFile;
import com.intellij.psi.codeStyle.CodeStyleManager;
import com.intellij.testFramework.ExtensionTestUtil;
import com.intellij.testFramework.fixtures.BasePlatformTestCase;
import java.io.ByteArrayOutputStream;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.List;

// Runs the binary that Gradle builds into the test sandbox.
public class AdocfmtFormattingServiceTest extends BasePlatformTestCase {
  private static final String UNFORMATTED = "= Title\n\n\n\nOne. Two.\n";
  private static final String FORMATTED = "= Title\n\nOne.\nTwo.\n";
  private static final String REFUSED = "ifdef::extra[]\n\n----\ncode\n";

  @Override
  protected void setUp() throws Exception {
    super.setUp();
    // The light project lives on from test to test, and so does its switch.
    AdocfmtSettings.getInstance(getProject()).setEnabled(true);
  }

  public void testFormatsTheFile() {
    myFixture.configureByText("test.adoc", UNFORMATTED);
    reformat();
    myFixture.checkResult(FORMATTED);
  }

  public void testFormatsTheWholeFileForARange() {
    PsiFile file = myFixture.configureByText("test.adoc", UNFORMATTED);
    WriteCommandAction.runWriteCommandAction(
        getProject(),
        () -> CodeStyleManager.getInstance(getProject()).reformatText(file, 0, "= Title".length()));
    myFixture.checkResult(FORMATTED);
  }

  public void testReportsTheFindingsOfARefusal() {
    myFixture.configureByText("test.adoc", REFUSED);
    String errors = reformatCapturingErrors();
    myFixture.checkResult(REFUSED);
    assertTrue(
        errors,
        errors.contains(
            "adocfmt refused to format test.adoc: Line 1: conditional region has no endif"
                + "<br>Line 3: block has no closing delimiter"));
  }

  public void testLeavesTheFileWhenSwitchedOff() {
    AdocfmtSettings.getInstance(getProject()).setEnabled(false);
    myFixture.configureByText("test.adoc", UNFORMATTED);
    reformat();
    myFixture.checkResult(UNFORMATTED);
  }

  public void testLeavesAFileInADoNotFormatScope() {
    myFixture.configureByText("test.adoc", UNFORMATTED);
    CodeStyle.doWithTemporarySettings(
        getProject(),
        CodeStyle.getSettings(getProject()),
        settings -> {
          settings.getExcludedFiles().addDescriptor(new GlobPatternDescriptor("*.adoc"));
          reformat();
        });
    myFixture.checkResult(UNFORMATTED);
  }

  public void testReportsAMissingBinary() {
    Path withoutBinary = Path.of("no-plugin-here");
    ExtensionTestUtil.maskExtensions(
        FormattingService.EP_NAME,
        List.of(new AdocfmtFormattingService(withoutBinary)),
        getTestRootDisposable());
    myFixture.configureByText("test.adoc", UNFORMATTED);
    String errors = reformatCapturingErrors();
    myFixture.checkResult(UNFORMATTED);
    assertTrue(errors, errors.contains("adocfmt could not run: The adocfmt binary is missing"));
  }

  private void reformat() {
    WriteCommandAction.runWriteCommandAction(
        getProject(),
        () -> {
          CodeStyleManager.getInstance(getProject()).reformat(myFixture.getFile());
        });
  }

  // A headless IDE prints what would be a notification to stderr.
  private String reformatCapturingErrors() {
    PrintStream stderr = System.err;
    ByteArrayOutputStream errors = new ByteArrayOutputStream();
    System.setErr(new PrintStream(errors, true, StandardCharsets.UTF_8));
    try {
      reformat();
    } finally {
      System.setErr(stderr);
    }
    return errors.toString(StandardCharsets.UTF_8);
  }
}
