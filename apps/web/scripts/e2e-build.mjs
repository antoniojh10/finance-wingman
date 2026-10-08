// Builds the standalone web app for e2e only when its inputs changed. The
// hash of the build inputs is stored in .e2e-build-hash after a successful
// build; a matching hash plus an existing standalone server skips `pnpm build`.
//
// Usage (from the Playwright webServer command): node scripts/e2e-build.mjs
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

// Everything `next build` reads. Paths missing from the checkout are skipped.
export const buildInputs = [
  "src",
  "public",
  "messages",
  "next.config.ts",
  "package.json",
  "pnpm-lock.yaml",
  "tsconfig.json",
  "postcss.config.mjs",
];

export const hashFile = ".e2e-build-hash";
const standaloneServer = path.join(".next", "standalone", "server.js");

function listFiles(absolute, relative, out) {
  const stat = statSync(absolute);
  if (stat.isDirectory()) {
    for (const entry of readdirSync(absolute).sort()) {
      listFiles(path.join(absolute, entry), path.posix.join(relative, entry), out);
    }
  } else {
    out.push({ relative, absolute });
  }
  return out;
}

// Hash of the build inputs under `root` plus the NEXT_PUBLIC_* values, which
// Next inlines into the client bundle.
export function buildHash(root, env = process.env) {
  const hash = createHash("sha256");
  const files = [];
  for (const input of buildInputs) {
    const absolute = path.join(root, input);
    if (existsSync(absolute)) listFiles(absolute, input, files);
  }
  for (const { relative, absolute } of files.sort((a, b) => (a.relative < b.relative ? -1 : 1))) {
    hash.update(`file ${relative}\0`);
    hash.update(readFileSync(absolute));
    hash.update("\0");
  }
  const publicEnv = Object.keys(env)
    .filter((key) => key.startsWith("NEXT_PUBLIC_"))
    .sort();
  for (const key of publicEnv) hash.update(`env ${key}=${env[key]}\0`);
  return hash.digest("hex");
}

// True when the stored hash matches and the standalone server exists.
export function canSkipBuild({ root, hash }) {
  const hashPath = path.join(root, hashFile);
  if (!existsSync(path.join(root, standaloneServer)) || !existsSync(hashPath)) return false;
  return readFileSync(hashPath, "utf8").trim() === hash;
}

export function ensureBuild({ root, env = process.env, build = defaultBuild, log = console.log }) {
  const hash = buildHash(root, env);
  if (canSkipBuild({ root, hash })) {
    log("e2e build: inputs unchanged, skipping `pnpm build`");
    return { built: false };
  }
  log("e2e build: inputs changed, running `pnpm build`");
  const status = build(root);
  if (status !== 0) return { built: true, status };
  // Stored only after success, so a failed build is never reused.
  writeFileSync(path.join(root, hashFile), `${hash}\n`);
  return { built: true, status: 0 };
}

function defaultBuild(root) {
  const result = spawnSync("pnpm", ["build"], { cwd: root, stdio: "inherit", env: process.env });
  return result.status ?? 1;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const root = path.resolve(import.meta.dirname, "..");
  const { status } = ensureBuild({ root });
  process.exit(status ?? 0);
}
