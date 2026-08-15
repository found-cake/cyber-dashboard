(function (root, factory) {
  "use strict";

  const createCollectionTask = factory(
    typeof module === "object" && module.exports
      ? require("./task-utils.js")
      : root.createSingleActiveTask
  );
  if (typeof module === "object" && module.exports) module.exports = createCollectionTask;
  if (root) root.createCollectionTask = createCollectionTask;
})(typeof window === "undefined" ? globalThis : window, function (createSingleActiveTask) {
  "use strict";

  return function createCollectionTask(requestCollection) {
    const task = createSingleActiveTask();

    return {
      activeDay() {
        return task.value();
      },
      start(day) {
        return task.start(day, () => requestCollection(day));
      },
      resume(day, request) {
        return task.resume(day, request);
      },
      cancel() {
        const request = task.request();
        if (request) request.abort();
      },
      subscribe(listener) {
        return task.subscribe(() => listener(task.value()));
      }
    };
  };
});
