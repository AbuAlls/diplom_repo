/* screens-docs.jsx — Документы (browser + upload + detail/analysis) */
const { useState } = React;

function UploadModal({ onClose }) {
  const [drag, setDrag] = useState(false);
  const [phase, setPhase] = useState('pick'); // pick | uploading | done
  const [pct, setPct] = useState(0);
  const start = () => {
    setPhase('uploading');
    let p = 0;
    const t = setInterval(() => {
      p += Math.random() * 22 + 8;
      if (p >= 100) { p = 100; clearInterval(t); setTimeout(() => setPhase('done'), 400); }
      setPct(Math.min(100, Math.round(p)));
    }, 260);
  };
  return (
    <Modal onClose={onClose}>
      <div style={{ padding: 26 }} className="stack">
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <h3 className="display" style={{ fontSize: 21, fontWeight: 800 }}>Загрузить документ</h3>
          <button className="btn icon btn-ghost" onClick={onClose} style={{ marginLeft: 'auto' }}><Icon name="x" s={18} /></button>
        </div>

        {phase === 'pick' && (
          <>
            <div className={`dropzone ${drag ? 'drag' : ''}`} onClick={start}
              onDragOver={(e) => { e.preventDefault(); setDrag(true); }}
              onDragLeave={() => setDrag(false)}
              onDrop={(e) => { e.preventDefault(); setDrag(false); start(); }}>
              <div className="icon-tile lg" style={{ margin: '0 auto 16px' }}><Icon name="upload" s={28} /></div>
              <div style={{ fontFamily: 'Manrope', fontWeight: 800, fontSize: 17 }}>Перетащите файл сюда</div>
              <div style={{ color: 'var(--muted)', marginTop: 5 }}>или нажмите, чтобы выбрать · PDF, DOCX, JPG до 25 МБ</div>
            </div>
            <div className="row" style={{ gap: 10 }}>
              {['Договор', 'Отчёт', 'Лицензия', 'Прочее'].map((c, i) => (
                <span key={c} className={`chip ${i ? '' : ''}`} style={i ? { background: 'var(--surface-inset)', color: 'var(--ink-2)' } : null}>{c}</span>
              ))}
            </div>
          </>
        )}

        {phase === 'uploading' && (
          <div className="stack" style={{ gap: 16, padding: '20px 0' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
              <IconTile icon="doc" tint="indigo" />
              <div style={{ flex: 1 }}>
                <div style={{ fontWeight: 700 }}>contract_472A.pdf</div>
                <div style={{ fontSize: 13, color: 'var(--muted)' }}>2.4 МБ · загрузка…</div>
              </div>
              <span className="stat-num" style={{ fontSize: 20, color: 'var(--brand-ink)' }}>{pct}%</span>
            </div>
            <Progress pct={pct} />
            <div style={{ fontSize: 13, color: 'var(--muted)', display: 'flex', alignItems: 'center', gap: 8 }}>
              <Icon name="scan" s={16} /> После загрузки документ будет отправлен на ИИ-анализ
            </div>
          </div>
        )}

        {phase === 'done' && (
          <div className="stack" style={{ gap: 16, padding: '14px 0', alignItems: 'center', textAlign: 'center' }}>
            <div className="icon-tile lg green" style={{ width: 72, height: 72 }}><Icon name="checkCircle" s={34} /></div>
            <div>
              <div className="display" style={{ fontSize: 19, fontWeight: 800 }}>Документ загружен</div>
              <div style={{ color: 'var(--muted)', marginTop: 4 }}>Статус: <b style={{ color: 'var(--indigo)' }}>загружен</b> → поставлен в очередь на анализ</div>
            </div>
            <Btn onClick={onClose} icon="check">Готово</Btn>
          </div>
        )}
      </div>
    </Modal>
  );
}

function DocRow({ d, onOpen }) {
  return (
    <div className="card tap flat" style={{ background: 'var(--surface)', boxShadow: 'var(--shadow-sm)', display: 'flex', alignItems: 'center', gap: 15 }} onClick={() => onOpen(d)}>
      <IconTile icon={d.type === 'Отчёт' ? 'receipt' : d.type === 'Лицензия' ? 'card' : 'doc'} tint={d.type === 'Отчёт' ? '' : d.type === 'Лицензия' ? 'green' : 'indigo'} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontWeight: 700, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{d.title}</div>
        <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>{d.type} · {d.org} · {d.date}</div>
      </div>
      <div className="hide-sm" style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 96 }}>
        {d.risk !== '—' && <span className="dot" style={{ width: 8, height: 8, borderRadius: '50%', background: d.riskColor }} />}
        <span style={{ fontSize: 13, fontWeight: 600, color: d.risk === '—' ? 'var(--muted-2)' : 'var(--ink-2)' }}>{d.risk === '—' ? '—' : d.risk + ' риск'}</span>
      </div>
      <div style={{ minWidth: 116 }}><StatusPill status={d.status} /></div>
      <Icon name="chevR" s={18} style={{ color: 'var(--muted-2)' }} />
    </div>
  );
}

function DocumentsScreen({ openUpload, openDoc }) {
  const [view, setView] = useState('all'); // all | folders
  const [active, setActive] = useState(null);
  const docs = active ? DOCS.filter((d) => d.folder === active) : DOCS;
  return (
    <div className="view stack">
      <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
        <div>
          <h1 className="page-title" style={{ fontSize: 27 }}>Документы</h1>
          <div className="page-sub">15 документов · 4 папки</div>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 10 }}>
          <div className="seg">
            <button className={view === 'all' ? 'on' : ''} onClick={() => { setView('all'); setActive(null); }}>Все</button>
            <button className={view === 'folders' ? 'on' : ''} onClick={() => setView('folders')}>Папки</button>
          </div>
          <Btn icon="upload" onClick={openUpload}>Загрузить</Btn>
        </div>
      </div>

      {view === 'folders' && !active && (
        <div className="grid" style={{ gridTemplateColumns: 'repeat(2,1fr)' }}>
          {FOLDERS.map((f) => (
            <Card key={f.id} tap style={{ display: 'flex', alignItems: 'center', gap: 16 }} onClick={() => setActive(f.name)}>
              <IconTile icon="folder" tint={f.tint} size="lg" />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontFamily: 'Manrope', fontWeight: 800, fontSize: 17 }}>{f.name}</div>
                <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 3 }}>{f.count} документов</div>
                {f.plan && <span className="badge ghost" style={{ marginTop: 10 }}><Icon name="target" s={13} />{f.plan}</span>}
              </div>
              <Icon name="chevR" s={20} style={{ color: 'var(--muted-2)' }} />
            </Card>
          ))}
        </div>
      )}

      {(view === 'all' || active) && (
        <>
          {active && (
            <button className="btn btn-ghost sm" style={{ alignSelf: 'flex-start' }} onClick={() => setActive(null)}>
              <Icon name="chevL" s={16} /> {active}
            </button>
          )}
          <div className="stack" style={{ gap: 12 }}>
            {docs.map((d) => <DocRow key={d.id} d={d} onOpen={openDoc} />)}
          </div>
        </>
      )}
    </div>
  );
}

/* -------- Document detail + analysis -------- */
function DocumentDetail({ doc, onBack }) {
  const a = ANALYSIS[doc.id] || ANALYSIS.d1;
  const s = STATUS[doc.status];
  return (
    <div className="view stack">
      <button className="btn btn-ghost sm" style={{ alignSelf: 'flex-start' }} onClick={onBack}>
        <Icon name="chevL" s={16} /> К документам
      </button>

      <div className="grid" style={{ gridTemplateColumns: '1.4fr 1fr', alignItems: 'start' }}>
        <div className="stack">
          {/* header card */}
          <Card>
            <div style={{ display: 'flex', gap: 18, alignItems: 'flex-start' }}>
              <IconTile icon={doc.type === 'Отчёт' ? 'receipt' : doc.type === 'Лицензия' ? 'card' : 'doc'} tint="indigo" size="lg" />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
                  <Badge tone={s.badge}>{s.label}</Badge>
                  {doc.risk !== '—' && <Badge tone={doc.risk === 'Высокий' ? 'red' : doc.risk === 'Средний' ? 'amber' : 'green'}>{doc.risk} риск</Badge>}
                </div>
                <h1 className="display" style={{ fontSize: 25, fontWeight: 800, letterSpacing: '-.02em', marginTop: 12, textWrap: 'pretty' }}>{doc.title}</h1>
                <div style={{ color: 'var(--muted)', marginTop: 6 }}>{doc.type} · {doc.pages} стр. · {doc.size}</div>
              </div>
            </div>
            <div className="grid" style={{ gridTemplateColumns: 'repeat(2,1fr)', gap: 12, marginTop: 18 }}>
              {[['Контрагент', doc.org], ['Дата', doc.date], ['Папка', doc.folder], ['Тип', doc.type]].map(([k, v]) => (
                <div key={k} style={{ background: 'var(--surface-2)', borderRadius: 'var(--r-md)', padding: '12px 15px' }}>
                  <div style={{ fontSize: 12, color: 'var(--muted)', fontWeight: 600 }}>{k}</div>
                  <div style={{ fontWeight: 700, marginTop: 3 }}>{v}</div>
                </div>
              ))}
            </div>
          </Card>

          {/* AI summary */}
          <Card style={{ borderLeft: '5px solid var(--accent-2)', paddingLeft: 'calc(var(--pad-card) - 1px)' }}>
            <span className="chip"><Icon name="bolt" s={14} sw={2.6} />ИИ-анализ</span>
            <p style={{ fontSize: 15.5, color: 'var(--ink-2)', marginTop: 13, textWrap: 'pretty' }}>{a.summary}</p>
          </Card>

          {/* extracted fields */}
          <Card>
            <SectionLabel>Извлечённые данные</SectionLabel>
            <div style={{ marginTop: 10 }}>
              {a.fields.map((f) => (
                <div className="lrow" key={f.k} style={{ padding: '13px 0' }}>
                  <span style={{ color: 'var(--muted)', fontWeight: 500 }}>{f.k}</span>
                  <span style={{ marginLeft: 'auto', fontWeight: 700 }}>{f.v}</span>
                </div>
              ))}
            </div>
          </Card>
        </div>

        {/* right column */}
        <div className="stack">
          <Card>
            <Placeholder label="предпросмотр PDF" h={210} />
            <div className="row" style={{ gap: 10, marginTop: 14 }}>
              <Btn icon="eye" className="" style={{ flex: 1 }}>Открыть</Btn>
              <Btn variant="ghost" icon="download" style={{ flex: 1 }}>Скачать</Btn>
            </div>
          </Card>

          <Card>
            <SectionLabel>Проверки аудита</SectionLabel>
            <div className="stack" style={{ gap: 11, marginTop: 13 }}>
              {a.checks.map((c, i) => (
                <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 11 }}>
                  <span className={`icon-tile sm ${c.ok ? 'green' : 'red'}`} style={{ width: 30, height: 30, borderRadius: 9 }}>
                    <Icon name={c.ok ? 'check' : 'x'} s={16} sw={2.6} />
                  </span>
                  <span style={{ fontWeight: 500, color: c.ok ? 'var(--ink)' : 'var(--ink-2)' }}>{c.label}</span>
                </div>
              ))}
            </div>
          </Card>

          <Card>
            <SectionLabel>Выявленные риски</SectionLabel>
            <div className="stack" style={{ gap: 12, marginTop: 13 }}>
              {a.risks.map((r, i) => (
                <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 11 }}>
                  <span className="dot" style={{ width: 9, height: 9, borderRadius: '50%', background: r.color }} />
                  <span style={{ fontWeight: 500 }}>{r.label}</span>
                  <span style={{ marginLeft: 'auto', fontWeight: 700, color: r.color, fontSize: 13 }}>{r.level}</span>
                </div>
              ))}
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { UploadModal, DocumentsScreen, DocumentDetail });
