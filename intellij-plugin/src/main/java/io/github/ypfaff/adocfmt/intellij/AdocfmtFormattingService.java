package io.github.ypfaff.adocfmt.intellij;

import com.intellij.application.options.CodeStyle;
import com.intellij.execution.ExecutionException;
import com.intellij.execution.configurations.GeneralCommandLine;
import com.intellij.execution.process.CapturingProcessHandler;
import com.intellij.execution.process.ProcessHandler;
import com.intellij.execution.process.ProcessOutput;
import com.intellij.formatting.service.AsyncDocumentFormattingService;
import com.intellij.formatting.service.AsyncFormattingRequest;
import com.intellij.openapi.extensions.PluginAware;
import com.intellij.openapi.extensions.PluginDescriptor;
import com.intellij.openapi.util.io.FileUtilRt;
import com.intellij.openapi.util.text.StringUtil;
import com.intellij.psi.PsiFile;
import java.io.IOException;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.EnumSet;
import java.util.List;
import java.util.Set;
import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

/** Formats AsciiDoc files with the adocfmt binary. */
public final class AdocfmtFormattingService extends AsyncDocumentFormattingService
    implements PluginAware {
  private Path pluginDirectory;

  public AdocfmtFormattingService() {}

  // A test points it at a directory without the binary.
  AdocfmtFormattingService(Path pluginDirectory) {
    this.pluginDirectory = pluginDirectory;
  }

  // The platform calls this right after it creates the service.
  @Override
  public void setPluginDescriptor(@NotNull PluginDescriptor plugin) {
    pluginDirectory = plugin.getPluginPath();
  }

  // The platform asks this for every file it formats, so it starts no process
  // and reads no file. CodeStyleManager does not skip the "Do not format"
  // scopes itself, so the check for them is here.
  @Override
  public boolean canFormat(@NotNull PsiFile file) {
    return isAsciiDoc(file.getName())
        && AdocfmtSettings.getInstance(file.getProject()).isEnabled()
        && CodeStyle.isFormattingEnabled(file);
  }

  // The extensions the CLI picks up in a directory, in any case.
  static boolean isAsciiDoc(String fileName) {
    String extension = FileUtilRt.getExtension(fileName);
    return extension.equalsIgnoreCase("adoc") || extension.equalsIgnoreCase("asciidoc");
  }

  // Without FORMAT_FRAGMENTS, formatting a selection or the changed lines would
  // fall back to the core formatter. adocfmt formats only whole documents, so
  // the ranges are ignored.
  @Override
  public @NotNull Set<Feature> getFeatures() {
    return EnumSet.of(Feature.FORMAT_FRAGMENTS);
  }

  // A paste with "Reformat block" and some quick fixes may change only
  // whitespace. adocfmt changes more than that, so it leaves the document as it
  // is.
  @Override
  protected @Nullable FormattingTask createFormattingTask(@NotNull AsyncFormattingRequest request) {
    if (request.canChangeWhitespaceOnly()) {
      return null;
    }
    return new Run(request, request.getContext().getContainingFile().getName());
  }

  @Override
  protected @NotNull String getNotificationGroupId() {
    return "adocfmt refused";
  }

  @Override
  protected @NotNull String getName() {
    return "adocfmt";
  }

  private final class Run implements FormattingTask {
    private final AsyncFormattingRequest request;
    private final String fileName;
    private volatile @Nullable ProcessHandler process;

    Run(AsyncFormattingRequest request, String fileName) {
      this.request = request;
      this.fileName = fileName;
    }

    // The platform takes only the first onTextReady or onError of a request,
    // and none once it is cancelled or timed out.
    @Override
    public void run() {
      ProcessOutput output;
      try {
        output = format(request.getDocumentText());
      } catch (ExecutionException e) {
        request.onError("adocfmt could not run", html(List.of(e.getMessage())));
        return;
      }
      // The platform reports the timeout itself.
      if (output.isTimeout()) {
        return;
      }
      switch (Outcome.of(output.getExitCode(), output.getStdout(), output.getStderrLines())) {
        case Outcome.Formatted formatted -> request.onTextReady(formatted.text());
        case Outcome.Refused refused ->
            request.onError("adocfmt refused to format " + fileName, html(refused.findings()));
        case Outcome.Crashed crashed ->
            request.onError("adocfmt crashed on " + fileName, html(crashReport(crashed)));
      }
    }

    private ProcessOutput format(String document) throws ExecutionException {
      String binary =
          Binary.choose(AdocfmtIdeSettings.getInstance().getBinaryPath(), pluginDirectory);
      GeneralCommandLine command =
          new GeneralCommandLine(binary).withCharset(StandardCharsets.UTF_8);
      CapturingProcessHandler handler = new CapturingProcessHandler(command);
      process = handler;
      // adocfmt reads all of stdin before it writes anything, so writing first
      // and reading afterwards cannot block.
      try (OutputStream stdin = handler.getProcess().getOutputStream()) {
        stdin.write(document.getBytes(StandardCharsets.UTF_8));
      } catch (IOException e) {
        // adocfmt quit early, and its exit code and stderr tell why.
      }
      // Killed after the platform stops waiting, so a hanging binary does not
      // outlive the request.
      return handler.runProcess((int) getTimeout().toMillis());
    }

    @Override
    public boolean cancel() {
      ProcessHandler running = process;
      if (running != null) {
        running.destroyProcess();
      }
      return true;
    }
  }

  private static List<String> crashReport(Outcome.Crashed crashed) {
    List<String> lines = new ArrayList<>();
    lines.add("Exit code " + crashed.exitCode());
    lines.addAll(crashed.stderr());
    return lines;
  }

  // Notification content is HTML.
  private static String html(List<String> lines) {
    return String.join("<br>", lines.stream().map(StringUtil::escapeXmlEntities).toList());
  }
}
