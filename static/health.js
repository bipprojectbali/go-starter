// File Health — filter + pagination + copy, sepenuhnya client-side. File
// terpisah (bukan inline) agar lolos CSP `script-src 'self'`. Semua baris
// sudah di-render server (dengan atribut data-*); skrip ini hanya menyaring,
// memenggal halaman, dan menyalin path — tak ada request tambahan.
(function () {
  "use strict";
  var PAGE_SIZE = 20;

  function ready(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
    } else {
      fn();
    }
  }

  ready(function () {
    var root = document.getElementById("health-panel");
    if (!root) return;

    var rows = Array.prototype.slice.call(root.querySelectorAll("[data-health-row]"));
    var qEl = root.querySelector("#health-q");
    var statusEl = root.querySelector("#health-status");
    var kindEl = root.querySelector("#health-kind");
    var pageInfo = root.querySelector("#health-page-info");
    var prevBtn = root.querySelector("#health-prev");
    var nextBtn = root.querySelector("#health-next");
    var selAll = root.querySelector("#health-select-all");
    var copySel = root.querySelector("#health-copy-selected");
    var copyBad = root.querySelector("#health-copy-unhealthy");
    var toast = root.querySelector("#health-toast");

    var page = 0;
    var filtered = rows;

    function matches(row) {
      var q = (qEl.value || "").toLowerCase().trim();
      var status = statusEl.value; // ""|healthy|unhealthy
      var kind = kindEl.value;     // ""|<kind>
      if (q && row.getAttribute("data-path").toLowerCase().indexOf(q) === -1) return false;
      if (status && row.getAttribute("data-status") !== status) return false;
      if (kind && row.getAttribute("data-kind") !== kind) return false;
      return true;
    }

    function applyFilter() {
      filtered = rows.filter(matches);
      page = 0;
      render();
    }

    function render() {
      var totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
      if (page >= totalPages) page = totalPages - 1;
      var start = page * PAGE_SIZE;
      var end = start + PAGE_SIZE;

      rows.forEach(function (r) { r.style.display = "none"; });
      filtered.slice(start, end).forEach(function (r) { r.style.display = ""; });

      var shown = Math.min(end, filtered.length);
      pageInfo.textContent = filtered.length === 0
        ? "0 file"
        : (start + 1) + "–" + shown + " dari " + filtered.length + " file";
      prevBtn.disabled = page === 0;
      nextBtn.disabled = page >= totalPages - 1;
    }

    function flash(msg) {
      if (!toast) return;
      toast.textContent = msg;
      toast.style.opacity = "1";
      setTimeout(function () { toast.style.opacity = "0"; }, 2000);
    }

    function copyPaths(paths) {
      // Tak ada pilihan → diam (jangan ganggu dgn toast noise).
      if (paths.length === 0) return;
      var text = paths.join("\n");
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(
          function () { flash(paths.length + " path tersalin"); },
          function () { flash("Gagal menyalin"); }
        );
      } else {
        flash("Clipboard tak tersedia");
      }
    }

    // Filter events.
    qEl.addEventListener("input", applyFilter);
    statusEl.addEventListener("change", applyFilter);
    kindEl.addEventListener("change", applyFilter);

    // Pagination.
    prevBtn.addEventListener("click", function () { if (page > 0) { page--; render(); } });
    nextBtn.addEventListener("click", function () { page++; render(); });

    // Select-all: hanya baris yang sedang terlihat (halaman + filter aktif).
    selAll.addEventListener("change", function () {
      var start = page * PAGE_SIZE;
      filtered.slice(start, start + PAGE_SIZE).forEach(function (r) {
        var cb = r.querySelector("[data-health-check]");
        if (cb) cb.checked = selAll.checked;
      });
    });

    // Copy terpilih (checkbox tercentang, lintas halaman).
    copySel.addEventListener("click", function () {
      var paths = rows
        .filter(function (r) {
          var cb = r.querySelector("[data-health-check]");
          return cb && cb.checked;
        })
        .map(function (r) { return r.getAttribute("data-path"); });
      copyPaths(paths);
    });

    // Copy semua file bermasalah (abaikan filter & centang).
    copyBad.addEventListener("click", function () {
      var paths = rows
        .filter(function (r) { return r.getAttribute("data-status") === "unhealthy"; })
        .map(function (r) { return r.getAttribute("data-path"); });
      copyPaths(paths);
    });

    render();
  });
})();
