import { spawn } from "node:child_process";
import * as path from "node:path";
import { Outcome, outcomeOf } from "./outcome";

/** The binary that the extension carries in its bin directory. */
export function bundledBinary(extensionDirectory: string): string {
  const fileName = process.platform === "win32" ? "adocfmt.exe" : "adocfmt";
  return path.join(extensionDirectory, "bin", fileName);
}

/**
 * Runs the binary on the text through stdin. Rejects if the binary cannot
 * run, or with an AbortError once the signal kills it.
 */
export function format(
  binary: string,
  text: string,
  signal?: AbortSignal,
): Promise<Outcome> {
  return new Promise((resolve, reject) => {
    // Not execFile, whose 1 MiB maxBuffer would kill a big document.
    const child = spawn(binary, [], { windowsHide: true, signal });
    const stdout: Buffer[] = [];
    const stderr: Buffer[] = [];
    child.stdout.on("data", (chunk: Buffer) => stdout.push(chunk));
    child.stderr.on("data", (chunk: Buffer) => stderr.push(chunk));
    child.on("error", reject);
    child.on("close", (exitCode, killedBy) => {
      if (exitCode === null) {
        reject(new Error(`adocfmt was killed by ${killedBy}`));
        return;
      }
      // Decoded once, because a chunk can end inside a multi-byte character.
      resolve(outcomeOf(exitCode, decode(stdout), decode(stderr)));
    });
    // A binary that quits early breaks the pipe, and its exit code and stderr
    // tell why.
    child.stdin.on("error", () => {});
    child.stdin.end(text);
  });
}

function decode(chunks: Buffer[]): string {
  return Buffer.concat(chunks).toString("utf8");
}
