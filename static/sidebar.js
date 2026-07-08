// Sidebar collapse state — persisten lintas full-page navigation lewat
// localStorage. File terpisah (bukan inline) agar lolos CSP `script-src 'self'`.
// Dimuat SINKRON di <head> supaya kelas ter-set sebelum paint (tanpa flicker).
(function () {
  "use strict";
  var KEY = "sidebar-collapsed";
  var root = document.documentElement;

  // Terapkan state tersimpan sebelum body dirender.
  try {
    if (localStorage.getItem(KEY) === "1") {
      root.setAttribute("data-sidebar", "collapsed");
    }
  } catch (e) {
    /* storage tak tersedia (mis. incognito) — abaikan, default expanded */
  }

  // Pasang handler toggle setelah DOM siap.
  function bind() {
    var btn = document.querySelector("[data-sidebar-toggle]");
    if (!btn) return;
    btn.addEventListener("click", function () {
      var collapsed = root.getAttribute("data-sidebar") === "collapsed";
      if (collapsed) {
        root.removeAttribute("data-sidebar");
        try { localStorage.setItem(KEY, "0"); } catch (e) {}
      } else {
        root.setAttribute("data-sidebar", "collapsed");
        try { localStorage.setItem(KEY, "1"); } catch (e) {}
      }
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bind);
  } else {
    bind();
  }
})();
