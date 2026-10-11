package io.github.ypfaff.adocfmt.intellij;

import static org.junit.Assert.assertEquals;

import java.util.List;
import org.junit.Test;

public class OutcomeTest {
  @Test
  public void exitZeroIsTheFormattedText() {
    assertEquals(new Outcome.Formatted("= Title\n"), Outcome.of(0, "= Title\n", List.of()));
  }

  @Test
  public void exitTwoWithFindingsIsARefusal() {
    List<String> stderr =
        List.of("<stdin>:3: block has no closing delimiter", "<stdin>: mixed line endings");
    assertEquals(
        new Outcome.Refused(
            List.of("Line 3: block has no closing delimiter", "mixed line endings")),
        Outcome.of(2, "", stderr));
  }

  @Test
  public void exitTwoWithAPanicIsACrash() {
    List<String> stderr = List.of("panic: runtime error: index out of range", "", "goroutine 1:");
    assertEquals(new Outcome.Crashed(2, stderr), Outcome.of(2, "", stderr));
  }

  @Test
  public void exitTwoWithAFindingAndAPanicIsACrash() {
    List<String> stderr = List.of("<stdin>:1: conditional region has no endif", "panic: boom");
    assertEquals(new Outcome.Crashed(2, stderr), Outcome.of(2, "", stderr));
  }

  @Test
  public void exitTwoWithAUsageErrorIsACrash() {
    List<String> stderr = List.of("adocfmt: flag provided but not defined: -x");
    assertEquals(new Outcome.Crashed(2, stderr), Outcome.of(2, "", stderr));
  }

  @Test
  public void exitTwoWithoutFindingsIsACrash() {
    assertEquals(new Outcome.Crashed(2, List.of()), Outcome.of(2, "", List.of()));
  }

  @Test
  public void anotherExitCodeIsACrash() {
    List<String> stderr = List.of("<stdin>:3: block has no closing delimiter");
    assertEquals(new Outcome.Crashed(1, stderr), Outcome.of(1, "", stderr));
  }
}
