/* config.js — runtime configuration for the frontend.
   Override window.API_BASE before this script loads to point at another backend.

   Default is SAME-ORIGIN ('' -> requests go to /v0/..., /api/..., /healthz on the
   host serving this page). This is what the production Caddy setup uses: the
   frontend and API live on one domain, so no CORS and no hardcoded host.

   For pure local dev (frontend on :5500 via serve.py, backend on :8080) set the
   backend explicitly, e.g. open the page as:
     http://localhost:5500/?api=http://localhost:8080
   or hardcode it here temporarily. */
(function () {
  var params = new URLSearchParams(window.location.search);
  var override = params.get('api'); // optional ?api=http://host:port for local dev
  if (override) {
    window.API_BASE = override;
  } else if (typeof window.API_BASE === 'undefined') {
    // Same-origin by default. On localhost:5500 (serve.py) fall back to :8080
    // so the standalone dev server still reaches the local backend.
    if (window.location.port === '5500') {
      window.API_BASE = 'http://localhost:8080';
    } else {
      window.API_BASE = ''; // relative -> same host that served the page
    }
  }
})();

/* Демо-аккаунт: единственный аккаунт, у которого на главной показывается
   готовый «витринный» пресет (мок-документы, инсайты, риски). Все остальные
   аккаунты стартуют пустыми и видят только свои реальные данные. */
window.DEMO_EMAIL = 'demo@orbita.ru';
window.DEMO_PASSWORD = 'demo12345';
window.DEMO_NAME = 'Алексей Климов';
window.DEMO_ORG = 'ООО «Орбита»';
