(function (root, factory) {
  "use strict";

  const createCollectionTask = factory();
  if (typeof module === "object" && module.exports) module.exports = createCollectionTask;
  if (root) root.createCollectionTask = createCollectionTask;
})(typeof window === "undefined" ? globalThis : window, function () {
  "use strict";

  return function createCollectionTask(requestCollection) {
    let active = null;
    const listeners = new Set();

    function notify() {
      listeners.forEach(listener => listener(active ? active.day : null));
    }

    function activate(day, request) {
      if (active) return null;
      active = { day, request };
      notify();
      request.always(() => {
        if (!active || active.request !== request) return;
        active = null;
        notify();
      });
      return request;
    }

    return {
      activeDay() {
        return active ? active.day : null;
      },
      start(day) {
        return active ? null : activate(day, requestCollection(day));
      },
      resume(day, request) {
        return activate(day, request);
      },
      cancel() {
        if (active) active.request.abort();
      },
      subscribe(listener) {
        listeners.add(listener);
        return () => listeners.delete(listener);
      }
    };
  };
});
