import * as fs from "node:fs";
import * as path from "node:path";
import * as vscode from "vscode";
import { bundledBinary, format } from "./adocfmt";
import type { Finding, Outcome } from "./outcome";

export function activate(context: vscode.ExtensionContext): void {
  const findings = vscode.languages.createDiagnosticCollection("adocfmt");
  // No range formatting provider, because VS Code would call it on paste.
  const provider: vscode.DocumentFormattingEditProvider = {
    provideDocumentFormattingEdits: (document, _options, token) =>
      formatDocument(document, context.extensionPath, findings, token),
  };
  context.subscriptions.push(
    findings,
    vscode.languages.registerDocumentFormattingEditProvider(
      { language: "asciidoc" },
      provider,
    ),
    vscode.workspace.onDidCloseTextDocument((document) =>
      findings.delete(document.uri),
    ),
  );
}

async function formatDocument(
  document: vscode.TextDocument,
  extensionDirectory: string,
  findings: vscode.DiagnosticCollection,
  token: vscode.CancellationToken,
): Promise<vscode.TextEdit[]> {
  if (!isAsciiDoc(document)) {
    return [];
  }
  const text = document.getText();
  const version = document.version;
  const cancel = new AbortController();
  const cancellation = token.onCancellationRequested(() => cancel.abort());
  let outcome: Outcome;
  try {
    outcome = await format(
      chooseBinary(extensionDirectory),
      text,
      cancel.signal,
    );
  } catch (error) {
    if (!token.isCancellationRequested) {
      const message = error instanceof Error ? error.message : String(error);
      showError("adocfmt could not run", message);
    }
    return [];
  } finally {
    cancellation.dispose();
  }
  // A caller without a cancellation token, such as another extension running
  // vscode.executeFormatDocumentProvider, lets the document change meanwhile.
  // The result then no longer fits it.
  if (document.version !== version) {
    return [];
  }
  // Each finished run replaces the findings of the document.
  findings.set(
    document.uri,
    outcome.kind === "refused"
      ? outcome.findings.map((finding) => diagnostic(document, finding))
      : undefined,
  );
  switch (outcome.kind) {
    case "formatted":
      return outcome.text === text
        ? []
        : [vscode.TextEdit.replace(wholeText(document, text), outcome.text)];
    case "refused":
      return [];
    case "crashed":
      showError(
        `adocfmt crashed on ${path.basename(document.fileName)}`,
        `Exit code ${outcome.exitCode}. ${outcome.stderr}`.trim(),
      );
      return [];
  }
}

// The extensions the CLI collects in a directory, in any case. The AsciiDoc
// extension also maps .asc, which is often PGP armor.
function isAsciiDoc(document: vscode.TextDocument): boolean {
  return document.isUntitled || /\.(adoc|asciidoc)$/i.test(document.fileName);
}

function chooseBinary(extensionDirectory: string): string {
  const customPath = vscode.workspace
    .getConfiguration("adocfmt")
    .get<string>("path", "")
    .trim();
  if (customPath !== "") {
    return customPath;
  }
  const binary = bundledBinary(extensionDirectory);
  if (!fs.existsSync(binary)) {
    throw new Error(
      `The bundled adocfmt binary is missing: ${binary}. Set adocfmt.path to an adocfmt binary.`,
    );
  }
  return binary;
}

// adocfmt counts lines from 1, VS Code from 0. A finding on no line concerns
// the whole file, so it goes on the first line.
function diagnostic(
  document: vscode.TextDocument,
  finding: Finding,
): vscode.Diagnostic {
  const line = document.lineAt((finding.line ?? 1) - 1);
  const result = new vscode.Diagnostic(
    line.range,
    finding.message,
    vscode.DiagnosticSeverity.Error,
  );
  result.source = "adocfmt";
  return result;
}

function wholeText(document: vscode.TextDocument, text: string): vscode.Range {
  return new vscode.Range(
    document.positionAt(0),
    document.positionAt(text.length),
  );
}

function showError(title: string, details: string): void {
  void vscode.window.showErrorMessage(`${title}: ${details}`);
}
