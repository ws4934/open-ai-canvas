import { afterEach, describe, expect, test } from "bun:test";
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const repository = fileURLToPath(new URL("../../", import.meta.url));
const temporaryDirectories = [];
afterEach(() => {
    for (const directory of temporaryDirectories.splice(0)) rmSync(directory, { recursive: true, force: true });
});

function fixture(overrides = {}) {
    const root = mkdtempSync(path.join(os.tmpdir(), "canvas-release-"));
    temporaryDirectories.push(root);
    const bin = path.join(root, "bin");
    mkdirSync(bin);
    mkdirSync(path.join(root, ".github"));
    symlinkSync(path.join(repository, ".github/scripts"), path.join(root, ".github/scripts"));
    mkdirSync(path.join(root, "dist/digests"), { recursive: true });
    for (const letter of ["a", "b"]) writeFileSync(path.join(root, "dist/digests", letter.repeat(64)), "");
    writeFileSync(path.join(root, "VERSION"), "v1.2.3\n");
    writeFileSync(path.join(root, "CHANGELOG.md"), "## v1.2.3\n\nRelease notes.\n");
    writeFileSync(path.join(root, "docker-compose.deploy.yml"), "services: {}\n");
    writeFileSync(path.join(root, "yingce-backend"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
    for (const arch of ["amd64", "arm64"]) {
        expect(spawnSync("tar", ["-czf", "dist/open-ai-canvas_v1.2.3_linux_" + arch + ".tar.gz", "yingce-backend"], { cwd: root }).status).toBe(0);
    }
    const git = (...args) => spawnSync("git", args, { cwd: root, encoding: "utf8" });
    expect(git("init", "-q").status).toBe(0);
    git("add", "VERSION", "CHANGELOG.md");
    expect(git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture").status).toBe(0);
    const sha = git("rev-parse", "HEAD").stdout.trim();
    const log = path.join(root, "commands");
    writeFileSync(log, "");
    writeFileSync(path.join(bin, "docker"), '#!/bin/sh\nprintf "docker %s\\n" "$*" >> "$MOCK_LOG"\n', { mode: 0o755 });
    writeFileSync(
        path.join(bin, "gh"),
        [
            "#!/usr/bin/env python3",
            "import json,os,sys",
            "args=sys.argv[1:]",
            'with open(os.environ["MOCK_LOG"],"a") as f: f.write("gh "+" ".join(args)+"\\n")',
            'if args[:1]==["api"] and "/releases/tags/" in args[1]:',
            '    mode=os.environ.get("MOCK_RELEASE","missing")',
            '    if mode=="missing": print("gh: Not Found (HTTP 404)",file=sys.stderr); sys.exit(1)',
            '    if mode=="error": print("gh: Forbidden (HTTP 403)",file=sys.stderr); sys.exit(1)',
            '    print(json.dumps({"draft":mode=="draft","target_commitish":os.environ["GITHUB_SHA"]}))',
            'elif args[:2]==["release","view"]:',
            '    names=["open-ai-canvas_v1.2.3_linux_amd64.tar.gz","open-ai-canvas_v1.2.3_linux_arm64.tar.gz","SHA256SUMS","docker-compose.deploy.yml"]',
            '    if os.environ.get("MOCK_MISSING_ASSET"): names.pop()',
            '    print("\\n".join(names))',
            "",
        ].join("\n"),
        { mode: 0o755 },
    );
    const output = path.join(root, "output");
    writeFileSync(output, "");
    const env = {
        ...process.env,
        PATH: bin + ":" + process.env.PATH,
        MOCK_LOG: log,
        GITHUB_OUTPUT: output,
        GITHUB_SHA: sha,
        GITHUB_EVENT_NAME: "push",
        GITHUB_REF: "refs/heads/main",
        GITHUB_REPOSITORY: "fixture/repo",
        REPOSITORY_OWNER: "fixture",
        IMAGE: "ghcr.io/fixture/open-ai-canvas",
        RELEASE_VERSION: "v1.2.3",
        IS_RELEASE: "true",
        EXPECTED_DIGESTS: "2",
        DIGEST_DIR: path.join(root, "dist/digests"),
        ...overrides,
    };
    return {
        root,
        sha,
        git,
        run: (script) => spawnSync("bash", [path.join(repository, ".github/scripts", script)], { cwd: root, env, encoding: "utf8" }),
        log: () => readFileSync(log, "utf8"),
        output: () => readFileSync(output, "utf8"),
    };
}

describe("release preparation", () => {
    test("main never creates a public release or a version tag", () => {
        const repo = fixture({ IS_RELEASE: "false" });
        expect(repo.run("prepare-release.sh").status).toBe(0);
        expect(repo.output()).toContain("release=false");
        expect(repo.run("publish-release.sh").status).toBe(0);
        expect(repo.log()).not.toContain("gh release");
        expect(repo.log()).not.toContain(":1.2.3");
        expect(repo.log()).not.toContain(":latest");
    });
    test("manual publication pins the selected SHA and continues in this workflow", () => {
        const repo = fixture({ GITHUB_EVENT_NAME: "workflow_dispatch", REQUESTED_VERSION: "v1.2.3" });
        expect(repo.run("prepare-release.sh").status).toBe(0);
        expect(repo.log()).toContain("-f ref=refs/tags/v1.2.3 -f sha=" + repo.sha);
        expect(repo.output()).toContain("release=true");
    });
    test("manual version must agree with tracked VERSION", () => {
        const repo = fixture({ GITHUB_EVENT_NAME: "workflow_dispatch", REQUESTED_VERSION: "v1.2.4" });
        expect(repo.run("prepare-release.sh").status).not.toBe(0);
        expect(repo.log()).toBe("");
    });
    test("an existing tag at another commit cannot be moved", () => {
        const repo = fixture({ GITHUB_EVENT_NAME: "workflow_dispatch", REQUESTED_VERSION: "v1.2.3" });
        writeFileSync(path.join(repo.root, "another"), "other");
        repo.git("add", "another");
        repo.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "another");
        repo.git("tag", "v1.2.3");
        expect(repo.run("prepare-release.sh").status).not.toBe(0);
        expect(repo.log()).toBe("");
    });
});

describe("public release boundary", () => {
    test("uploads and verifies all assets in draft before publishing", () => {
        const repo = fixture();
        expect(repo.run("publish-release.sh").status).toBe(0);
        const log = repo.log();
        expect(log).toContain("--draft");
        expect(log.indexOf("release upload")).toBeLessThan(log.indexOf("release edit"));
        expect(log.indexOf("release view")).toBeLessThan(log.indexOf("imagetools create"));
        expect(log).toContain("--draft=false");
        expect(log).toContain("ghcr.io/fixture/open-ai-canvas:1.2.3");
    });
    test("resumes a draft at the same SHA without recreating the release", () => {
        const repo = fixture({ MOCK_RELEASE: "draft" });
        expect(repo.run("publish-release.sh").status).toBe(0);
        expect(repo.log()).not.toContain("release create");
        expect(repo.log()).toContain("release edit");
    });
    for (const mode of ["published", "error"]) {
        test("never uploads when the release is " + mode, () => {
            const repo = fixture({ MOCK_RELEASE: mode });
            expect(repo.run("publish-release.sh").status).not.toBe(0);
            expect(repo.log()).not.toContain("release upload");
            expect(repo.log()).not.toContain("imagetools create");
        });
    }
    test("a missing uploaded asset leaves the release in draft", () => {
        const repo = fixture({ MOCK_MISSING_ASSET: "1" });
        expect(repo.run("publish-release.sh").status).not.toBe(0);
        expect(repo.log()).not.toContain("release edit");
    });
    test("a missing architecture prevents manifest promotion", () => {
        const repo = fixture();
        rmSync(path.join(repo.root, "dist/digests", "b".repeat(64)));
        expect(repo.run("promote-image.sh").status).not.toBe(0);
        expect(repo.log()).toBe("");
    });
});

describe("persistent application entrypoint", () => {
    function runEntry(root, relativeBinary = "system/bin/yingce-backend", uid = "1000") {
        const bin = path.join(root, "bin");
        mkdirSync(bin, { recursive: true });
        writeFileSync(path.join(bin, "id"), "#!/bin/sh\necho " + uid + "\n", { mode: 0o755 });
        writeFileSync(path.join(bin, "chown"), '#!/bin/sh\nshift\nfor path in "$@"; do test -e "$path" || exit 1; done\n', { mode: 0o755 });
        writeFileSync(path.join(bin, "su-exec"), '#!/bin/sh\nshift\nexec "$@"\n', { mode: 0o755 });
        return spawnSync("sh", [path.join(repository, "docker/canvas-entrypoint.sh")], {
            env: { ...process.env, PATH: bin + ":" + process.env.PATH, CANVAS_BACKEND_DATA_DIR: root, CANVAS_MANAGED_BINARY_PATH: path.join(root, relativeBinary), CANVAS_SEED_BINARY: root + "/seed" },
            encoding: "utf8",
        });
    }
    test("initializes once and never downgrades an updated program on recreation", () => {
        const repo = fixture();
        writeFileSync(path.join(repo.root, "seed"), "#!/bin/sh\necho seed\n");
        expect(runEntry(repo.root).stdout).toBe("seed\n");
        writeFileSync(path.join(repo.root, "system/bin/yingce-backend"), "#!/bin/sh\necho updated\n", { mode: 0o755 });
        expect(runEntry(repo.root).stdout).toBe("updated\n");
    });
    test("interrupted replacement restores a schema-compatible backup, not the seed", () => {
        const repo = fixture();
        mkdirSync(path.join(repo.root, "system/bin"), { recursive: true });
        writeFileSync(path.join(repo.root, "seed"), '#!/bin/sh\n[ "$1" = --restore-program ] || exit 1\n"$CANVAS_MANAGED_BINARY_PATH.backup" --check-schema || exit 1\nmv "$CANVAS_MANAGED_BINARY_PATH.backup" "$CANVAS_MANAGED_BINARY_PATH"\n', {
            mode: 0o755,
        });
        writeFileSync(path.join(repo.root, "system/bin/yingce-backend.backup"), '#!/bin/sh\nif [ "$1" = --check-schema ]; then exit 0; fi\necho backup\n', { mode: 0o755 });
        expect(runEntry(repo.root).stdout).toBe("backup\n");
        expect(existsSync(path.join(repo.root, "system/bin/yingce-backend.backup"))).toBe(false);
    });
    test("root initialization creates system state separately from a custom managed path", () => {
        const repo = fixture();
        writeFileSync(path.join(repo.root, "seed"), "#!/bin/sh\necho custom\n");
        const result = runEntry(repo.root, "custom/yingce-backend", "0");
        expect(result.status).toBe(0);
        expect(result.stdout).toBe("custom\n");
        expect(existsSync(path.join(repo.root, "system"))).toBe(true);
        expect(existsSync(path.join(repo.root, "custom/yingce-backend"))).toBe(true);
    });
    test("incompatible backup and lost programs preserve the recovery state", () => {
        const repo = fixture();
        mkdirSync(path.join(repo.root, "system/bin"), { recursive: true });
        writeFileSync(path.join(repo.root, "seed"), '#!/bin/sh\n"$CANVAS_MANAGED_BINARY_PATH.backup" --check-schema\n', { mode: 0o755 });
        writeFileSync(path.join(repo.root, "system/bin/yingce-backend.backup"), "#!/bin/sh\nexit 1\n", { mode: 0o755 });
        expect(runEntry(repo.root).status).not.toBe(0);
        expect(existsSync(path.join(repo.root, "system/bin/yingce-backend.backup"))).toBe(true);
        rmSync(path.join(repo.root, "system/bin/yingce-backend.backup"));
        writeFileSync(path.join(repo.root, "system/update.json"), "{}");
        expect(runEntry(repo.root).status).not.toBe(0);
        expect(existsSync(path.join(repo.root, "system/bin/yingce-backend"))).toBe(false);
    });
});
