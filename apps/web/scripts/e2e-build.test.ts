import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { buildHash, ensureBuild, hashFile } from "./e2e-build.mjs";

let root: string;

// The build reads only NEXT_PUBLIC_* values, so tests pass partial envs.
function env(vars: Record<string, string>): NodeJS.ProcessEnv {
  return vars as NodeJS.ProcessEnv;
}

function write(relative: string, content: string) {
  const file = path.join(root, relative);
  mkdirSync(path.dirname(file), { recursive: true });
  writeFileSync(file, content);
}

// A minimal checkout: the inputs `next build` reads, plus a standalone server.
function seed() {
  write("src/app/page.tsx", "export default function Page() {}");
  write("messages/en.json", "{}");
  write("package.json", "{}");
  write("pnpm-lock.yaml", "lock");
  write("next.config.ts", "export default {};");
  write(".next/standalone/server.js", "server");
}

beforeEach(() => {
  root = mkdtempSync(path.join(tmpdir(), "e2e-build-"));
  seed();
});

afterEach(() => {
  rmSync(root, { recursive: true, force: true });
});

describe("buildHash", () => {
  it("is stable when nothing changed", () => {
    expect(buildHash(root, env({}))).toBe(buildHash(root, env({})));
  });

  it("changes when a source file changes", () => {
    const before = buildHash(root, env({}));
    write("src/app/page.tsx", "export default function Page() { return null; }");
    expect(buildHash(root, env({}))).not.toBe(before);
  });

  it("changes when a new source file is added", () => {
    const before = buildHash(root, env({}));
    write("src/app/new/page.tsx", "export {};");
    expect(buildHash(root, env({}))).not.toBe(before);
  });

  it("changes when the lockfile or a message file changes", () => {
    const before = buildHash(root, env({}));
    write("pnpm-lock.yaml", "lock v2");
    const afterLock = buildHash(root, env({}));
    expect(afterLock).not.toBe(before);
    write("messages/en.json", '{"a":1}');
    expect(buildHash(root, env({}))).not.toBe(afterLock);
  });

  it("ignores files outside the build inputs", () => {
    const before = buildHash(root, env({}));
    write("e2e/auth.spec.ts", "test");
    write(".next/cache/x", "cache");
    write(hashFile, "stale");
    expect(buildHash(root, env({}))).toBe(before);
  });

  it("changes with NEXT_PUBLIC_ values but not other env vars", () => {
    const before = buildHash(root, env({ NEXT_PUBLIC_SITE: "a" }));
    expect(buildHash(root, env({ NEXT_PUBLIC_SITE: "b" }))).not.toBe(before);
    expect(buildHash(root, env({ NEXT_PUBLIC_SITE: "a", API_URL: "x" }))).toBe(before);
  });
});

describe("ensureBuild", () => {
  it("builds when there is no stored hash, then stores it", () => {
    const build = vi.fn(() => 0);
    const result = ensureBuild({ root, env: env({}), build, log: () => {} });
    expect(result).toEqual({ built: true, status: 0 });
    expect(build).toHaveBeenCalledTimes(1);
    expect(readFileSync(path.join(root, hashFile), "utf8").trim()).toBe(buildHash(root, env({})));
  });

  it("skips the build when the inputs match the stored hash", () => {
    ensureBuild({ root, env: env({}), build: () => 0, log: () => {} });
    const build = vi.fn(() => 0);
    const result = ensureBuild({ root, env: env({}), build, log: () => {} });
    expect(result).toEqual({ built: false });
    expect(build).not.toHaveBeenCalled();
  });

  it("rebuilds after a source change", () => {
    ensureBuild({ root, env: env({}), build: () => 0, log: () => {} });
    write("src/app/page.tsx", "changed");
    const build = vi.fn(() => 0);
    expect(ensureBuild({ root, env: env({}), build, log: () => {} })).toEqual({ built: true, status: 0 });
    expect(build).toHaveBeenCalledTimes(1);
  });

  it("rebuilds when the standalone server is missing even if the hash matches", () => {
    ensureBuild({ root, env: env({}), build: () => 0, log: () => {} });
    rmSync(path.join(root, ".next/standalone/server.js"));
    const build = vi.fn(() => 0);
    ensureBuild({ root, env: env({}), build, log: () => {} });
    expect(build).toHaveBeenCalledTimes(1);
  });

  it("does not store the hash when the build fails, so the next run rebuilds", () => {
    const failed = ensureBuild({ root, env: env({}), build: () => 1, log: () => {} });
    expect(failed).toEqual({ built: true, status: 1 });
    const build = vi.fn(() => 0);
    ensureBuild({ root, env: env({}), build, log: () => {} });
    expect(build).toHaveBeenCalledTimes(1);
  });
});
