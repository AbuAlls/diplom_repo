/* screens-misc.jsx — Планы, Настройки, Авторизация */
const { useState, useEffect } = React;

/* ---------------- Планы (реальные: план → цель → позиция) ---------------- */
const itemPct = (it) => (it.progress_percent != null ? Math.round(it.progress_percent) : 0);
const avg = (arr) => (arr.length ? Math.round(arr.reduce((a, b) => a + b, 0) / arr.length) : 0);

// Загружает полное дерево планов с вычисленным прогрессом.
async function loadPlanTree() {
  const plans = (await API.plans.list()).items || [];
  const tree = [];
  for (const plan of plans) {
    const goals = (await API.goals.list(plan.id)).items || [];
    const gNodes = [];
    for (const goal of goals) {
      const items = (await API.items.list(plan.id, goal.id)).items || [];
      gNodes.push({ goal, items, progress: avg(items.map(itemPct)) });
    }
    tree.push({ plan, goals: gNodes, progress: avg(gNodes.map((g) => g.progress)) });
  }
  return tree;
}

function ItemRow({ planId, goalId, item }) {
  const [open, setOpen] = useState(false);
  const [data, setData] = useState(null);
  const [rec, setRec] = useState(null);
  const [busy, setBusy] = useState(false);
  const pct = itemPct(item);

  const toggle = async () => {
    const next = !open; setOpen(next);
    if (next && !data) {
      try { setData(await API.items.analytics(item.id)); } catch (e) { setData({ error: e.message }); }
    }
  };
  const analyze = async () => {
    if (busy) return;
    setBusy(true); setRec(null);
    try {
      const res = await API.items.analyze(item.id, 'Дай рекомендации по этой позиции плана');
      setRec(res.recommendations || 'Нет рекомендаций.');
    } catch (e) { setRec('Ошибка: ' + e.message); }
    finally { setBusy(false); }
  };

  return (
    <div style={{ borderTop: '1px solid var(--line)' }}>
      <div className="lrow" style={{ alignItems: 'center', borderTop: 'none', cursor: 'pointer' }} onClick={toggle}>
        <span className="icon-tile sm" style={{ width: 32, height: 32, borderRadius: 9 }}><Icon name="target" s={16} sw={2.4} /></span>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontWeight: 700 }}>{item.name}</div>
          <div style={{ fontSize: 12.5, color: 'var(--muted)' }}>
            {item.current_value != null ? item.current_value : '—'}{item.target_value != null ? ` / ${item.target_value}` : ''}{item.unit ? ' ' + item.unit : ''}
          </div>
        </div>
        <div style={{ width: 120 }} className="hide-sm"><Progress pct={pct} color={pct >= 80 ? 'var(--green)' : 'var(--accent-2)'} h={7} /></div>
        <Badge tone={pct >= 80 ? 'green' : 'ghost'}>{pct}%</Badge>
        <Icon name={open ? 'chevL' : 'chevR'} s={16} style={{ color: 'var(--muted-2)' }} />
      </div>

      {open && (
        <div style={{ padding: '4px 0 16px 44px' }} className="stack">
          {!data && <div style={{ color: 'var(--muted)' }}>Загрузка аналитики…</div>}
          {data && data.error && <div style={{ color: 'var(--red)' }}>{data.error}</div>}
          {data && !data.error && (
            <div style={{ fontSize: 13.5, color: 'var(--ink-2)' }}>
              Документов-источников: <b>{data.source_documents_count}</b>
              {data.notes && data.notes.length > 0 && (
                <ul style={{ margin: '6px 0 0', paddingLeft: 18 }}>{data.notes.map((n, i) => <li key={i}>{n}</li>)}</ul>
              )}
            </div>
          )}
          <Btn variant="soft" size="sm" icon="bolt" onClick={analyze} style={{ alignSelf: 'flex-start', opacity: busy ? 0.6 : 1 }}>{busy ? 'Анализ…' : 'Проанализировать (ИИ)'}</Btn>
          {rec && (
            <Card style={{ borderLeft: '4px solid var(--accent-2)' }}>
              <p style={{ fontSize: 14, color: 'var(--ink-2)', whiteSpace: 'pre-wrap', margin: 0 }}>{rec}</p>
            </Card>
          )}
        </div>
      )}
    </div>
  );
}

function PlansScreen() {
  const [tree, setTree] = useState(null); // null = загрузка
  const [error, setError] = useState(null);
  const [open, setOpen] = useState(null); // id выбранного плана

  const reload = () => {
    setTree(null); setError(null);
    loadPlanTree()
      .then((t) => { setTree(t); setOpen((cur) => cur || (t[0] && t[0].plan.id)); })
      .catch((e) => { setError(e.message); setTree([]); });
  };
  useEffect(reload, []);

  const newPlan = async () => {
    const name = window.prompt('Название плана');
    if (!name) return;
    try { await API.plans.create(name.trim()); reload(); } catch (e) { alert(e.message); }
  };
  const newGoal = async (planId) => {
    const name = window.prompt('Название цели');
    if (!name) return;
    try { await API.goals.create(planId, name.trim()); reload(); } catch (e) { alert(e.message); }
  };
  const newItem = async (planId, goalId) => {
    const name = window.prompt('Название позиции');
    if (!name) return;
    const target = window.prompt('Целевое значение (необязательно)');
    const unit = window.prompt('Единица измерения (необязательно)');
    const body = { name: name.trim() };
    if (target && !isNaN(parseFloat(target))) body.target_value = parseFloat(target);
    if (unit) body.unit = unit.trim();
    try { await API.items.create(planId, goalId, body); reload(); } catch (e) { alert(e.message); }
  };

  const sel = tree && tree.find((x) => x.plan.id === open);

  return (
    <div className="view stack">
      <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
        <div>
          <h1 className="page-title" style={{ fontSize: 27 }}>Планы</h1>
          <div className="page-sub">{tree ? `${tree.length} планов` : 'Загрузка…'}</div>
        </div>
        <Btn icon="plus" style={{ marginLeft: 'auto' }} onClick={newPlan}>Новый план</Btn>
      </div>

      {error && <div style={{ color: 'var(--red)', fontWeight: 600 }}>{error}</div>}
      {tree === null && <Placeholder label="загрузка планов…" h={120} />}
      {tree && tree.length === 0 && !error && <Placeholder label="нет планов — создайте первый" h={120} />}

      {tree && tree.length > 0 && (
        <div className="grid" style={{ gridTemplateColumns: 'repeat(3,1fr)' }}>
          {tree.map(({ plan, progress, goals }) => (
            <Card key={plan.id} tap style={{ outline: open === plan.id ? '2px solid color-mix(in srgb, var(--accent-2) 50%, transparent)' : 'none' }} onClick={() => setOpen(plan.id)}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <IconTile icon="target" tint="indigo" />
                <Ring pct={progress} size={58} color={progress >= 80 ? 'var(--green)' : 'var(--indigo)'}>
                  <span className="stat-num" style={{ fontSize: 14 }}>{progress}%</span>
                </Ring>
              </div>
              <h3 className="display" style={{ fontSize: 18, fontWeight: 800, marginTop: 14, textWrap: 'pretty' }}>{plan.name}</h3>
              <div style={{ display: 'flex', gap: 8, marginTop: 10 }}>
                <Badge tone="ghost">{goals.length} целей</Badge>
                <Badge tone={progress >= 80 ? 'green' : 'indigo'}>{plan.status}</Badge>
              </div>
            </Card>
          ))}
        </div>
      )}

      {sel && (
        <Card className="view" key={sel.plan.id}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
            <IconTile icon="target" tint="indigo" size="lg" />
            <div style={{ flex: 1 }}>
              <h2 className="display" style={{ fontSize: 22, fontWeight: 800 }}>{sel.plan.name}</h2>
              <div style={{ color: 'var(--muted)', marginTop: 3 }}>{sel.goals.length} целей · прогресс {sel.progress}%</div>
            </div>
            <Btn variant="soft" size="sm" icon="plus" onClick={() => newGoal(sel.plan.id)}>Цель</Btn>
          </div>
          <div style={{ marginTop: 14 }} className="stack">
            {sel.goals.length === 0 && <div style={{ color: 'var(--muted)' }}>Нет целей. Добавьте первую.</div>}
            {sel.goals.map(({ goal, items, progress }) => (
              <div key={goal.id}>
                <div className="lrow" style={{ alignItems: 'center' }}>
                  <span className={`icon-tile sm ${progress >= 100 ? 'green' : ''}`} style={{ width: 34, height: 34, borderRadius: 10 }}>
                    <Icon name={progress >= 100 ? 'check' : 'target'} s={17} sw={2.4} />
                  </span>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontWeight: 700 }}>{goal.name}</div>
                    <div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{items.length} позиций · прогресс {progress}%</div>
                  </div>
                  <Btn variant="ghost" size="sm" icon="plus" onClick={() => newItem(sel.plan.id, goal.id)}>Позиция</Btn>
                </div>
                <div style={{ marginLeft: 8 }}>
                  {items.map((it) => <ItemRow key={it.id} planId={sel.plan.id} goalId={goal.id} item={it} />)}
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}
    </div>
  );
}

/* ---------------- Настройки ---------------- */
function SettingsScreen({ theme, setTheme, onLogout }) {
  const [notif, setNotif] = useState(true);
  const [auto, setAuto] = useState(true);
  const Toggle = ({ on, onClick }) => (
    <button onClick={onClick} style={{ width: 46, height: 27, borderRadius: 999, border: 'none', cursor: 'pointer', padding: 3,
      background: on ? 'var(--brand-grad)' : 'var(--line-2)', transition: '.2s', display: 'flex', justifyContent: on ? 'flex-end' : 'flex-start' }}>
      <span style={{ width: 21, height: 21, borderRadius: '50%', background: 'white', boxShadow: '0 2px 4px rgba(0,0,0,.2)', transition: '.2s' }} />
    </button>
  );
  return (
    <div className="view stack" style={{ maxWidth: 760 }}>
      <h1 className="page-title" style={{ fontSize: 27 }}>Настройки</h1>

      {(() => { const u = API.currentUser(); return (
      <Card>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <div className="avatar" style={{ width: 64, height: 64, borderRadius: 18, fontSize: 23 }}>{u.initials}</div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div className="display" style={{ fontSize: 20, fontWeight: 800 }}>{u.name}</div>
            <div style={{ color: 'var(--muted)' }}>{u.email}{u.demo ? ' · ' + window.DEMO_ORG : ''}</div>
          </div>
          <Btn variant="ghost" size="sm" icon="user">Профиль</Btn>
        </div>
        {u.demo && (
          <div className="grid" style={{ gridTemplateColumns: 'repeat(3,1fr)', gap: 12, marginTop: 18 }}>
            {[['15', 'Документов'], ['3', 'Плана'], ['Pro', 'Тариф']].map(([n, l]) => (
              <div key={l} style={{ background: 'var(--surface-2)', borderRadius: 'var(--r-md)', padding: '14px', textAlign: 'center' }}>
                <div className="stat-num" style={{ fontSize: 22, color: 'var(--brand-ink)' }}>{n}</div>
                <div style={{ fontSize: 12.5, color: 'var(--muted)', fontWeight: 600 }}>{l}</div>
              </div>
            ))}
          </div>
        )}
      </Card>
      ); })()}

      <Card>
        <SectionLabel>Внешний вид</SectionLabel>
        <div className="lrow" style={{ borderTop: 'none', paddingTop: 16 }}>
          <span className="icon-tile sm" style={{ width: 38, height: 38 }}><Icon name="moon" s={19} /></span>
          <div style={{ flex: 1 }}>
            <div style={{ fontWeight: 700 }}>Тёмная тема</div>
            <div style={{ fontSize: 12.5, color: 'var(--muted)' }}>Переключить оформление интерфейса</div>
          </div>
          <div className="seg">
            <button className={theme === 'light' ? 'on' : ''} onClick={() => setTheme('light')}>Светлая</button>
            <button className={theme === 'dark' ? 'on' : ''} onClick={() => setTheme('dark')}>Тёмная</button>
          </div>
        </div>
      </Card>

      <Card>
        <SectionLabel>Предпочтения</SectionLabel>
        <div style={{ marginTop: 6 }}>
          {[
            { icon: 'bell', t: 'Уведомления об инсайтах', s: 'Срочные рекомендации и риски', on: notif, set: () => setNotif(!notif) },
            { icon: 'scan', t: 'Авто-анализ при загрузке', s: 'Запускать ИИ сразу после загрузки', on: auto, set: () => setAuto(!auto) },
          ].map((r) => (
            <div className="lrow" key={r.t}>
              <span className="icon-tile sm" style={{ width: 38, height: 38 }}><Icon name={r.icon} s={19} /></span>
              <div style={{ flex: 1 }}>
                <div style={{ fontWeight: 700 }}>{r.t}</div>
                <div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{r.s}</div>
              </div>
              <Toggle on={r.on} onClick={r.set} />
            </div>
          ))}
        </div>
      </Card>

      <Card style={{ display: 'flex', alignItems: 'center', gap: 14, background: 'var(--brand-grad)', border: 'none', color: 'white', position: 'relative', overflow: 'hidden' }}>
        <div className="blob" style={{ position: 'absolute', width: 160, height: 160, borderRadius: '50%', background: 'rgba(255,255,255,.12)', top: -80, right: -20 }} />
        <div className="frost" style={{ position: 'relative' }}><Icon name="sparkles" s={26} /></div>
        <div style={{ position: 'relative', flex: 1 }}>
          <div className="display" style={{ fontSize: 18, fontWeight: 800 }}>Тариф Pro</div>
          <div style={{ color: 'var(--on-grad)', fontSize: 13.5 }}>Безлимитный ИИ-анализ и расширенная аналитика</div>
        </div>
        <button className="btn" style={{ position: 'relative', background: 'white', color: 'var(--brand-ink)' }}>Управление</button>
      </Card>

      <button className="btn btn-ghost" style={{ alignSelf: 'flex-start', color: 'var(--red)' }} onClick={onLogout}><Icon name="logout" s={18} /> Выйти из аккаунта</button>
    </div>
  );
}

/* ---------------- Авторизация ---------------- */
function AuthScreen({ onLogin }) {
  const [mode, setMode] = useState('login'); // login | register
  const [email, setEmail] = useState('');
  const [pwd, setPwd] = useState('');
  const [name, setName] = useState('');
  const [showPwd, setShowPwd] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);
  const isReg = mode === 'register';

  const doDemo = async () => {
    if (busy) return;
    setError(null); setBusy(true);
    try { await API.auth.demo(); onLogin(); }
    catch (e) { setError(e.message || 'Не удалось открыть демо'); }
    finally { setBusy(false); }
  };

  const submit = async () => {
    if (busy) return;
    setError(null);
    setBusy(true);
    try {
      if (isReg) await API.auth.register(email, pwd, name);
      else await API.auth.login(email, pwd);
      onLogin();
    } catch (e) {
      setError(e.message || 'Не удалось войти');
    } finally {
      setBusy(false);
    }
  };
  const onKey = (e) => { if (e.key === 'Enter') submit(); };
  return (
    <div style={{ minHeight: '100vh', display: 'grid', gridTemplateColumns: '1.05fr 1fr', background: 'var(--bg)' }}>
      {/* brand panel */}
      <div className="auth-brand" style={{ background: 'var(--brand-grad)', color: 'white', padding: 56, position: 'relative', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
        <div className="blob" style={{ position: 'absolute', width: 360, height: 360, borderRadius: '50%', background: 'rgba(255,255,255,.10)', top: -120, right: -80 }} />
        <div className="blob" style={{ position: 'absolute', width: 240, height: 240, borderRadius: '50%', background: 'rgba(255,255,255,.08)', bottom: -90, left: -40 }} />
        <div style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 13 }}>
          <div className="frost" style={{ width: 48, height: 48, borderRadius: 14 }}><Icon name="bulb" s={24} /></div>
          <div style={{ fontFamily: 'Manrope', fontWeight: 800, fontSize: 18 }}>ИИ-Консультант</div>
        </div>
        <div style={{ position: 'relative', marginTop: 'auto' }}>
          <h1 className="display" style={{ fontSize: 42, fontWeight: 800, letterSpacing: '-.03em', lineHeight: 1.08, textWrap: 'balance' }}>
            Ваш бизнес под<br />контролем ИИ
          </h1>
          <p style={{ color: 'var(--on-grad)', fontSize: 16, marginTop: 16, maxWidth: 380, textWrap: 'pretty' }}>
            Загружайте документы, получайте инсайты и управляйте рисками — всё в одном месте.
          </p>
          <div style={{ display: 'flex', gap: 26, marginTop: 36 }}>
            {[['15', 'документов'], ['80%', 'проанализир.'], ['4', 'инсайта']].map(([n, l]) => (
              <div key={l}>
                <div className="display" style={{ fontSize: 26, fontWeight: 800 }}>{n}</div>
                <div style={{ fontSize: 12.5, color: 'var(--on-grad)' }}>{l}</div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* form */}
      <div style={{ display: 'grid', placeItems: 'center', padding: 40 }}>
        <div style={{ width: 'min(380px, 100%)' }}>
          <h2 className="display" style={{ fontSize: 28, fontWeight: 800, letterSpacing: '-.02em' }}>{isReg ? 'Создать аккаунт' : 'С возвращением'}</h2>
          <p style={{ color: 'var(--muted)', marginTop: 6 }}>{isReg ? 'Зарегистрируйтесь, чтобы начать' : 'Войдите, чтобы продолжить работу'}</p>
          <div className="stack" style={{ gap: 14, marginTop: 26 }}>
            {isReg && (
              <label className="field">
                <span>Имя</span>
                <div className="field-in"><Icon name="user" s={18} style={{ color: 'var(--muted)' }} /><input value={name} onChange={(e) => setName(e.target.value)} onKeyDown={onKey} placeholder="Иван Иванов" /></div>
              </label>
            )}
            <label className="field">
              <span>Email</span>
              <div className="field-in"><Icon name="user" s={18} style={{ color: 'var(--muted)' }} /><input type="email" value={email} onChange={(e) => setEmail(e.target.value)} onKeyDown={onKey} placeholder="you@example.com" /></div>
            </label>
            <label className="field">
              <span>Пароль</span>
              <div className="field-in">
                <Icon name="shield" s={18} style={{ color: 'var(--muted)' }} />
                <input type={showPwd ? 'text' : 'password'} value={pwd} onChange={(e) => setPwd(e.target.value)} onKeyDown={onKey} />
                <button type="button" onClick={() => setShowPwd((v) => !v)} title={showPwd ? 'Скрыть пароль' : 'Показать пароль'}
                  style={{ marginLeft: 'auto', background: 'none', border: 'none', cursor: 'pointer', padding: 0, display: 'flex', color: showPwd ? 'var(--brand-ink)' : 'var(--muted-2)' }}>
                  <Icon name="eye" s={18} />
                </button>
              </div>
            </label>
            {error && (
              <div style={{ background: 'color-mix(in srgb, var(--red) 12%, transparent)', color: 'var(--red)', borderRadius: 'var(--r-md)', padding: '10px 13px', fontSize: 13.5, fontWeight: 600 }}>{error}</div>
            )}
            <Btn onClick={submit} iconR="arrowR" style={{ marginTop: 4, padding: '14px', opacity: busy ? 0.7 : 1, pointerEvents: busy ? 'none' : 'auto' }}>{busy ? 'Подождите…' : isReg ? 'Создать' : 'Войти'}</Btn>
            <button className="btn btn-ghost" onClick={doDemo} style={{ justifyContent: 'center', pointerEvents: busy ? 'none' : 'auto' }}>
              <Icon name="sparkles" s={17} /> Посмотреть демо
            </button>
            <div style={{ textAlign: 'center', fontSize: 13.5, color: 'var(--muted)', marginTop: 4 }}>
              {isReg ? 'Уже есть аккаунт? ' : 'Нет аккаунта? '}
              <a onClick={() => { setMode(isReg ? 'login' : 'register'); setError(null); }} style={{ color: 'var(--brand-ink)', fontWeight: 700, cursor: 'pointer' }}>{isReg ? 'Войти' : 'Создать'}</a>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { PlansScreen, SettingsScreen, AuthScreen });
