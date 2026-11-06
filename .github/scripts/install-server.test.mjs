import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, realpathSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));
const installer = path.join(root, "scripts/install-server-image.sh");
const sourceInstaller = path.join(root, "scripts/install-server.sh");
const compose = readFileSync(path.join(root, "docker-compose.deploy.yml"), "utf8");
const version = "v1.7.3.6";

function fixture(t, { installed = false, failure = "", invalid = "" } = {}) {
    const directory = realpathSync(mkdtempSync(path.join(os.tmpdir(), "canvas-install-test-")));
    const install = path.join(directory, "application");
    const bin = path.join(directory, "bin");
    const home = path.join(directory, "home");
    const log = path.join(directory, "commands.jsonl");
    mkdirSync(bin);
    mkdirSync(home);
    if (installed) {
        mkdirSync(install);
        writeFileSync(path.join(install, ".env"), "POSTGRES_PASSWORD=old-password\nCANVAS_DATA_PATH=./data\nCANVAS_POSTGRES_DATA_PATH=./postgres-data\nCANVAS_REDIS_DATA_PATH=./redis-data\nCANVAS_HTTP_BIND=0.0.0.0\nCANVAS_HTTP_PORT=4567\n");
        writeFileSync(path.join(install, "docker-compose.deploy.yml"), compose);
        for (const name of ["data", "postgres-data", "redis-data"]) {
            mkdirSync(path.join(install, name));
            writeFileSync(path.join(install, name, "old-data"), "must not enter new deployment");
        }
    }
    const mock = `#!${process.execPath}
const fs = require("node:fs"), path = require("node:path");
const command = path.basename(process.argv[1]), args = process.argv.slice(2);
fs.appendFileSync(process.env.TEST_LOG, JSON.stringify({ command, args }) + "\\n");
const fail = (name) => { if (process.env.TEST_FAILURE === name) process.exit(2); };
if (command === "uname") { console.log(args[0] === "-s" ? "Linux" : "x86_64"); process.exit(0); }
if (command === "curl") {
    fail("download");
    const url = args.find(v => v.startsWith("https:"));
    if (args.includes("-o")) {
        let body = url.endsWith("SHA256SUMS") ? process.env.TEST_SUMS : process.env.TEST_COMPOSE;
        fs.writeFileSync(args[args.indexOf("-o") + 1], body);
    } else if (url.endsWith("/latest")) console.log(JSON.stringify({ tag_name: process.env.TEST_VERSION }));
    process.exit(0);
}
if (command === "git") {
    fail("clone");
    const target = args[args.length - 1];
    fs.mkdirSync(path.join(target, "scripts"), { recursive: true });
    fs.copyFileSync(process.env.TEST_INSTALLER, path.join(target, "scripts/install-server-image.sh"));
    fs.writeFileSync(path.join(target, "Dockerfile"), "FROM scratch\\n");
    fs.writeFileSync(path.join(target, "docker-compose.deploy.yml"), process.env.TEST_COMPOSE);
    fs.writeFileSync(path.join(target, "docker-compose.build.yml"), "services: {}\\n");
    if (process.env.TEST_INVALID === "source-data") fs.mkdirSync(path.join(target, "data"));
    process.exit(0);
}
const install = process.env.TEST_INSTALL;
const expected = { backend: ["data", "/data"], postgres: ["postgres-data", "/var/lib/postgresql/data"], redis: ["redis-data", "/data"] };
if (args[0] === "compose") {
    if (args[1] === "version") { console.log("2.39"); process.exit(0); }
    const dir = args[args.indexOf("--project-directory") + 1];
    const env = Object.fromEntries(fs.readFileSync(path.join(dir, ".env"), "utf8").trim().split("\\n").map(line => { const i = line.indexOf("="); return [line.slice(0, i), line.slice(i + 1)]; }));
    if (args.includes("config")) {
        const services = {};
        for (const [service, [folder, target]] of Object.entries(expected)) {
            let source = path.join(dir, folder), type = "bind";
            if (service === "backend" && process.env.CANVAS_DATA_PATH) source = process.env.CANVAS_DATA_PATH;
            if (dir === install && service === "backend" && process.env.TEST_INVALID === "external") source = "/outside/data";
            if (dir === install && process.env.TEST_INVALID === "named") type = "volume";
            services[service] = { volumes: [{ type, source, target }], environment: { CANVAS_DATABASE_DRIVER: "postgres", CANVAS_MANAGED_BINARY_PATH: "/data/system/bin/yingce-backend" } };
        }
        services.backend.ports = [{ target: 8080, published: env.CANVAS_HTTP_PORT || "3000", host_ip: env.CANVAS_HTTP_BIND || "127.0.0.1", protocol: "tcp" }];
        if (dir === install && process.env.TEST_INVALID === "agent") services["yingce-agent"] = {};
        console.log(JSON.stringify({ name: env.COMPOSE_PROJECT_NAME || "old_canvas", services }));
    } else if (args.includes("pull")) fail("pull");
    else if (args.includes("build")) fail("build");
    else if (args.includes("up")) {
        fail("up");
        for (const [folder] of Object.values(expected)) fs.mkdirSync(path.join(dir, folder), { recursive: true });
    }
    process.exit(0);
}
if (args[0] === "ps") {
    if (process.env.TEST_INVALID === "no-containers") process.exit(0);
    console.log("old_backend\\nold_postgres\\nold_redis");
    if (process.env.TEST_INVALID === "project-name" && args.some(v => v.includes("working_dir="))) console.log("another_project_backend");
    process.exit(0);
}
if (args[0] === "inspect") {
    console.log(JSON.stringify(Object.entries(expected).map(([service, [folder, target]]) => ({
        Id: "old_" + service,
        Config: { Labels: {
            "com.docker.compose.project": "old_canvas",
            "com.docker.compose.service": service,
            "com.docker.compose.project.working_dir": process.env.TEST_INVALID === "owner" ? "/another/installation" : install,
            "com.docker.compose.project.config_files": path.join(install, "docker-compose.deploy.yml")
        }},
        Mounts: [{ Type: "bind", Source: process.env.TEST_INVALID === "live-mount" ? "/outside/data" : path.join(install, folder), Destination: target }]
    }))));
    process.exit(0);
}
if (args[0] === "run") { fail("check"); console.log(JSON.stringify({ version: process.env.TEST_FAILURE === "version" ? "v0.0.1" : process.env.TEST_VERSION })); process.exit(0); }
if (args[0] === "stop") fail("stop");
if (args[0] === "rm" || args[0] === "stop" || args[0] === "login") process.exit(0);
process.exit(3);
`;
    for (const name of ["docker", "curl", "uname", "git"]) {
        writeFileSync(path.join(bin, name), mock);
        chmodSync(path.join(bin, name), 0o755);
    }
    const checksum = failure === "checksum" ? "0".repeat(64) : createHash("sha256").update(compose).digest("hex");
    const env = {
        ...process.env, PATH: bin + path.delimiter + process.env.PATH, HOME: home,
        INSTALL_DIR: install, CANVAS_IMAGE_TAG: version, TEST_INSTALL: install, TEST_INSTALLER: installer,
        TEST_LOG: log, TEST_FAILURE: failure, TEST_INVALID: invalid, TEST_VERSION: version, TEST_COMPOSE: compose,
        TEST_SUMS: checksum + "  docker-compose.deploy.yml\n" + "0".repeat(64) + "  open-ai-canvas_" + version + "_linux_amd64.tar.gz\n",
    };
    t.after(() => rmSync(directory, { recursive: true, force: true }));
    return {
        install, directory, home, env,
        run(args = [], overrides = {}, script = installer, input = "") { return spawnSync("bash", [script, ...args], { env: { ...env, ...overrides }, input, encoding: "utf8", timeout: 30000 }); },
        commands() { return existsSync(log) ? readFileSync(log, "utf8").trim().split("\n").map(JSON.parse) : []; },
        backups() { return readdirSync(directory).filter(name => name.startsWith("application.replaced-")).map(name => path.join(directory, name)); },
    };
}

const stopped = f => f.commands().filter(entry => entry.command === "docker" && ["stop", "rm"].includes(entry.args[0]));
function success(result) { assert.equal(result.status, 0, result.stdout + result.stderr); }
function untouched(f) {
    assert.deepEqual(stopped(f), []);
    assert.equal(f.backups().length, 0);
    assert.equal(readFileSync(path.join(f.install, "data/old-data"), "utf8"), "must not enter new deployment");
}

test("first installation uses a fresh Compose project and private new configuration", t => {
    const f = fixture(t);
    success(f.run());
    const env = readFileSync(path.join(f.install, ".env"), "utf8");
    assert.match(env, /COMPOSE_PROJECT_NAME=canvas_[a-f0-9]{16}/);
    assert.match(env, /POSTGRES_PASSWORD=[a-f0-9]{64}/);
    assert.equal(stopped(f).length, 0);
});

test("normal installation never overwrites an existing directory", t => {
    const f = fixture(t, { installed: true });
    assert.notEqual(f.run().status, 0);
    untouched(f);
    assert.ok(!f.commands().some(entry => entry.command === "curl"));
});

test("installation persists explicit private upstream hosts for Compose", t => {
    const f = fixture(t);
    success(f.run([], { CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS: "grok2api,sub2api" }));
    const env = readFileSync(path.join(f.install, ".env"), "utf8");
    assert.match(env, /^CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS=grok2api,sub2api$/m);
    assert.doesNotMatch(env, /^CANVAS_ALLOW_PRIVATE_UPSTREAMS=/m);
});

test("overwrite requires exact confirmation, or explicit --yes", async t => {
    for (const input of ["", "/wrong/directory\n"]) await t.test(JSON.stringify(input), t => {
        const f = fixture(t, { installed: true });
        assert.notEqual(f.run(["--overwrite"], {}, installer, input).status, 0);
        untouched(f);
    });
    await t.test("exact directory", t => {
        const f = fixture(t, { installed: true });
        success(f.run(["--overwrite"], {}, installer, f.install + "\n"));
        assert.equal(f.backups().length, 1);
    });
    await t.test("--yes alone rejected", t => {
        const f = fixture(t);
        assert.notEqual(f.run(["--yes"]).status, 0);
        assert.equal(existsSync(f.install), false);
    });
});

test("overwrite validates before stop, isolates old data, and stops only owned IDs", t => {
    const f = fixture(t, { installed: true });
    success(f.run(["--overwrite", "--yes"], { CANVAS_DATA_PATH: "/must-not-override-old-env" }));
    const [backup] = f.backups();
    assert.ok(backup);
    assert.ok(existsSync(path.join(backup, "data/old-data")));
    assert.ok(!existsSync(path.join(f.install, "data/old-data")));
    assert.ok(readFileSync(path.join(backup, ".env"), "utf8").includes("old-password"));
    assert.ok(!readFileSync(path.join(f.install, ".env"), "utf8").includes("old-password"));
    assert.equal(JSON.parse(readFileSync(path.join(backup, ".canvas-overwrite-recovery.json"), "utf8")).composeProject, "old_canvas");
    const commands = f.commands();
    assert.ok(commands.findIndex(v => v.command === "docker" && v.args[0] === "run") < commands.findIndex(v => v.command === "docker" && v.args[0] === "stop"));
    assert.deepEqual(stopped(f).map(v => v.args), [["stop", "--time", "600", "old_backend", "old_postgres", "old_redis"], ["rm", "old_backend", "old_postgres", "old_redis"]]);
    assert.ok(!commands.some(v => v.args.includes("down") || v.args.includes("--volumes")));
});

test("preflight failures never stop the old deployment", async t => {
    for (const failure of ["download", "checksum", "pull", "check", "version"]) await t.test(failure, t => {
        const f = fixture(t, { installed: true, failure });
        assert.notEqual(f.run(["--overwrite", "--yes"]).status, 0);
        untouched(f);
    });
});

test("unknown layouts, external mounts and other projects are refused", async t => {
    for (const invalid of ["external", "named", "agent", "owner", "live-mount", "project-name"]) await t.test(invalid, t => {
        const f = fixture(t, { installed: true, invalid });
        assert.notEqual(f.run(["--overwrite", "--yes"]).status, 0);
        untouched(f);
    });
    await t.test("independent updater", t => {
        const f = fixture(t, { installed: true });
        writeFileSync(path.join(f.install, ".env"), "CANVAS_UPDATER_TOKEN=fixture\n");
        assert.notEqual(f.run(["--overwrite", "--yes"]).status, 0);
        untouched(f);
    });
});

test("root, home, path traversal and symlinks cannot be installation targets", async t => {
    for (const variant of ["root", "home", "traversal", "symlink", "data-symlink"]) await t.test(variant, t => {
        const f = fixture(t, { installed: true });
        let target = variant === "root" ? "/" : variant === "home" ? f.home : f.install;
        if (variant === "traversal") target = f.directory + "/ignored/../application";
        if (variant === "symlink") { target = path.join(f.directory, "link"); symlinkSync(f.install, target); }
        if (variant === "data-symlink") {
            rmSync(path.join(f.install, "data"), { recursive: true });
            symlinkSync(f.home, path.join(f.install, "data"));
        }
        assert.notEqual(f.run(["--overwrite", "--yes"], { INSTALL_DIR: target }).status, 0);
        assert.deepEqual(stopped(f), []);
        assert.equal(f.backups().length, 0);
    });
});

test("failed startup keeps isolated old configuration and reports recovery location", t => {
    const f = fixture(t, { installed: true, failure: "up" });
    const result = f.run(["--overwrite", "--yes"]);
    assert.notEqual(result.status, 0);
    const [backup] = f.backups();
    assert.ok(backup && existsSync(path.join(backup, "postgres-data/old-data")));
    assert.ok(result.stderr.includes(backup));
    assert.ok(existsSync(path.join(f.install, ".env")));
    const recovery = path.join(backup, ".canvas-overwrite-recovery.sh");
    assert.ok(result.stderr.includes(recovery));
    const metadata = JSON.parse(readFileSync(path.join(backup, ".canvas-overwrite-recovery.json"), "utf8"));
    const before = f.commands().length;
    success(f.run([], { TEST_FAILURE: "" }, recovery));
    assert.ok(readFileSync(path.join(f.install, ".env"), "utf8").includes("old-password"));
    assert.ok(existsSync(path.join(f.install, "postgres-data/old-data")));
    assert.ok(existsSync(path.join(metadata.failedDirectory, ".env")));
    assert.ok(!existsSync(backup));
    const commands = f.commands().slice(before).filter(v => v.command === "docker");
    assert.equal(commands.length, 3);
    for (const entry of commands.slice(0, 2)) {
        assert.equal(entry.args[entry.args.indexOf("--project-name") + 1], metadata.replacementProject);
    }
    assert.ok(commands[0].args.includes("stop"));
    assert.ok(commands[1].args.includes("rm") && commands[1].args.includes("--force"));
    assert.ok(!commands[1].args.includes("--volumes"));
    assert.equal(commands[2].args[commands[2].args.indexOf("--project-name") + 1], "old_canvas");
    assert.ok(commands[2].args.includes("up"));
});

test("a previously stopped deployment can be replaced without inventing container IDs", t => {
    const f = fixture(t, { installed: true, invalid: "no-containers" });
    const result = f.run(["--overwrite", "--yes"]);
    success(result);
    assert.deepEqual(stopped(f), []);
    assert.equal(f.backups().length, 1, result.stdout + result.stderr);
});

test("failed stop leaves old files in place and does not claim an absent backup", t => {
    const f = fixture(t, { installed: true, failure: "stop" });
    const result = f.run(["--overwrite", "--yes"]);
    assert.notEqual(result.status, 0);
    assert.equal(f.backups().length, 0);
    assert.ok(existsSync(path.join(f.install, "postgres-data/old-data")));
    assert.match(result.stderr, /旧目录仍在原位置/);
    const recovery = path.join(f.install, ".canvas-overwrite-recovery.sh");
    assert.ok(result.stderr.includes(recovery));
    const before = f.commands().length;
    success(f.run([], { TEST_FAILURE: "" }, recovery));
    const commands = f.commands().slice(before).filter(v => v.command === "docker");
    assert.equal(commands.length, 1);
    assert.ok(commands[0].args.includes("up"));
    assert.equal(commands[0].args[commands[0].args.indexOf("--project-name") + 1], "old_canvas");
});

test("manual recovery refuses a new directory reassigned to another project", t => {
    const f = fixture(t, { installed: true, failure: "up" });
    assert.notEqual(f.run(["--overwrite", "--yes"]).status, 0);
    const [backup] = f.backups();
    writeFileSync(path.join(f.install, ".env"), "COMPOSE_PROJECT_NAME=another_project\n");
    const before = f.commands().length;
    const result = f.run([], { TEST_FAILURE: "" }, path.join(backup, ".canvas-overwrite-recovery.sh"));
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /其他项目/);
    assert.equal(f.commands().slice(before).filter(v => v.command === "docker").length, 0);
    assert.ok(existsSync(path.join(backup, "postgres-data/old-data")));
});

test("overwrite preserves the verified listener unless explicitly overridden", async t => {
    await t.test("preserve", t => {
        const f = fixture(t, { installed: true });
        const result = f.run(["--overwrite", "--yes"], { CANVAS_HTTP_PORT: undefined, CANVAS_HTTP_BIND: undefined });
        success(result);
        const env = readFileSync(path.join(f.install, ".env"), "utf8");
        assert.match(env, /CANVAS_HTTP_PORT=4567/);
        assert.match(env, /CANVAS_HTTP_BIND=0.0.0.0/);
        assert.match(result.stdout, /0.0.0.0:4567 → 0.0.0.0:4567/);
    });
    await t.test("explicit override", t => {
        const f = fixture(t, { installed: true });
        const result = f.run(["--overwrite", "--yes"], { CANVAS_HTTP_PORT: "6789", CANVAS_HTTP_BIND: "127.0.0.1" });
        success(result);
        assert.match(readFileSync(path.join(f.install, ".env"), "utf8"), /CANVAS_HTTP_PORT=6789/);
        assert.match(result.stdout, /0.0.0.0:4567 → 127.0.0.1:6789/);
    });
});

test("source installer shares overwrite flow and builds before stopping", async t => {
    await t.test("success", t => {
        const f = fixture(t, { installed: true });
        success(f.run(["--overwrite", "--yes"], {}, sourceInstaller));
        const commands = f.commands();
        assert.ok(commands.findIndex(v => v.command === "docker" && v.args.includes("build")) < commands.findIndex(v => v.command === "docker" && v.args[0] === "stop"));
        assert.match(readFileSync(path.join(f.install, ".env"), "utf8"), /CANVAS_IMAGE=open-ai-canvas:source-[a-f0-9]{16}/);
    });
    await t.test("build failure", t => {
        const f = fixture(t, { installed: true, failure: "build" });
        assert.notEqual(f.run(["--overwrite", "--yes"], {}, sourceInstaller).status, 0);
        untouched(f);
    });
    await t.test("source tree with runtime data", t => {
        const f = fixture(t, { installed: true, invalid: "source-data" });
        assert.notEqual(f.run(["--overwrite", "--yes"], {}, sourceInstaller).status, 0);
        untouched(f);
    });
});
