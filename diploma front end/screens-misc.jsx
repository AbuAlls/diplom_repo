/* screens-misc.jsx — Планы, Настройки, Авторизация */
const { useState } = React;

/* ---------------- Планы ---------------- */
function PlansScreen() {
  const [open, setOpen] = useState(PLANS[0].id);
  return (
    <div className="view stack">
      <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
        <div>
          <h1 className="page-title" style={{ fontSize: 27 }}>Планы</h1>
          <div className="page-sub">3 активных плана · 7 целей</div>
        </div>
        <Btn icon="plus" style={{ marginLeft: 'auto' }}>Новый план</Btn>
      </div>

      <div className="grid" style={{ gridTemplateColumns: 'repeat(3,1fr)' }}>
        {PLANS.map((p) => (
          <Card key={p.id} tap style={{ outline: open === p.id ? '2px solid color-mix(in srgb, var(--accent-2) 50%, transparent)' : 'none' }} onClick={() => setOpen(p.id)}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
              <IconTile icon={p.icon} tint={p.tint} />
              <Ring pct={p.progress} size={58} color={p.tint === 'green' ? 'var(--green)' : p.tint === 'amber' ? 'var(--amber)' : 'var(--indigo)'}>
                <span className="stat-num" style={{ fontSize: 14 }}>{p.progress}%</span>
              </Ring>
            </div>
            <h3 className="display" style={{ fontSize: 18, fontWeight: 800, marginTop: 14, textWrap: 'pretty' }}>{p.title}</h3>
            <div style={{ display: 'flex', gap: 8, marginTop: 10 }}>
              <Badge tone="ghost"><Icon name="calendar" s={13} />{p.period}</Badge>
              <Badge tone={p.progress >= 80 ? 'green' : 'indigo'}>{p.status}</Badge>
            </div>
          </Card>
        ))}
      </div>

      {(() => {
        const p = PLANS.find((x) => x.id === open);
        return (
          <Card className="view" key={p.id}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
              <IconTile icon={p.icon} tint={p.tint} size="lg" />
              <div style={{ flex: 1 }}>
                <h2 className="display" style={{ fontSize: 22, fontWeight: 800 }}>{p.title}</h2>
                <div style={{ color: 'var(--muted)', marginTop: 3 }}>{p.goals.length} целей · период {p.period}</div>
              </div>
              <Btn variant="soft" size="sm" icon="plus">Цель</Btn>
            </div>
            <div style={{ marginTop: 8 }}>
              {p.goals.map((g) => (
                <div className="lrow" key={g.id} style={{ alignItems: 'center' }}>
                  <span className={`icon-tile sm ${g.done ? 'green' : ''}`} style={{ width: 34, height: 34, borderRadius: 10 }}>
                    <Icon name={g.done ? 'check' : 'target'} s={17} sw={2.4} />
                  </span>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontWeight: 700, textDecoration: g.done ? 'none' : 'none' }}>{g.title}</div>
                    <div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{g.doneItems} из {g.items} задач выполнено</div>
                  </div>
                  <div style={{ width: 130 }} className="hide-sm"><Progress pct={Math.round(g.doneItems / g.items * 100)} color={g.done ? 'var(--green)' : 'var(--accent-2)'} h={7} /></div>
                  {g.done
                    ? <Badge tone="green" icon="check">Готово</Badge>
                    : <Badge tone="ghost">{Math.round(g.doneItems / g.items * 100)}%</Badge>}
                </div>
              ))}
            </div>
          </Card>
        );
      })()}
    </div>
  );
}

/* ---------------- Настройки ---------------- */
function SettingsScreen({ theme, setTheme }) {
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

      <Card>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <div className="avatar" style={{ width: 64, height: 64, borderRadius: 18, fontSize: 23 }}>АК</div>
          <div style={{ flex: 1 }}>
            <div className="display" style={{ fontSize: 20, fontWeight: 800 }}>Алексей Климов</div>
            <div style={{ color: 'var(--muted)' }}>alexey@orbita.ru · ООО «Орбита»</div>
          </div>
          <Btn variant="ghost" size="sm" icon="user">Профиль</Btn>
        </div>
        <div className="grid" style={{ gridTemplateColumns: 'repeat(3,1fr)', gap: 12, marginTop: 18 }}>
          {[['15', 'Документов'], ['3', 'Плана'], ['Pro', 'Тариф']].map(([n, l]) => (
            <div key={l} style={{ background: 'var(--surface-2)', borderRadius: 'var(--r-md)', padding: '14px', textAlign: 'center' }}>
              <div className="stat-num" style={{ fontSize: 22, color: 'var(--brand-ink)' }}>{n}</div>
              <div style={{ fontSize: 12.5, color: 'var(--muted)', fontWeight: 600 }}>{l}</div>
            </div>
          ))}
        </div>
      </Card>

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

      <button className="btn btn-ghost" style={{ alignSelf: 'flex-start', color: 'var(--red)' }}><Icon name="logout" s={18} /> Выйти из аккаунта</button>
    </div>
  );
}

/* ---------------- Авторизация ---------------- */
function AuthScreen({ onLogin }) {
  const [email, setEmail] = useState('alexey@orbita.ru');
  const [pwd, setPwd] = useState('••••••••');
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
          <h2 className="display" style={{ fontSize: 28, fontWeight: 800, letterSpacing: '-.02em' }}>С возвращением</h2>
          <p style={{ color: 'var(--muted)', marginTop: 6 }}>Войдите, чтобы продолжить работу</p>
          <div className="stack" style={{ gap: 14, marginTop: 26 }}>
            <label className="field">
              <span>Email</span>
              <div className="field-in"><Icon name="user" s={18} style={{ color: 'var(--muted)' }} /><input value={email} onChange={(e) => setEmail(e.target.value)} /></div>
            </label>
            <label className="field">
              <span>Пароль</span>
              <div className="field-in"><Icon name="shield" s={18} style={{ color: 'var(--muted)' }} /><input type="password" value={pwd} onChange={(e) => setPwd(e.target.value)} /><Icon name="eye" s={18} style={{ color: 'var(--muted-2)', marginLeft: 'auto' }} /></div>
            </label>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: 13 }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'var(--ink-2)', cursor: 'pointer' }}>
                <span className="icon-tile sm" style={{ width: 22, height: 22, borderRadius: 7 }}><Icon name="check" s={14} sw={3} /></span> Запомнить
              </label>
              <a style={{ color: 'var(--brand-ink)', fontWeight: 600, textDecoration: 'none', cursor: 'pointer' }}>Забыли пароль?</a>
            </div>
            <Btn onClick={onLogin} iconR="arrowR" style={{ marginTop: 4, padding: '14px' }}>Войти</Btn>
            <div style={{ textAlign: 'center', fontSize: 13.5, color: 'var(--muted)', marginTop: 4 }}>
              Нет аккаунта? <a onClick={onLogin} style={{ color: 'var(--brand-ink)', fontWeight: 700, cursor: 'pointer' }}>Создать</a>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { PlansScreen, SettingsScreen, AuthScreen });
