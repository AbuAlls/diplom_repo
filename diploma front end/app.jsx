/* app.jsx — shell: sidebar, topbar, router, tweaks */
const { useState: useS, useEffect: useE } = React;

const ACCENTS = {
  violet:  ['#7C3AED', '#9333EA', '#C026D3'],
  indigo:  ['#4F46E5', '#6366F1', '#8B5CF6'],
  blue:    ['#2563EB', '#3B82F6', '#06B6D4'],
  emerald: ['#0D9488', '#10B981', '#22C55E'],
  sunset:  ['#DB2777', '#E11D48', '#F97316'],
};

const TWEAK_DEFAULTS = /*EDITMODE-BEGIN*/{
  "theme": "light",
  "accent": "violet",
  "density": "comfortable"
}/*EDITMODE-END*/;

function Sidebar({ route, go, open, setOpen }) {
  return (
    <>
      {open && <div className="scrim" onClick={() => setOpen(false)} />}
      <aside className={`sidebar ${open ? 'open' : ''}`}>
        <div className="brand">
          <div className="brand-mark"><Icon name="bulb" s={22} /></div>
          <div>
            <div className="brand-name">ИИ-Консультант</div>
            <div className="brand-sub">Premium</div>
          </div>
        </div>
        <div className="nav-label">Меню</div>
        {NAV.map((n) => (
          <button key={n.key} className={`nav-item ${route === n.key || (route === 'docdetail' && n.key === 'documents') ? 'active' : ''}`}
            onClick={() => { go(n.key); setOpen(false); }}>
            <span className="nav-ico"><Icon name={n.icon} s={20} /></span>
            {n.label}
            {n.dot && <span className="badge amber" style={{ marginLeft: 'auto', padding: '3px 9px', fontSize: 11.5 }}>{n.dot}</span>}
          </button>
        ))}
        <div className="side-user" onClick={() => { go('settings'); setOpen(false); }}>
          <Avatar name="АК" />
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontWeight: 700, fontSize: 14, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>Алексей Климов</div>
            <div style={{ fontSize: 12, color: 'var(--muted)' }}>ООО «Орбита»</div>
          </div>
          <Icon name="gear" s={17} style={{ color: 'var(--muted)' }} />
        </div>
      </aside>
    </>
  );
}

const TITLES = {
  overview: ['Обзор', 'Главная панель бизнеса'],
  insights: ['Инсайты', 'ИИ-рекомендации'],
  documents: ['Документы', 'Хранилище и анализ'],
  docdetail: ['Документ', 'Результаты анализа'],
  plans: ['Планы', 'Цели и задачи'],
  settings: ['Настройки', 'Профиль и предпочтения'],
};

function App() {
  const [t, setTweak] = useTweaks(TWEAK_DEFAULTS);
  const [authed, setAuthed] = useS(false);
  const [route, setRoute] = useS('overview');
  const [menuOpen, setMenuOpen] = useS(false);
  const [upload, setUpload] = useS(false);
  const [doc, setDoc] = useS(null);

  const theme = t.theme;
  const setTheme = (v) => setTweak('theme', v);

  // apply theme tokens
  useE(() => {
    const r = document.documentElement;
    r.setAttribute('data-theme', t.theme);
    r.setAttribute('data-density', t.density);
    const a = ACCENTS[t.accent] || ACCENTS.violet;
    r.style.setProperty('--accent-1', a[0]);
    r.style.setProperty('--accent-2', a[1]);
    r.style.setProperty('--accent-3', a[2]);
  }, [t.theme, t.accent, t.density]);

  const go = (r) => { setDoc(null); setRoute(r); window.scrollTo({ top: 0 }); };
  const openDoc = (d) => { setDoc(d); setRoute('docdetail'); window.scrollTo({ top: 0 }); };

  const panel = (
    <TweaksPanel>
      <TweakSection label="Тема" />
      <TweakRadio label="Оформление" value={t.theme} options={['light', 'dark']} onChange={(v) => setTweak('theme', v)} />
      <TweakSection label="Акцент" />
      <TweakColor label="Цвет бренда" value={ACCENTS[t.accent][1]}
        options={Object.values(ACCENTS).map((a) => a[1])}
        onChange={(v) => { const key = Object.keys(ACCENTS).find((k) => ACCENTS[k][1] === v) || 'violet'; setTweak('accent', key); }} />
      <TweakSection label="Плотность" />
      <TweakRadio label="Интерфейс" value={t.density} options={['comfortable', 'compact']} onChange={(v) => setTweak('density', v)} />
    </TweaksPanel>
  );

  if (!authed) {
    return <><AuthScreen onLogin={() => setAuthed(true)} />{panel}</>;
  }

  const [title, sub] = TITLES[route];

  return (
    <div className="app-shell">
      <Sidebar route={route} go={go} open={menuOpen} setOpen={setMenuOpen} />
      <div className="main">
        <header className="topbar">
          <button className="btn icon btn-ghost menu-btn" onClick={() => setMenuOpen(true)}><Icon name="menu" s={20} /></button>
          <div>
            <div className="page-title">{title}</div>
            <div className="page-sub hide-sm">{sub}</div>
          </div>
          <div className="search hide-sm" style={{ marginLeft: 'auto' }}>
            <Icon name="search" s={18} />
            <input placeholder="Поиск документов, инсайтов…" />
          </div>
          <button className="btn icon btn-ghost" style={{ position: 'relative' }}>
            <Icon name="bell" s={19} />
            <span style={{ position: 'absolute', top: 7, right: 7, width: 8, height: 8, borderRadius: '50%', background: 'var(--red)', border: '2px solid var(--surface)' }} />
          </button>
          <Btn icon="upload" className="hide-sm" onClick={() => setUpload(true)}>Загрузить</Btn>
        </header>

        <div className="content scroll">
          {route === 'overview' && <OverviewScreen go={go} />}
          {route === 'insights' && <InsightsScreen />}
          {route === 'documents' && <DocumentsScreen openUpload={() => setUpload(true)} openDoc={openDoc} />}
          {route === 'docdetail' && doc && <DocumentDetail doc={doc} onBack={() => go('documents')} />}
          {route === 'plans' && <PlansScreen />}
          {route === 'settings' && <SettingsScreen theme={theme} setTheme={setTheme} />}
        </div>
      </div>

      {upload && <UploadModal onClose={() => setUpload(false)} />}
      {panel}
    </div>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(<App />);
