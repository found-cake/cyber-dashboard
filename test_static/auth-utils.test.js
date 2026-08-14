"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const createAuthTransport = require("../static/auth-utils.js");

function deferred() {
  let resolveNative;
  let rejectNative;
  const native = new Promise((resolve, reject) => {
    resolveNative = resolve;
    rejectNative = reject;
  });
  const promise = {
    then(onFulfilled, onRejected) {
      return native.then(onFulfilled, onRejected);
    },
    done(callback) {
      native.then(callback, () => {});
      return promise;
    },
    fail(callback) {
      native.then(() => {}, callback);
      return promise;
    },
    always(callback) {
      native.then(callback, callback);
      return promise;
    }
  };
  const control = {
    resolve(value) {
      resolveNative(value);
      return control;
    },
    reject(error) {
      rejectNative(error);
      return control;
    },
    promise() {
      return promise;
    }
  };
  return control;
}

function harness(options = {}) {
  const calls = [];
  const requests = [];
  const $ = {
    ajax(settings) {
      calls.push(settings);
      const next = deferred();
      requests.push(next);
      return next.promise();
    },
    Deferred: deferred
  };
  const sessionExpirations = [];
  const transport = createAuthTransport({
    $,
    locks: options.locks,
    isAuthenticated: options.isAuthenticated || (() => true),
    onSessionExpired: options.onSessionExpired || ((retry, error) => {
      sessionExpirations.push({ retry, error });
      return $.Deferred().reject(error).promise();
    })
  });
  return { $, calls, requests, sessionExpirations, transport };
}

async function until(predicate) {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    if (predicate()) return;
    await new Promise(resolve => setImmediate(resolve));
  }
  assert.fail("condition was not reached");
}

test("UMD exports the factory through CommonJS and the browser global", () => {
  const source = fs.readFileSync(path.join(__dirname, "../static/auth-utils.js"), "utf8");
  const browser = { window: {} };

  vm.runInNewContext(source, browser);

  assert.equal(typeof createAuthTransport, "function");
  assert.equal(typeof browser.window.createAuthTransport, "function");
});

test("request sends the exact JSON ajax options", async () => {
  const { calls, requests, transport } = harness();

  const withBody = transport.request("POST", "/api/reports", { title: "Daily" });
  assert.deepEqual(calls[0], {
    method: "POST",
    url: "/api/reports",
    contentType: "application/json",
    data: JSON.stringify({ title: "Daily" }),
    dataType: "json"
  });
  requests[0].resolve({ id: 7 });
  assert.deepEqual(await withBody, { id: 7 });

  const withoutBody = transport.request("GET", "/api/bootstrap");
  assert.deepEqual(calls[1], {
    method: "GET",
    url: "/api/bootstrap",
    contentType: undefined,
    data: undefined,
    dataType: "json"
  });
  requests[1].resolve({ ok: true });
  assert.deepEqual(await withoutBody, { ok: true });
});

test("login, refresh, and logout 401 responses are refresh-exempt", async () => {
  for (const endpoint of ["/api/auth/login", "/api/auth/refresh", "/api/auth/logout"]) {
    const { calls, requests, transport } = harness();
    const originalError = { status: 401, endpoint };

    const result = transport.api("POST", endpoint).then(
      () => assert.fail(`${endpoint} unexpectedly resolved`),
      error => error
    );
    requests[0].reject(originalError);

    assert.equal(await result, originalError);
    assert.equal(calls.length, 1);
  }
});

test("an unauthenticated request does not attempt refresh", async () => {
  const { calls, requests, transport } = harness({ isAuthenticated: () => false });
  const originalError = { status: 401 };
  const result = transport.api("GET", "/api/reports").then(
    () => assert.fail("request unexpectedly resolved"),
    error => error
  );

  requests[0].reject(originalError);

  assert.equal(await result, originalError);
  assert.equal(calls.length, 1);
});

test("a non-exempt 401 refreshes once and replays the raw request once", async () => {
  const { calls, requests, transport } = harness();
  const result = transport.api("PATCH", "/api/settings", { language: "ko" });

  requests[0].reject({ status: 401 });
  await until(() => requests.length === 2);
  requests[1].resolve({ refreshed: true });
  await until(() => requests.length === 3);
  requests[2].resolve({ saved: true });

  assert.deepEqual(await result, { saved: true });
  assert.deepEqual(calls.map(call => [call.method, call.url]), [
    ["PATCH", "/api/settings"],
    ["POST", "/api/auth/refresh"],
    ["PATCH", "/api/settings"]
  ]);
  assert.deepEqual(calls[2], calls[0]);
});

test("a replayed 401 is returned without recursive refresh", async () => {
  const { calls, requests, transport } = harness();
  const replayError = { status: 401, replay: true };
  const result = transport.api("DELETE", "/api/reports/9").then(
    () => assert.fail("request unexpectedly resolved"),
    error => error
  );

  requests[0].reject({ status: 401 });
  await until(() => requests.length === 2);
  requests[1].resolve({ refreshed: true });
  await until(() => requests.length === 3);
  requests[2].reject(replayError);

  assert.equal(await result, replayError);
  assert.equal(calls.length, 3);
  assert.equal(calls.filter(call => call.url === "/api/auth/refresh").length, 1);
});

test("concurrent 401 responses share one in-tab refresh without Web Locks", async () => {
  const { calls, requests, transport } = harness();
  const first = transport.api("GET", "/api/reports/1");
  const second = transport.api("GET", "/api/reports/2");

  requests[0].reject({ status: 401 });
  requests[1].reject({ status: 401 });
  await until(() => requests.length === 3);
  assert.equal(calls.filter(call => call.url === "/api/auth/refresh").length, 1);

  requests[2].resolve({ refreshed: true });
  await until(() => requests.length === 5);
  requests[3].resolve({ id: 1 });
  requests[4].resolve({ id: 2 });

  assert.deepEqual(await Promise.all([first, second]), [{ id: 1 }, { id: 2 }]);
  assert.equal(calls.length, 5);
});

test("concurrent 401 responses share one refresh through the named Web Lock", async () => {
  const lockCalls = [];
  const locks = {
    request(name, callback) {
      lockCalls.push(name);
      return Promise.resolve().then(callback);
    }
  };
  const { calls, requests, transport } = harness({ locks });
  const first = transport.api("GET", "/api/sources");
  const second = transport.api("GET", "/api/settings");

  requests[0].reject({ status: 401 });
  requests[1].reject({ status: 401 });
  await until(() => requests.length === 3);

  assert.deepEqual(lockCalls, ["cyber-dashboard-refresh"]);
  assert.equal(typeof transport.refreshSession().always, "function");
  assert.equal(calls.filter(call => call.url === "/api/auth/refresh").length, 1);

  requests[2].resolve({ refreshed: true });
  await until(() => requests.length === 5);
  requests[3].resolve({ sources: [] });
  requests[4].resolve({ settings: {} });

  assert.deepEqual(await Promise.all([first, second]), [{ sources: [] }, { settings: {} }]);
  assert.equal(lockCalls.length, 1);
});

test("refresh failure delegates retry and the original 401 to session expiry", async () => {
  const delegated = [];
  const environment = harness({
    onSessionExpired(retry, originalError) {
      delegated.push({ retry, originalError });
      return retry();
    }
  });
  const { calls, requests, transport } = environment;
  const originalError = { status: 401, original: true };
  const refreshError = { status: 401, refresh: true };
  const result = transport.api("POST", "/api/collect", { day: "2026-08-14" });

  requests[0].reject(originalError);
  await until(() => requests.length === 2);
  requests[1].reject(refreshError);
  await until(() => requests.length === 3);
  requests[2].resolve({ job_id: "job-1" });

  assert.deepEqual(await result, { job_id: "job-1" });
  assert.equal(delegated.length, 1);
  assert.equal(typeof delegated[0].retry, "function");
  assert.equal(delegated[0].originalError, originalError);
  assert.equal(calls.filter(call => call.url === "/api/auth/refresh").length, 1);
  assert.deepEqual(calls[2], calls[0]);
});

test("index loads auth-utils exactly once after jQuery and before app", () => {
  const html = fs.readFileSync(path.join(__dirname, "../static/index.html"), "utf8");
  const jquery = '<script defer src="/jquery-4.0.0.min.js"></script>';
  const auth = '<script defer src="/auth-utils.js"></script>';
  const app = '<script defer src="/app.js"></script>';

  assert.equal(html.split(auth).length - 1, 1);
  assert.ok(html.indexOf(jquery) < html.indexOf(auth));
  assert.ok(html.indexOf(auth) < html.indexOf(app));
});
