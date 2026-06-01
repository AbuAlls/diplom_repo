/* config.js — runtime configuration for the frontend.
   Override window.API_BASE before this script loads to point at another backend. */
window.API_BASE = window.API_BASE || 'http://localhost:8080';

/* Демо-аккаунт: единственный аккаунт, у которого на главной показывается
   готовый «витринный» пресет (мок-документы, инсайты, риски). Все остальные
   аккаунты стартуют пустыми и видят только свои реальные данные. */
window.DEMO_EMAIL = 'demo@orbita.ru';
window.DEMO_PASSWORD = 'demo12345';
window.DEMO_NAME = 'Алексей Климов';
window.DEMO_ORG = 'ООО «Орбита»';
