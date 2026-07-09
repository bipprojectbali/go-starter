// Pemilih tema — persisten lintas full-page navigation lewat localStorage.
// File terpisah (bukan inline) agar lolos CSP `script-src 'self'`. Dimuat
// SINKRON di <head> supaya data-theme ter-set sebelum paint (tanpa flash tema
// salah/FOUC).
//
// daisyUI theme-controller sudah mengubah tema via CSS murni (`:has()` pada
// radio yang tercentang), TAPI itu tak persisten & tak berlaku sebelum radio
// ter-render. Maka kita juga set atribut `data-theme` di <html> — yang punya
// spesifisitas lebih tinggi dari `@media (prefers-color-scheme)`, jadi pilihan
// user menang atas default OS. Radio dicentang saat load agar tampilan dropdown
// konsisten dengan tema aktif.
(function () {
  "use strict";
  var KEY = "theme";
  var root = document.documentElement;

  // Terapkan tema tersimpan SEBELUM body dirender (no-FOUC). Bila belum pernah
  // memilih, biarkan tanpa data-theme → daisyUI pakai default/prefersdark (OS).
  var saved = null;
  try {
    saved = localStorage.getItem(KEY);
  } catch (e) {
    /* storage tak tersedia (mis. incognito) — abaikan, ikut default OS */
  }
  if (saved) {
    root.setAttribute("data-theme", saved);
  }

  // Setelah DOM siap: centang radio yang cocok + pasang listener persist.
  function bind() {
    var radios = document.querySelectorAll("input.theme-controller[type=radio]");
    for (var i = 0; i < radios.length; i++) {
      var r = radios[i];
      if (saved && r.value === saved) {
        r.checked = true;
      }
      r.addEventListener("change", onChange);
    }
  }

  function onChange(e) {
    var val = e.target.value;
    root.setAttribute("data-theme", val);
    try {
      localStorage.setItem(KEY, val);
    } catch (err) {
      /* abaikan bila storage tak tersedia */
    }
    // Tutup dropdown <details> setelah memilih (UX; CSS-only tak menutup sendiri).
    var dd = e.target.closest("[data-theme-dropdown]");
    if (dd) {
      dd.removeAttribute("open");
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bind);
  } else {
    bind();
  }
})();
