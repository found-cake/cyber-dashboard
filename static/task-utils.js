(function (root, factory) {
  "use strict";

  const createSingleActiveTask = factory();
  if (typeof module === "object" && module.exports) module.exports = createSingleActiveTask;
  if (root) root.createSingleActiveTask = createSingleActiveTask;
})(typeof window === "undefined" ? globalThis : window, function () {
  "use strict";

  return function createSingleActiveTask() {
    let active = null;
    const listeners = new Set();

    function notify() {
      listeners.forEach(listener => listener());
    }

    function activate(value, request) {
      if (active) return null;
      active = { value, request };
      notify();
      request.always(() => {
        if (!active || active.request !== request) return;
        active = null;
        notify();
      });
      return request;
    }

    return {
      start(value, requestFactory) {
        return active ? null : activate(value, requestFactory());
      },
      resume(value, request) {
        return activate(value, request);
      },
      value() {
        return active ? active.value : null;
      },
      request() {
        return active ? active.request : null;
      },
      isActive() {
        return active !== null;
      },
      subscribe(listener) {
        listeners.add(listener);
        return () => listeners.delete(listener);
      }
    };
  };
});
