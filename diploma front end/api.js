/* api.js — единственный сетевой слой фронтенда.
   Оборачивает REST-эндпоинты Go-бэкенда (см. project/internal/adapters/httpapi).
   Всё авторизуется Bearer-токеном; токен хранится в localStorage. */
(function () {
  // Same-origin is represented by API_BASE = '' (empty string). Use a typeof
  // check, NOT `|| fallback`, because '' is falsy and would wrongly fall back to
  // :8080 — which the production Caddy setup does not expose, breaking fetches.
  const BASE = (typeof window.API_BASE === 'string') ? window.API_BASE : 'http://localhost:8080';
  const TOKEN_KEY = 'access_token';
  const EMAIL_KEY = 'user_email';
  const NAME_KEY = 'user_name';

  const getToken = () => localStorage.getItem(TOKEN_KEY);
  const setToken = (t) => (t ? localStorage.setItem(TOKEN_KEY, t) : localStorage.removeItem(TOKEN_KEY));
  const getEmail = () => localStorage.getItem(EMAIL_KEY) || '';
  const getName = () => localStorage.getItem(NAME_KEY) || '';

  // Поднимается при 401, чтобы App мог вернуть пользователя на экран входа.
  class AuthError extends Error {}
  // Ошибка с разобранным конвертом {error:{code,message}} бэкенда.
  class ApiError extends Error {
    constructor(message, code, status) {
      super(message);
      this.code = code;
      this.status = status;
    }
  }

  // Низкоуровневый запрос. opts.body — объект (JSON), opts.form — объект (urlencoded),
  // opts.raw — FormData/Blob как есть. Возвращает распарсенный JSON либо null (204).
  async function request(path, opts = {}) {
    const headers = Object.assign({}, opts.headers);
    let body = opts.raw || null;

    if (opts.body !== undefined) {
      headers['Content-Type'] = 'application/json';
      body = JSON.stringify(opts.body);
    } else if (opts.form !== undefined) {
      headers['Content-Type'] = 'application/x-www-form-urlencoded';
      body = new URLSearchParams(opts.form).toString();
    }

    if (opts.auth !== false) {
      const tok = getToken();
      if (tok) headers['Authorization'] = 'Bearer ' + tok;
    }

    const res = await fetch(BASE + path, { method: opts.method || 'GET', headers, body });

    if (res.status === 401) {
      setToken(null);
      window.dispatchEvent(new Event('api:unauthorized'));
      throw new AuthError('Сессия истекла, войдите снова');
    }
    if (res.status === 204) return null;

    const text = await res.text();
    let data = null;
    if (text) {
      try { data = JSON.parse(text); } catch (e) { data = text; }
    }
    if (!res.ok) {
      const err = data && data.error;
      throw new ApiError(
        (err && err.message) || ('Ошибка запроса (' + res.status + ')'),
        err && err.code,
        res.status
      );
    }
    return data;
  }

  const qs = (params) => {
    const p = new URLSearchParams();
    Object.entries(params || {}).forEach(([k, v]) => {
      if (v !== undefined && v !== null && v !== '') p.set(k, v);
    });
    const s = p.toString();
    return s ? '?' + s : '';
  };

  window.API = {
    AuthError,
    ApiError,
    base: BASE,
    isAuthed: () => !!getToken(),
    email: getEmail,
    name: getName,
    // Текущий аккаунт — демонстрационный (показывает витринный пресет)?
    isDemo: () => getEmail().toLowerCase() === String(window.DEMO_EMAIL || '').toLowerCase(),
    // Данные для отображения профиля (сайдбар, настройки).
    currentUser() {
      const demo = this.isDemo();
      const email = getEmail();
      const name = demo ? window.DEMO_NAME : (getName() || (email ? email.split('@')[0] : 'Пользователь'));
      const org = demo ? window.DEMO_ORG : email;
      const initials = (name || '?').trim().split(/\s+/).map((w) => w[0]).slice(0, 2).join('').toUpperCase();
      return { name, org, email, initials, demo };
    },

    auth: {
      // POST /v0/auth/token — form username/password (username = email).
      async login(email, password) {
        const res = await request('/v0/auth/token', {
          method: 'POST', auth: false,
          form: { username: email, password },
        });
        setToken(res.access_token);
        localStorage.setItem(EMAIL_KEY, email);
        return res;
      },
      // POST /v0/auth/register — JSON email/password/full_name.
      async register(email, password, fullName) {
        const res = await request('/v0/auth/register', {
          method: 'POST', auth: false,
          body: { email, password, full_name: fullName },
        });
        setToken(res.access_token);
        localStorage.setItem(EMAIL_KEY, email);
        if (fullName) localStorage.setItem(NAME_KEY, fullName);
        return res;
      },
      // Вход в демо-аккаунт: пробуем логин, при отсутствии — регистрируем.
      async demo() {
        try {
          await this.login(window.DEMO_EMAIL, window.DEMO_PASSWORD);
        } catch (e) {
          await this.register(window.DEMO_EMAIL, window.DEMO_PASSWORD, window.DEMO_NAME);
        }
        localStorage.setItem(NAME_KEY, window.DEMO_NAME);
      },
      logout() { setToken(null); localStorage.removeItem(EMAIL_KEY); localStorage.removeItem(NAME_KEY); },
    },

    plans: {
      list: (page = 1, size = 50) => request('/v0/plans' + qs({ page, size })),
      create: (name, description, status = 'active') =>
        request('/v0/plans', { method: 'POST', body: { name, description, status } }),
    },

    goals: {
      list: (planId, page = 1, size = 100) =>
        request('/v0/plans/' + planId + '/goals' + qs({ page, size })),
      create: (planId, name, description, sortOrder = 0) =>
        request('/v0/plans/' + planId + '/goals', {
          method: 'POST', body: { name, description, sort_order: sortOrder },
        }),
    },

    items: {
      list: (planId, goalId, page = 1, size = 100) =>
        request('/v0/plans/' + planId + '/goals/' + goalId + '/items' + qs({ page, size })),
      create: (planId, goalId, body) =>
        request('/v0/plans/' + planId + '/goals/' + goalId + '/items', { method: 'POST', body }),
      update: (planId, goalId, itemId, patch) =>
        request('/v0/plans/' + planId + '/goals/' + goalId + '/items/' + itemId, {
          method: 'PATCH', body: patch,
        }),
      analytics: (itemId) => request('/v0/items/' + itemId + '/analytics'),
      analyze: (itemId, message, model) =>
        request('/v0/items/' + itemId + '/analyze', { method: 'POST', body: { message, model } }),
    },

    documents: {
      list: ({ planItemId, page = 1, size = 50 } = {}) =>
        request('/v0/documents' + qs({ plan_item_id: planItemId, page, size })),
      get: (id) => request('/v0/documents/' + id),
      // POST /v0/documents/upload/{id_plan_item} — multipart, поле "file".
      upload: (planItemId, file) => {
        const fd = new FormData();
        fd.append('file', file);
        return request('/v0/documents/upload/' + planItemId, { method: 'POST', raw: fd });
      },
      confirm: (id) => request('/v0/documents/' + id + '/confirm', { method: 'POST' }),
      reject: (id) => request('/v0/documents/' + id + '/reject', { method: 'POST' }),
      reanalyze: (id) => request('/v0/documents/' + id + '/reanalyze', { method: 'POST' }),
      // Скачивание требует Bearer-заголовок, поэтому тянем blob и отдаём object URL.
      async downloadBlob(id) {
        const tok = getToken();
        const res = await fetch(BASE + '/v0/documents/' + id + '/download', {
          headers: tok ? { Authorization: 'Bearer ' + tok } : {},
        });
        if (res.status === 401) { setToken(null); window.dispatchEvent(new Event('api:unauthorized')); throw new AuthError('Сессия истекла'); }
        if (!res.ok) throw new ApiError('Не удалось скачать файл', null, res.status);
        return res.blob();
      },
    },
  };
})();
