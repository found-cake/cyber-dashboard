"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const createCollectionTask = require("../static/collection-task.js");

function pendingRequest() {
  const callbacks = [];
  return {
    abortCount: 0,
    always(callback) {
      callbacks.push(callback);
      return this;
    },
    finish() {
      callbacks.forEach(callback => callback());
    },
    abort() {
      this.abortCount += 1;
      this.finish();
    }
  };
}

test("collection stays locked until the active request finishes", () => {
  const first = pendingRequest();
  const task = createCollectionTask(() => first);

  assert.equal(task.start("2026-08-03"), first);
  assert.equal(task.activeDay(), "2026-08-03");
  assert.equal(task.start("2026-08-03"), null);

  first.finish();
  assert.equal(task.activeDay(), null);
});

test("cancel aborts the active request and releases the lock", () => {
  const request = pendingRequest();
  const states = [];
  const task = createCollectionTask(() => request);
  task.subscribe(day => states.push(day));

  task.start("2026-08-02");
  task.cancel();

  assert.equal(request.abortCount, 1);
  assert.equal(task.activeDay(), null);
  assert.deepEqual(states, ["2026-08-02", null]);
});

test("resume attaches to an existing server job without starting another", () => {
  const request = pendingRequest();
  let starts = 0;
  const task = createCollectionTask(() => { starts += 1; return pendingRequest(); });

  task.resume("2026-08-03", request);

  assert.equal(starts, 0);
  assert.equal(task.activeDay(), "2026-08-03");
  request.finish();
});
