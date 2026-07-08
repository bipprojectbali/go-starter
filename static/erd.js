// Inisialisasi Mermaid untuk diagram ERD. File terpisah (bukan inline) agar
// lolos CSP script-src 'self'. mermaid.min.js (UMD) sudah dimuat sebelum ini,
// mengekspos global `mermaid`.
(function () {
  "use strict";
  if (typeof mermaid === "undefined") return;
  mermaid.initialize({
    startOnLoad: true,
    theme: "default",
    securityLevel: "strict", // tak eksekusi skrip dalam diagram
    er: { useMaxWidth: false }, // biar diagram bisa lebih lebar dari kontainer (scroll)
  });
})();
