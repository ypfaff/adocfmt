import * as fs from "node:fs";
import * as path from "node:path";
import * as vscode from "vscode";
import { bundledBinary, format } from "./adocfmt";
import { Outcome } from "./outcome";

export function activate(context: vscode.ExtensionContext): void {
  // No range formatting provider, because VS Code would call it on paste.
  const provider: vscode.DocumentFormattingEditProvider = {
    provideDocumentFormattingEdits: (document, _options, token) =>
      formatDocument(document, context.extensionPath, token),
  };
  context.subscriptions.push(
    vscode.languages.registerDocumentFormattingEditProvider(
      { language: "asciidoc" },
      provider,
    ),
  );
}

async function formatDocument(
  document: vscode.TextDocument,
  extensionDirectory: string,
  token: vscode.CancellationToken,
): Promise<vscode.TextEdit[]> {
  if (!isAsciiDoc(document)) {
    return [];
  }
  const text = document.getText();
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

function wholeText(document: vscode.TextDocument, text: string): vscode.Range {
  return new vscode.Range(
    document.positionAt(0),
    document.positionAt(text.length),
  );
}

function showError(title: string, details: string): void {
  void vscode.window.showErrorMessage(`${title}: ${details}`);
}
