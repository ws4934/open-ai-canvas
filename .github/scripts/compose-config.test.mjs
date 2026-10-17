import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { after, test } from "node:test";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));
const temporary = mkdtempSync(path.join(os.tmpdir(), "canvas-compose-"));
const envFile = path.join(temporary, ".env");
writeFileSync(envFile, "");
after(() => rmSync(temporary, { recursive: true, force: true }));

const password = "fixture@:/?%# '\"$ with spaces";
const variants = [
    ["default", ["docker-compose.yml"]],
    ["local", ["docker-compose.local.yml"]],
    ["development", ["docker-compose.dev.yml"]],
    ["release", ["docker-compose.deploy.yml"]],
    ["source server", ["docker-compose.deploy.yml", "docker-compose.build.yml"]],
    ["server alias", ["docker-compose.server.yml"]],
];

function configuration(files, overrides = {}) {
    const env = {
        ...process.env,
        POSTGRES_PASSWORD: password,
        POSTGRES_DB: "contract_db",
        POSTGRES_USER: "contract_user",
        CANVAS_IMAGE_TAG: "1.2.3",
        CANVAS_IMAGE: "",
        CANVAS_DATABASE_DRIVER: "sqlite",
        DATABASE_URL: "file:/must-not-be-used.db",
        REDIS_URL: "",
        CANVAS_DATA_PATH: "",
        CANVAS_POSTGRES_DATA_PATH: "",
        CANVAS_REDIS_DATA_PATH: "",
        CANVAS_HTTP_BIND: "127.0.0.1",
        CANVAS_HTTP_PORT: "34567",
        CANVAS_WEB_HOST_PORT: "34567",
        CANVAS_BACKEND_HOST_PORT: "34568",
        ...overrides,
    };
    for (const [key, value] of Object.entries(env)) if (value === undefined) delete env[key];
    return spawnSync("docker", ["compose", "--project-directory", root, "--env-file", envFile, ...files.flatMap((file) => ["-f", file]), "config", "--format", "json"], {
        cwd: root,
        env,
        encoding: "utf8",
    });
}

for (const [name, files] of variants) {
    test(name + " uses one PostgreSQL credential set and private persistent infrastructure", () => {
        const result = configuration(files);
        assert.equal(result.status, 0, result.stderr || String(result.error));
        const config = JSON.parse(result.stdout);
        const { backend, postgres, redis } = config.services;
        assert.equal(backend.environment.CANVAS_DATABASE_DRIVER, "postgres");
        assert.equal(backend.environment.DATABASE_URL, "postgresql://postgres:5432/?sslmode=disable");
        assert.equal(backend.environment.PGDATABASE, postgres.environment.POSTGRES_DB);
        assert.equal(backend.environment.PGUSER, postgres.environment.POSTGRES_USER);
        // Compose serializes literal dollars as $$; both containers receive the same original value.
        assert.equal(backend.environment.PGPASSWORD, postgres.environment.POSTGRES_PASSWORD);
        assert.equal(backend.environment.PGPASSWORD.replaceAll("$$", "$"), password);
        assert.equal(backend.environment.REDIS_URL, "redis://redis:6379/0");
        assert.equal(backend.depends_on.postgres.condition, "service_healthy");
        assert.equal(backend.depends_on.redis.condition, "service_healthy");
        for (const service of [postgres, redis]) {
            assert.ok(!service.ports?.length);
            assert.equal(service.volumes[0].type, "volume");
            assert.ok(config.volumes[service.volumes[0].source]);
            assert.ok(service.healthcheck.test.length);
            assert.equal(service.restart, "unless-stopped");
        }
        if (name === "development") {
            assert.equal(config.services.web.ports[0].published, "34567");
            assert.equal(backend.ports[0].published, "34568");
            assert.ok(backend.volumes.some((mount) => mount.target === "/opt/canvas-agent/agent-runtime.mjs" && mount.read_only));
        } else {
            assert.deepEqual(Object.keys(config.services).sort(), ["backend", "postgres", "redis"]);
            assert.equal(backend.ports[0].published, "34567");
            assert.equal(backend.init, true);
        }
    });

    test(name + " accepts only a password and preserves container healthcheck variables", () => {
        const result = configuration(files, { POSTGRES_DB: undefined, POSTGRES_USER: undefined });
        assert.equal(result.status, 0, result.stderr || String(result.error));
        assert.doesNotMatch(result.stderr, /POSTGRES_(DB|USER).*not set/i);
        const { backend, postgres } = JSON.parse(result.stdout).services;
        assert.equal(postgres.environment.POSTGRES_DB, "open_ai_canvas");
        assert.equal(postgres.environment.POSTGRES_USER, "open_ai_canvas");
        assert.equal(backend.environment.PGDATABASE, "open_ai_canvas");
        assert.equal(backend.environment.PGUSER, "open_ai_canvas");
        assert.match(postgres.healthcheck.test[1], /\$POSTGRES_USER/);
        assert.match(postgres.healthcheck.test[1], /\$POSTGRES_DB/);
        assert.doesNotMatch(postgres.healthcheck.test[1], /open_ai_canvas/);
    });

    test(name + " rejects an empty password instead of falling back to SQLite", () => {
        const result = configuration(files, { POSTGRES_PASSWORD: "" });
        assert.notEqual(result.status, 0);
        assert.match(result.stderr, /POSTGRES_PASSWORD/);
    });
}
