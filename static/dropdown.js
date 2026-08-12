// Close-on-outside-click untuk dropdown <details class="dropdown"> (menu akun,
// pemilih Tema, switcher workspace). <details> native TIDAK menutup saat klik di
// luar — hanya saat summary-nya diklik lagi. File terpisah (bukan inline) agar
// lolos CSP `script-src 'self'`. Tak butuh pre-paint → dimuat defer.
//
// Perilaku: (a) klik di luar dropdown terbuka → tutup; (b) Escape → tutup semua;
// (c) saat satu dropdown dibuka, tutup yang lain (cegah dua menu terbuka bareng).
(function () {
  "use strict";
  var SEL = "details.dropdown[open]";

  function closeAll(except) {
    var open = document.querySelectorAll(SEL);
    for (var i = 0; i < open.length; i++) {
      if (open[i] !== except) open[i].removeAttribute("open");
    }
  }

  // Klik di luar → tutup dropdown yang tak memuat target klik. Aksi default
  // (toggle summary) berjalan SETELAH listener, jadi buka/tutup via summary
  // sendiri tak terganggu: saat membuka, target ada DI DALAM details.
  document.addEventListener("click", function (e) {
    var open = document.querySelectorAll(SEL);
    for (var i = 0; i < open.length; i++) {
      if (!open[i].contains(e.target)) open[i].removeAttribute("open");
    }
  });

  // Escape → tutup semua (pola menu umum).
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") closeAll(null);
  });

  // Satu dropdown dibuka → tutup sisanya. Event `toggle` tak bubble, jadi
  // didengar di fase CAPTURE (document → target) agar tetap tertangkap.
  document.addEventListener(
    "toggle",
    function (e) {
      var d = e.target;
      if (
        d.tagName === "DETAILS" &&
        d.classList.contains("dropdown") &&
        d.open
      ) {
        closeAll(d);
      }
    },
    true
  );
})();
