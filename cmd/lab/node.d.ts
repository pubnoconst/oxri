// Types for the few Node APIs the lab uses (lab.mjs). Only `tsc` reads this
// file (cmd/lab/tsconfig.json). The project carries no @types package: as
// assets/types/globals.d.ts does for the site's scripts, this declares the
// little the lab needs, so a new use of Node shows up here first.

/** The ends of a pipe to a child process, as the lab reads and writes them. */
interface LabNodeStream {
  write(chunk: string): boolean;
  setEncoding(encoding: "utf8"): void;
  on(event: "data", listener: (chunk: string) => void): void;
  on(event: "error", listener: (error: Error) => void): void;
  end(): void;
}

declare const process: {
  argv: string[];
  env: Record<string, string | undefined>;
  platform: string;
  exitCode?: number;
  exit(code: number): never;
  stdout: { write(chunk: string): boolean };
  stderr: { write(chunk: string): boolean };
};

declare module "node:child_process" {
  export interface ChildProcess {
    pid: number | undefined;
    /** null while the process runs. */
    exitCode: number | null;
    /** stdin, stdout, stderr, then what the caller asked for. */
    stdio: readonly (LabNodeStream | null)[];
    kill(signal?: string): boolean;
    on(event: "exit", listener: (code: number | null, signal: string | null) => void): void;
  }
  /** Starts a command, with or without arguments (through the system shell when shell is set). */
  export function spawn(command: string, options?: { shell?: boolean; stdio?: "ignore" | readonly string[] }): ChildProcess;
  export function spawn(command: string, args: readonly string[], options?: { shell?: boolean; stdio?: "ignore" | readonly string[] }): ChildProcess;
}

declare module "node:fs" {
  export function existsSync(path: string): boolean;
  export function mkdtempSync(prefix: string): string;
  export function readFileSync(path: string, encoding: "utf8"): string;
  export function writeFileSync(path: string, data: string): void;
  export function rmSync(path: string, options: { recursive: boolean; force: boolean; maxRetries?: number; retryDelay?: number }): void;
}

declare module "node:os" {
  export function tmpdir(): string;
}

declare module "node:path" {
  export function dirname(path: string): string;
  export function join(...parts: string[]): string;
}

declare module "node:url" {
  export function fileURLToPath(url: string): string;
}
