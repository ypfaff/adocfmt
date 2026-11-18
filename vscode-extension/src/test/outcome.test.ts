import * as assert from "node:assert/strict";
import { test } from "node:test";
import { outcomeOf } from "../outcome";

test("exit 0 is the formatted text", () => {
  assert.deepEqual(outcomeOf(0, "== Title\n", ""), {
    kind: "formatted",
    text: "== Title\n",
  });
});

test("exit 2 with findings is a refusal", () => {
  const stderr =
    "<stdin>:1: malformed preprocessor directive\n" +
    "<stdin>:3: endif closes no conditional region\n";
  assert.deepEqual(outcomeOf(2, "", stderr), {
    kind: "refused",
    findings: [
      { line: 1, message: "malformed preprocessor directive" },
      { line: 3, message: "endif closes no conditional region" },
    ],
  });
});

test("a finding without a line is on the whole document", () => {
  assert.deepEqual(outcomeOf(2, "", "<stdin>: source is not valid UTF-8\n"), {
    kind: "refused",
    findings: [{ line: undefined, message: "source is not valid UTF-8" }],
  });
});

test("a panic among the findings is a crash", () => {
  const stderr =
    "<stdin>:1: malformed preprocessor directive\n" +
    "panic: runtime error: index out of range\n";
  assert.deepEqual(outcomeOf(2, "", stderr), {
    kind: "crashed",
    exitCode: 2,
    stderr,
  });
});

test("a usage error is a crash", () => {
  const stderr = "adocfmt: unknown flag: --nope\n";
  assert.deepEqual(outcomeOf(2, "", stderr), {
    kind: "crashed",
    exitCode: 2,
    stderr,
  });
});

test("exit 1 is a crash, even with findings", () => {
  const stderr = "<stdin>:1: malformed preprocessor directive\n";
  assert.deepEqual(outcomeOf(1, "", stderr), {
    kind: "crashed",
    exitCode: 1,
    stderr,
  });
});

test("exit 2 without stderr is a crash", () => {
  assert.deepEqual(outcomeOf(2, "", ""), {
    kind: "crashed",
    exitCode: 2,
    stderr: "",
  });
});
