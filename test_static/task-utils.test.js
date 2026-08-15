"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const createSingleActiveTask = require("../static/task-utils.js");

function pendingRequest() {
  const callbacks = [];
  return {
    always(callback) {
      callbacks.push(callback);
      return this;
    },
    finish() {
      callbacks.forEach(callback => callback());
    }
  };
}

test("single active task starts lazily and releases on completion", () => {
  const first = pendingRequest();
  const states = [];
  let starts = 0;
  const task = createSingleActiveTask();
  task.subscribe(() => states.push(task.value()));

  assert.equal(task.start("first", () => {
    starts += 1;
    return first;
  }), first);
  assert.equal(task.start("second", () => {
    starts += 1;
    return pendingRequest();
  }), null);
  assert.equal(starts, 1);
  assert.equal(task.isActive(), true);
  assert.equal(task.value(), "first");
  assert.equal(task.request(), first);

  first.finish();
  assert.equal(task.isActive(), false);
  assert.equal(task.value(), null);
  assert.equal(task.request(), null);
  assert.deepEqual(states, ["first", null]);
});

test("stale completion does not release a replacement request", () => {
  const first = pendingRequest();
  const second = pendingRequest();
  const task = createSingleActiveTask();

  task.resume("first", first);
  first.finish();
  task.resume("second", second);
  first.finish();

  assert.equal(task.isActive(), true);
  assert.equal(task.value(), "second");
  assert.equal(task.request(), second);
});

test("resume rejects a request while another request is active", () => {
  const first = pendingRequest();
  const second = pendingRequest();
  const task = createSingleActiveTask();

  assert.equal(task.resume("first", first), first);
  assert.equal(task.resume("second", second), null);
  assert.equal(task.value(), "first");
});

test("unsubscribe stops active-state notifications", () => {
  const request = pendingRequest();
  let notifications = 0;
  const task = createSingleActiveTask();
  const unsubscribe = task.subscribe(() => {
    notifications += 1;
  });

  unsubscribe();
  task.resume("active", request);
  request.finish();

  assert.equal(notifications, 0);
});
