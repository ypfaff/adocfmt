/** What a run of adocfmt on stdin came to. */
export type Outcome =
  | { kind: "formatted"; text: string }
  | { kind: "refused"; findings: Finding[] }
  | { kind: "crashed"; exitCode: number; stderr: string };

/** One reason adocfmt refused the document. */
export interface Finding {
  /** The line the finding is on, from 1, or undefined for the whole document. */
  line: number | undefined;
  message: string;
}

// <stdin>:<line>: <message>, or <stdin>: <message> for a finding on no line.
const FINDING = /^<stdin>(?::(\d+))?: (.*)$/;

export function outcomeOf(
  exitCode: number,
  stdout: string,
  stderr: string,
): Outcome {
  if (exitCode === 0) {
    return { kind: "formatted", text: stdout };
  }
  const findings = exitCode === 2 ? parseFindings(stderr) : [];
  return findings.length === 0
    ? { kind: "crashed", exitCode, stderr }
    : { kind: "refused", findings };
}

// A Go panic or a usage error exits with 2 as well, so a single line that is
// no finding makes a crash of it.
export function parseFindings(stderr: string): Finding[] {
  const findings: Finding[] = [];
  for (const line of stderr.split("\n").filter((line) => line !== "")) {
    const match = FINDING.exec(line);
    if (match === null) {
      return [];
    }
    const [, number, message] = match;
    findings.push({
      line: number === undefined ? undefined : Number(number),
      message,
    });
  }
  return findings;
}
