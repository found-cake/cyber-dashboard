(function (root, factory) {
  "use strict";

  const createAuthTransport = factory();
  if (typeof module === "object" && module.exports) module.exports = createAuthTransport;
  if (root) root.createAuthTransport = createAuthTransport;
})(typeof window === "undefined" ? globalThis : window, function () {
  "use strict";

  const refreshExemptPaths = ["/api/auth/login", "/api/auth/refresh", "/api/auth/logout"];

  return function createAuthTransport({ $, locks, isAuthenticated, onSessionExpired }) {
    let pendingRefresh = null;

    function request(method, path, data) {
      return $.ajax({
        method,
        url: path,
        contentType: data ? "application/json" : undefined,
        data: data ? JSON.stringify(data) : undefined,
        dataType: "json"
      });
    }

    function issueRefresh() {
      const send = () => Promise.resolve(request("POST", "/api/auth/refresh"));
      const operation = locks && typeof locks.request === "function"
        ? locks.request("cyber-dashboard-refresh", send)
        : send();
      const deferred = $.Deferred();
      operation.then(deferred.resolve, deferred.reject);
      return deferred.promise();
    }

    function refreshSession() {
      if (pendingRefresh) return pendingRefresh;
      pendingRefresh = issueRefresh();
      pendingRefresh.always(() => { pendingRefresh = null; });
      return pendingRefresh;
    }

    function api(method, path, data) {
      return request(method, path, data).then(null, error => {
        const canRefresh = error.status === 401
          && !refreshExemptPaths.includes(path)
          && isAuthenticated();
        if (!canRefresh) return $.Deferred().reject(error).promise();
        const retry = () => request(method, path, data);
        return refreshSession().then(retry, () => onSessionExpired(retry, error));
      });
    }

    return { request, api, refreshSession };
  };
});
