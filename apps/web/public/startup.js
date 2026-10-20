// Standalone so a failed application module still leaves an accessible error.
(() => {
  let ready = false;
  function unavailable() {
    if (ready) return;
    const root = document.getElementById("root");
    if (!root) return;
    const message = document.createElement("p");
    message.setAttribute("role", "alert");
    message.textContent =
      "Kurier could not finish loading. Check your connection and reload the page.";
    const reload = document.createElement("a");
    reload.href = "/";
    reload.textContent = "Reload Kurier";
    root.replaceChildren(message, reload);
  }
  window.addEventListener(
    "kurier:ready",
    () => {
      ready = true;
    },
    { once: true },
  );
  window.addEventListener(
    "error",
    (event) => {
      if (
        event.target instanceof HTMLScriptElement ||
        event.target instanceof HTMLLinkElement
      )
        unavailable();
    },
    true,
  );
  window.addEventListener("unhandledrejection", unavailable);
  setTimeout(unavailable, 20000);
})();
