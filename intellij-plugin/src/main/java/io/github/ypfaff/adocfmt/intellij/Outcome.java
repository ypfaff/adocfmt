package io.github.ypfaff.adocfmt.intellij;

import java.util.ArrayList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** What a run of adocfmt on stdin came to. */
sealed interface Outcome {
  record Formatted(String text) implements Outcome {}

  record Refused(List<String> findings) implements Outcome {}

  record Crashed(int exitCode, List<String> stderr) implements Outcome {}

  // <stdin>:<line>: <message>, or <stdin>: <message> for a finding on no line.
  Pattern FINDING = Pattern.compile("<stdin>(?::(\\d+))?: (.*)");

  static Outcome of(int exitCode, String stdout, List<String> stderr) {
    if (exitCode == 0) {
      return new Formatted(stdout);
    }
    List<String> findings = exitCode == 2 ? findings(stderr) : List.of();
    return findings.isEmpty() ? new Crashed(exitCode, stderr) : new Refused(findings);
  }

  // A Go panic or a usage error exits with 2 as well, so a single line that is
  // no finding makes a crash of it.
  private static List<String> findings(List<String> stderr) {
    List<String> findings = new ArrayList<>();
    for (String line : stderr) {
      Matcher finding = FINDING.matcher(line);
      if (!finding.matches()) {
        return List.of();
      }
      String number = finding.group(1);
      String message = finding.group(2);
      findings.add(number == null ? message : "Line " + number + ": " + message);
    }
    return findings;
  }
}
