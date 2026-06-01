/* screens-home.jsx — Обзор (Overview) + Инсайты (Insights) */
const { useState, useEffect } = React;

function Hero({ icon, title, sub, children }) {
  return (
    <div className="hero">
      <div className="blob a" /><div className="blob b" />
      <div style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 20 }}>
        <div className="frost"><Icon name={icon} s={30} sw={2} /></div>
        <div>
          <h2 className="display" style={{ fontSize: 30, fontWeight: 800, letterSpacing: '-.02em' }}>{title}</h2>
          <div style={{ fontSize: 15, color: 'var(--on-grad)', fontWeight: 500, marginTop: 2 }}>{sub}</div>
        </div>
        <div style={{ marginLeft: 'auto' }}>{children}</div>
      </div>
    </div>
  );
}

/* ---------------- Обзор ---------------- */
// Демо-аккаунт видит витринный пресет; остальные — свою реальную (или пустую) картину.
function OverviewScreen({ go }) {
  return API.isDemo() ? <DemoOverview go={go} /> : <RealOverview go={go} />;
}

function StatCard({ value, label, hint, hintColor }) {
  return (
    <Card>
      <div className="stat-num" style={{ fontSize: 40 }}>{value}</div>
      <div style={{ fontWeight: 600, marginTop: 4, color: 'var(--ink-2)' }}>{label}</div>
      {hint && <div className="status" style={{ color: hintColor || 'var(--muted)', marginTop: 18 }}>{hint}</div>}
    </Card>
  );
}

// Реальный обзор: KPI считаются из данных текущего аккаунта; пусто = нули.
function RealOverview({ go }) {
  const [stats, setStats] = useState(null);
  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const docs = await API.documents.list({ size: 100 });
        const plans = await API.plans.list();
        const items = (docs.items || []);
        const analyzed = items.filter((d) => ['pending_review', 'confirmed'].includes(d.status)).length;
        if (alive) setStats({ docs: docs.meta ? docs.meta.total : items.length, analyzed, plans: plans.meta ? plans.meta.total : (plans.items || []).length });
      } catch (e) {
        if (alive) setStats({ docs: 0, analyzed: 0, plans: 0, error: e.message });
      }
    })();
    return () => { alive = false; };
  }, []);

  const s = stats || { docs: '—', analyzed: '—', plans: '—' };
  const empty = stats && stats.docs === 0 && stats.plans === 0;

  return (
    <div className="view stack">
      <Hero icon="grid" title="Обзор" sub="Общая картина вашего бизнеса" />

      <div className="grid" style={{ gridTemplateColumns: 'repeat(4, 1fr)' }}>
        <Card className="kpi-grad" style={{ background: 'var(--brand-grad)', color: 'white', border: 'none', position: 'relative', overflow: 'hidden' }}>
          <div className="blob" style={{ position: 'absolute', width: 130, height: 130, borderRadius: '50%', background: 'rgba(255,255,255,.12)', top: -50, right: -30 }} />
          <div style={{ position: 'relative' }}>
            <div className="stat-num" style={{ fontSize: 40 }}>{s.docs}</div>
            <div style={{ fontWeight: 600, marginTop: 4 }}>Всего документов</div>
          </div>
        </Card>
        <StatCard value={s.analyzed} label="Проанализировано" hint={<><Icon name="checkCircle" s={17} /> готово</>} hintColor="var(--green)" />
        <StatCard value={s.plans} label="Планов" />
        <StatCard value={0} label="Новых инсайта" />
      </div>

      {empty && (
        <Card style={{ textAlign: 'center', padding: '34px 22px' }}>
          <div className="icon-tile lg" style={{ margin: '0 auto 14px' }}><Icon name="upload" s={28} /></div>
          <div className="display" style={{ fontSize: 19, fontWeight: 800 }}>Здесь пока пусто</div>
          <div style={{ color: 'var(--muted)', marginTop: 6 }}>Создайте план и загрузите документ — данные появятся автоматически.</div>
        </Card>
      )}

      <Card>
        <SectionLabel>Рекомендуемые действия</SectionLabel>
        <div className="grid" style={{ gridTemplateColumns: 'repeat(3,1fr)', marginTop: 14 }}>
          {[
            { icon: 'target', tint: 'green', t: 'Создать план', s: 'Задайте цели и позиции', go: 'plans' },
            { icon: 'upload', tint: 'indigo', t: 'Загрузить документ', s: 'Добавьте файл для анализа', go: 'documents' },
            { icon: 'bulb', tint: 'amber', t: 'Открыть инсайты', s: 'ИИ-рекомендации', go: 'insights' },
          ].map((a) => (
            <div key={a.t} className="card tap flat" style={{ background: 'var(--surface-2)', display: 'flex', alignItems: 'center', gap: 14 }} onClick={() => go(a.go)}>
              <IconTile icon={a.icon} tint={a.tint} size="sm" />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontWeight: 700, fontSize: 14.5 }}>{a.t}</div>
                <div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{a.s}</div>
              </div>
              <Icon name="chevR" s={18} style={{ color: 'var(--muted-2)' }} />
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}

function DemoOverview({ go }) {
  const overall = { color: 'var(--amber)', label: 'Требует внимания' };
  return (
    <div className="view stack">
      <Hero icon="grid" title="Обзор" sub="Общая картина вашего бизнеса">
        <div className="hide-sm"><Ring pct={80} size={92} color="rgba(255,255,255,.95)" track="rgba(255,255,255,.25)">
          <div><div style={{ fontFamily: 'Manrope', fontWeight: 800, fontSize: 20, color: 'var(--ink)' }}>80%</div>
          <div style={{ fontSize: 10.5, color: 'var(--muted)', fontWeight: 600 }}>готово</div></div>
        </Ring></div>
      </Hero>

      {/* KPI cards */}
      <div className="grid" style={{ gridTemplateColumns: 'repeat(4, 1fr)' }}>
        <Card className="kpi-grad" style={{ background: 'var(--brand-grad)', color: 'white', border: 'none', position: 'relative', overflow: 'hidden' }}>
          <div className="blob" style={{ position: 'absolute', width: 130, height: 130, borderRadius: '50%', background: 'rgba(255,255,255,.12)', top: -50, right: -30 }} />
          <div style={{ position: 'relative' }}>
            <div className="stat-num" style={{ fontSize: 40 }}>15</div>
            <div style={{ fontWeight: 600, marginTop: 4 }}>Всего документов</div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 18, fontSize: 13, fontWeight: 600, color: 'var(--on-grad)' }}>
              <Icon name="arrowR" s={16} /> +3 в этом месяце
            </div>
          </div>
        </Card>
        <Card>
          <div className="stat-num" style={{ fontSize: 40 }}>12</div>
          <div style={{ fontWeight: 600, marginTop: 4, color: 'var(--ink-2)' }}>Проанализировано</div>
          <div className="status" style={{ color: 'var(--green)', marginTop: 18 }}><Icon name="checkCircle" s={17} /> 80% готово</div>
        </Card>
        <Card>
          <IconTile icon="bulb" tint="amber" size="sm" />
          <div className="stat-num" style={{ fontSize: 34, marginTop: 12 }}>4</div>
          <div style={{ fontWeight: 600, color: 'var(--ink-2)' }}>Новых инсайта</div>
        </Card>
        <Card>
          <IconTile icon="shield" tint="red" size="sm" />
          <div className="stat-num" style={{ fontSize: 34, marginTop: 12 }}>3</div>
          <div style={{ fontWeight: 600, color: 'var(--ink-2)' }}>Высоких риска</div>
        </Card>
      </div>

      <div className="grid" style={{ gridTemplateColumns: '1.3fr 1fr', alignItems: 'start' }}>
        {/* doc types */}
        <Card>
          <SectionLabel>Типы документов</SectionLabel>
          <div style={{ marginTop: 14 }}>
            {DOC_TYPES.map((t) => (
              <div className="lrow" key={t.key}>
                <IconTile icon={t.icon} tint={t.tint} size="sm" />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontWeight: 700 }}>{t.label}</div>
                  <div style={{ fontSize: 13, color: 'var(--muted)' }}>{t.count} документа</div>
                </div>
                <div style={{ width: 120 }} className="hide-sm"><Progress pct={t.pct} h={7} /></div>
                <div className="stat-num" style={{ fontSize: 22, width: 54, textAlign: 'right' }}>{t.pct}%</div>
              </div>
            ))}
          </div>
        </Card>

        {/* risks */}
        <Card>
          <SectionLabel>Уровень рисков</SectionLabel>
          <div className="stack" style={{ gap: 16, marginTop: 16 }}>
            {RISKS.map((r) => (
              <div key={r.label}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                  <span style={{ fontFamily: 'Manrope', fontWeight: 800, fontSize: 17 }}>{r.label}</span>
                  <span className="stat-num" style={{ fontSize: 19, color: r.color }}>{r.count}</span>
                </div>
                <Progress pct={r.pct} color={r.color} />
              </div>
            ))}
          </div>
          <div style={{ borderTop: '1px solid var(--line)', marginTop: 18, paddingTop: 16, display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ color: 'var(--muted)', fontWeight: 500 }}>Общий статус</span>
            <span className="dot" style={{ width: 9, height: 9, borderRadius: '50%', background: overall.color }} />
            <span style={{ fontFamily: 'Manrope', fontWeight: 800, marginLeft: 'auto', color: 'var(--ink)' }}>{overall.label}</span>
          </div>
        </Card>
      </div>

      {/* recommended actions */}
      <Card>
        <SectionLabel>Рекомендуемые действия</SectionLabel>
        <div className="grid" style={{ gridTemplateColumns: 'repeat(3,1fr)', marginTop: 14 }}>
          {[
            { icon: 'upload', tint: 'indigo', t: 'Загрузить новый документ', s: 'Добавьте файл для анализа', go: 'documents' },
            { icon: 'bulb', tint: 'amber', t: 'Решить срочные вопросы (2)', s: 'Просмотрите инсайты', go: 'insights' },
            { icon: 'target', tint: 'green', t: 'Обновить планы', s: '3 активных плана', go: 'plans' },
          ].map((a) => (
            <div key={a.t} className="card tap flat" style={{ background: 'var(--surface-2)', display: 'flex', alignItems: 'center', gap: 14 }} onClick={() => go(a.go)}>
              <IconTile icon={a.icon} tint={a.tint} size="sm" />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontWeight: 700, fontSize: 14.5 }}>{a.t}</div>
                <div style={{ fontSize: 12.5, color: 'var(--muted)' }}>{a.s}</div>
              </div>
              <Icon name="chevR" s={18} style={{ color: 'var(--muted-2)' }} />
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}

/* ---------------- Инсайты ---------------- */
function InsightCard({ ins, onOpen }) {
  return (
    <Card tap className="view" style={{ borderLeft: `5px solid ${ins.accent}`, paddingLeft: 'calc(var(--pad-card) - 1px)' }} onClick={() => onOpen(ins)}>
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}>
        <IconTile icon={ins.icon} tint={ins.tile} />
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
            <span className="chip"><Icon name="bolt" s={14} sw={2.6} />{ins.tag}</span>
            <Badge tone={ins.kindClass}>{ins.kind}</Badge>
          </div>
          <h3 className="display" style={{ fontSize: 21, fontWeight: 800, letterSpacing: '-.02em', marginTop: 13, textWrap: 'pretty' }}>{ins.title}</h3>
          <p style={{ color: 'var(--ink-2)', marginTop: 7, maxWidth: 620, textWrap: 'pretty' }}>{ins.body}</p>
          <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginTop: 16, flexWrap: 'wrap' }}>
            <span style={{ fontSize: 12.5, color: 'var(--muted)', fontWeight: 600 }}>{ins.meta}</span>
            <span style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 7, color: ins.accent, fontWeight: 700, fontSize: 13 }}>
              <Icon name="sparkles" s={16} />{ins.impact}
            </span>
          </div>
        </div>
      </div>
    </Card>
  );
}

function InsightsScreen() {
  return API.isDemo() ? <DemoInsights /> : <EmptyInsights />;
}

// Реальные аккаунты: ленты инсайтов на бэкенде нет — показываем пустое состояние.
// (ИИ-рекомендации доступны точечно на странице «Планы» по каждой позиции.)
function EmptyInsights() {
  return (
    <div className="view stack">
      <Hero icon="bulb" title="Инсайты" sub="ИИ-рекомендации для вашего бизнеса" />
      <Card style={{ textAlign: 'center', padding: '40px 22px' }}>
        <div className="icon-tile lg amber" style={{ margin: '0 auto 14px' }}><Icon name="bulb" s={28} /></div>
        <div className="display" style={{ fontSize: 19, fontWeight: 800 }}>Инсайтов пока нет</div>
        <div style={{ color: 'var(--muted)', marginTop: 6, maxWidth: 420, marginInline: 'auto' }}>
          Загрузите документы и запустите ИИ-анализ по позициям плана — рекомендации появятся здесь.
        </div>
      </Card>
    </div>
  );
}

function DemoInsights() {
  const [open, setOpen] = useState(null);
  const [filter, setFilter] = useState('Все');
  const filters = ['Все', 'Срочно', 'Важно', 'Возможность'];
  const list = filter === 'Все' ? INSIGHTS : INSIGHTS.filter((i) => i.kind === filter);
  return (
    <div className="view stack">
      <Hero icon="bulb" title="Инсайты" sub="ИИ-рекомендации для вашего бизнеса" />
      <div className="grid" style={{ gridTemplateColumns: 'repeat(3,1fr)' }}>
        {[{ n: 2, l: 'Срочно', c: 'var(--red)' }, { n: 2, l: 'Важно', c: 'var(--amber)' }, { n: 1, l: 'К сведению', c: 'var(--indigo)' }].map((s) => (
          <Card key={s.l} style={{ textAlign: 'center' }}>
            <div className="stat-num" style={{ fontSize: 34, color: s.c }}>{s.n}</div>
            <div style={{ fontWeight: 600, color: 'var(--ink-2)', marginTop: 4 }}>{s.l}</div>
          </Card>
        ))}
      </div>
      <div className="seg" style={{ alignSelf: 'flex-start' }}>
        {filters.map((f) => <button key={f} className={filter === f ? 'on' : ''} onClick={() => setFilter(f)}>{f}</button>)}
      </div>
      <div className="stack">
        {list.map((ins) => <InsightCard key={ins.id} ins={ins} onOpen={setOpen} />)}
      </div>

      {open && (
        <Modal onClose={() => setOpen(null)} wide>
          <div style={{ background: 'var(--brand-grad)', color: 'white', padding: '24px 26px', borderRadius: 'var(--r-xl) var(--r-xl) 0 0', position: 'relative', overflow: 'hidden' }}>
            <div className="blob" style={{ position: 'absolute', width: 160, height: 160, borderRadius: '50%', background: 'rgba(255,255,255,.12)', top: -70, right: -30 }} />
            <div style={{ position: 'relative', display: 'flex', gap: 16, alignItems: 'center' }}>
              <div className="frost"><Icon name={open.icon} s={28} /></div>
              <div><Badge tone="ghost">{open.kind}</Badge><h3 className="display" style={{ fontSize: 23, fontWeight: 800, marginTop: 8 }}>{open.title}</h3></div>
              <button className="btn icon" onClick={() => setOpen(null)} style={{ marginLeft: 'auto', background: 'rgba(255,255,255,.18)', color: 'white' }}><Icon name="x" s={18} /></button>
            </div>
          </div>
          <div style={{ padding: 26 }} className="stack">
            <p style={{ fontSize: 15.5, color: 'var(--ink-2)', textWrap: 'pretty' }}>{open.body}</p>
            <div className="card flat" style={{ background: 'var(--brand-soft)', display: 'flex', alignItems: 'center', gap: 12, color: 'var(--brand-ink)' }}>
              <Icon name="sparkles" s={20} /><span style={{ fontWeight: 700 }}>{open.impact}</span>
            </div>
            <div>
              <SectionLabel>Что предлагает ИИ</SectionLabel>
              <div className="stack" style={{ gap: 10, marginTop: 12 }}>
                {['Проанализировать связанные документы', 'Подготовить сводку для согласования', 'Назначить ответственного и срок'].map((s, i) => (
                  <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <span className="icon-tile sm" style={{ width: 30, height: 30, borderRadius: 9 }}><Icon name="check" s={16} /></span>
                    <span style={{ fontWeight: 500 }}>{s}</span>
                  </div>
                ))}
              </div>
            </div>
            <div className="row" style={{ gap: 12 }}>
              <Btn icon="check">Принять рекомендацию</Btn>
              <Btn variant="ghost" onClick={() => setOpen(null)}>Отложить</Btn>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}

Object.assign(window, { OverviewScreen, InsightsScreen, Hero });
