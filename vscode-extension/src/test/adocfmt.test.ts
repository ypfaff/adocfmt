import * as assert from "node:assert/strict";
import * as fs from "node:fs";
import * as path from "node:path";
import { before, test } from "node:test";
import { bundledBinary, format } from "../adocfmt";

// The extension directory, seen from out/test.
const binary = bundledBinary(path.join(__dirname, "..", ".."));

before(() => {
  assert.ok(
    fs.existsSync(binary),
    `${binary} is missing: run npm run build-adocfmt`,
  );
});

test("formats the document", async () => {
  assert.deepEqual(await format(binary, "## Title\n"), {
    kind: "formatted",
    text: "== Title\n",
  });
});

test("reports the findings of a refusal", async () => {
  assert.deepEqual(await format(binary, "ifdef::[]\nText.\nendif::[]\n"), {
    kind: "refused",
    findings: [
      { line: 1, message: "malformed preprocessor directive" },
      { line: 3, message: "endif closes no conditional region" },
    ],
  });
});

test("rejects when the binary is missing", async () => {
  const missing = path.join(__dirname, "no-such-adocfmt");
  await assert.rejects(format(missing, "## Title\n"), { code: "ENOENT" });
});

test("rejects with an AbortError once the signal is aborted", async () => {
  await assert.rejects(format(binary, "## Title\n", AbortSignal.abort()), {
    name: "AbortError",
  });
});

test("formats a document of several MB with multi-byte characters", async () => {
  const body = "Grüße aus Köln, 東京 und Αθήνα.\n".repeat(150_000);
  const outcome = await format(binary, `## Größe\n\n${body}`);
  assert.ok(outcome.kind === "formatted");
  assert.ok(outcome.text === `== Größe\n\n${body}`);
});
