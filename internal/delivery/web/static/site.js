(function () {
  const token = document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || "";
  window.commuBin = window.commuBin || {};
  window.commuBin.csrfToken = token;
  window.commuBin.csrfFetch = function (input, init) {
    const options = init || {};
    const headers = new Headers(options.headers || {});
    if (token && !headers.has("X-CSRF-Token")) {
      headers.set("X-CSRF-Token", token);
    }
    options.headers = headers;
    return fetch(input, options);
  };
})();

