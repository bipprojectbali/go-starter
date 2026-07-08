// Inisialisasi Mermaid untuk diagram ERD + kontrol zoom. File terpisah (bukan
// inline) agar lolos CSP script-src 'self'. mermaid.min.js (UMD) sudah dimuat
// sebelum ini, mengekspos global `mermaid`.
(function () {
  "use strict";
  if (typeof mermaid === "undefined") return;

  mermaid.initialize({
    startOnLoad: true,
    theme: "default",
    securityLevel: "strict", // tak eksekusi skrip dalam diagram
    er: { useMaxWidth: false }, // diagram bisa lebih lebar dari kontainer (scroll)
  });

  function ready(fn) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", fn);
    } else {
      fn();
    }
  }

  ready(function () {
    var diagram = document.getElementById("erd-diagram");
    if (!diagram) return;
    var out = document.getElementById("erd-zoom-out");
    var inn = document.getElementById("erd-zoom-in");
    var reset = document.getElementById("erd-zoom-reset");
    var level = document.getElementById("erd-zoom-level");

    var MIN = 0.25, MAX = 3, STEP = 0.25;
    var scale = 1;

    function apply() {
      scale = Math.min(MAX, Math.max(MIN, scale));
      // Skala pada elemen diagram (Mermaid mengganti isinya jadi SVG saat render;
      // transform tetap berlaku ke SVG di dalamnya).
      diagram.style.transform = "scale(" + scale + ")";
      if (level) level.textContent = Math.round(scale * 100) + "%";
      if (out) out.disabled = scale <= MIN;
      if (inn) inn.disabled = scale >= MAX;
    }

    if (out) out.addEventListener("click", function () { scale -= STEP; apply(); });
    if (inn) inn.addEventListener("click", function () { scale += STEP; apply(); });
    if (reset) reset.addEventListener("click", function () { scale = 1; apply(); });

    apply();
  });
})();
