/* screens-docs.jsx — Документы (живой список + загрузка + детальный анализ).
   Источник данных — реальный бэкенд через window.API. Папки и карточки
   «аудит/риски» остаются на мок-данных (бэкенд их не отдаёт). */
const { useState, useEffect } = React;

/* -------- helpers: маппинг documentResponse → отображение -------- */
function fmtSize(bytes) {
  if (bytes == null) return '—';
  if (bytes < 1024) return bytes + ' Б';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' КБ';
  return (bytes / 1024 / 1024).toFixed(1) + ' МБ';
}
function fmtDate(doc) {
  const raw = doc.document_date || doc.created_at;
  if (!raw) return '—';
  const d = new Date(raw);
  if (isNaN(d)) return raw;
  return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}
function docIcon(mime) {
  const m = (mime || '').toLowerCase();
  if (m.includes('image')) return 'scan';
  if (m.includes('sheet') || m.includes('excel') || m.includes('csv')) return 'receipt';
  return 'doc';
}
function typeLabel(mime) {
  const m = (mime || '').toLowerCase();
  if (m.includes('pdf')) return 'PDF';
  if (m.includes('image')) return 'Изображение';
  if (m.includes('word') || m.includes('document')) return 'Документ';
  if (m.includes('sheet') || m.includes('excel')) return 'Таблица';
  return m.split('/')[1] ? m.split('/')[1].toUpperCase() : 'Файл';
}

/* Загружает все плановые позиции пользователя (план → цель → позиция),
   чтобы выбрать, к чему привязать загружаемый документ. */
async function fetchAllItems() {
  const out = [];
  const plans = (await API.plans.list()).items || [];
  for (const p of plans) {
    const goals = (await API.goals.list(p.id)).items || [];
    for (const g of goals) {
      const items = (await API.items.list(p.id, g.id)).items || [];
      for (const it of items) {
        out.push({ id: it.id, label: `${p.name} · ${g.name} · ${it.name}` });
      }
    }
  }
  return out;
}

function UploadModal({ onClose, onUploaded }) {
  const [drag, setDrag] = useState(false);
  const [phase, setPhase] = useState('pick'); // pick | uploading | done | error
  const [file, setFile] = useState(null);
  const [items, setItems] = useState(null); // null = загрузка
  const [itemId, setItemId] = useState('');
  const [error, setError] = useState(null);
  const [result, setResult] = useState(null);

  useEffect(() => {
    fetchAllItems()
      .then((list) => { setItems(list); if (list[0]) setItemId(String(list[0].id)); })
      .catch((e) => { setItems([]); setError(e.message); });
  }, []);

  const pickFile = (f) => { if (f) setFile(f); };
  const start = async () => {
    if (!file || !itemId) return;
    setPhase('uploading');
    setError(null);
    try {
      const doc = await API.documents.upload(itemId, file);
      setResult(doc);
      setPhase('done');
      if (onUploaded) onUploaded(doc);
    } catch (e) {
      setError(e.message || 'Ошибка загрузки');
      setPhase('error');
    }
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
            <label className="field">
              <span>Привязать к плановой позиции</span>
              {items === null
                ? <div className="field-in" style={{ color: 'var(--muted)' }}>Загрузка позиций…</div>
                : items.length === 0
                  ? <div className="field-in" style={{ color: 'var(--red)' }}>Нет доступных позиций — создайте план, цель и позицию</div>
                  : (
                    <div className="field-in">
                      <Icon name="target" s={18} style={{ color: 'var(--muted)' }} />
                      <select value={itemId} onChange={(e) => setItemId(e.target.value)} style={{ flex: 1, border: 'none', background: 'transparent', font: 'inherit', color: 'inherit', outline: 'none' }}>
                        {items.map((it) => <option key={it.id} value={it.id}>{it.label}</option>)}
                      </select>
                    </div>
                  )}
            </label>

            <div className={`dropzone ${drag ? 'drag' : ''}`}
              onClick={() => document.getElementById('upload-input').click()}
              onDragOver={(e) => { e.preventDefault(); setDrag(true); }}
              onDragLeave={() => setDrag(false)}
              onDrop={(e) => { e.preventDefault(); setDrag(false); pickFile(e.dataTransfer.files[0]); }}>
              <input id="upload-input" type="file" style={{ display: 'none' }} onChange={(e) => pickFile(e.target.files[0])} />
              <div className="icon-tile lg" style={{ margin: '0 auto 16px' }}><Icon name="upload" s={28} /></div>
              <div style={{ fontFamily: 'Manrope', fontWeight: 800, fontSize: 17 }}>{file ? file.name : 'Перетащите файл сюда'}</div>
              <div style={{ color: 'var(--muted)', marginTop: 5 }}>{file ? fmtSize(file.size) : 'или нажмите, чтобы выбрать · PDF, DOCX, JPG до 32 МБ'}</div>
            </div>

            {error && <div style={{ color: 'var(--red)', fontSize: 13.5, fontWeight: 600 }}>{error}</div>}
            <Btn icon="upload" onClick={start} style={{ opacity: file && itemId ? 1 : 0.5, pointerEvents: file && itemId ? 'auto' : 'none' }}>Загрузить и проанализировать</Btn>
          </>
        )}

        {phase === 'uploading' && (
          <div className="stack" style={{ gap: 16, padding: '20px 0' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
              <IconTile icon="doc" tint="indigo" />
              <div style={{ flex: 1 }}>
                <div style={{ fontWeight: 700 }}>{file && file.name}</div>
                <div style={{ fontSize: 13, color: 'var(--muted)' }}>{file && fmtSize(file.size)} · загрузка…</div>
              </div>
            </div>
            <Progress pct={100} />
            <div style={{ fontSize: 13, color: 'var(--muted)', display: 'flex', alignItems: 'center', gap: 8 }}>
              <Icon name="scan" s={16} /> Документ загружается и отправляется на ИИ-анализ
            </div>
          </div>
        )}

        {phase === 'error' && (
          <div className="stack" style={{ gap: 16, padding: '14px 0', alignItems: 'center', textAlign: 'center' }}>
            <div className="icon-tile lg red" style={{ width: 72, height: 72 }}><Icon name="alert" s={34} /></div>
            <div>
              <div className="display" style={{ fontSize: 19, fontWeight: 800 }}>Не удалось загрузить</div>
              <div style={{ color: 'var(--red)', marginTop: 4 }}>{error}</div>
            </div>
            <Btn variant="ghost" onClick={() => setPhase('pick')}>Назад</Btn>
          </div>
        )}

        {phase === 'done' && (
          <div className="stack" style={{ gap: 16, padding: '14px 0', alignItems: 'center', textAlign: 'center' }}>
            <div className="icon-tile lg green" style={{ width: 72, height: 72 }}><Icon name="checkCircle" s={34} /></div>
            <div>
              <div className="display" style={{ fontSize: 19, fontWeight: 800 }}>Документ загружен</div>
              <div style={{ color: 'var(--muted)', marginTop: 4 }}>Статус: <b style={{ color: 'var(--indigo)' }}>{result && STATUS[result.status] ? STATUS[result.status].label : (result && result.status)}</b></div>
            </div>
            <Btn onClick={onClose} icon="check">Готово</Btn>
          </div>
        )}
      </div>
    </Modal>
  );
}

function DocRow({ d, onOpen }) {
  const s = STATUS[d.status] || { label: d.status, badge: 'indigo' };
  return (
    <div className="card tap flat" style={{ background: 'var(--surface)', boxShadow: 'var(--shadow-sm)', display: 'flex', alignItems: 'center', gap: 15 }} onClick={() => onOpen(d)}>
      <IconTile icon={docIcon(d.mime_type)} tint="indigo" />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontWeight: 700, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{d.title || d.file_name}</div>
        <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>{typeLabel(d.mime_type)} · {d.organization_name || '—'} · {fmtDate(d)}</div>
      </div>
      <div className="hide-sm" style={{ minWidth: 96, fontSize: 13, color: 'var(--muted-2)' }}>{fmtSize(d.file_size)}</div>
      <div style={{ minWidth: 116 }}><StatusPill status={d.status} /></div>
      <Icon name="chevR" s={18} style={{ color: 'var(--muted-2)' }} />
    </div>
  );
}

function DocumentsScreen({ openUpload, openDoc, refreshKey }) {
  const demo = API.isDemo(); // папки — витринная мок-функция, только для демо
  const [view, setView] = useState('all'); // all | folders
  const [active, setActive] = useState(null);
  const [docs, setDocs] = useState(null); // null = загрузка
  const [error, setError] = useState(null);

  useEffect(() => {
    let alive = true;
    setDocs(null); setError(null);
    API.documents.list()
      .then((res) => { if (alive) setDocs(res.items || []); })
      .catch((e) => { if (alive) { setError(e.message); setDocs([]); } });
    return () => { alive = false; };
  }, [refreshKey]);

  return (
    <div className="view stack">
      <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
        <div>
          <h1 className="page-title" style={{ fontSize: 27 }}>Документы</h1>
          <div className="page-sub">{docs ? `${docs.length} документов` : 'Загрузка…'}{demo ? ` · ${FOLDERS.length} папок` : ''}</div>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 10 }}>
          {demo && (
            <div className="seg">
              <button className={view === 'all' ? 'on' : ''} onClick={() => { setView('all'); setActive(null); }}>Все</button>
              <button className={view === 'folders' ? 'on' : ''} onClick={() => setView('folders')}>Папки</button>
            </div>
          )}
          <Btn icon="upload" onClick={openUpload}>Загрузить</Btn>
        </div>
      </div>

      {/* Папки — мок-данные, только для демо-аккаунта */}
      {demo && view === 'folders' && !active && (
        <div className="grid" style={{ gridTemplateColumns: 'repeat(2,1fr)' }}>
          {FOLDERS.map((f) => (
            <Card key={f.id} tap style={{ display: 'flex', alignItems: 'center', gap: 16 }} onClick={() => { setView('all'); setActive(null); }}>
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

      {view === 'all' && (
        <div className="stack" style={{ gap: 12 }}>
          {docs === null && <Placeholder label="загрузка документов…" h={120} />}
          {error && <div style={{ color: 'var(--red)', fontWeight: 600 }}>{error}</div>}
          {docs && docs.length === 0 && !error && <Placeholder label="нет документов — загрузите первый" h={120} />}
          {docs && docs.map((d) => <DocRow key={d.id} d={d} onOpen={openDoc} />)}
        </div>
      )}
    </div>
  );
}

/* -------- Document detail + analysis -------- */
function DocumentDetail({ doc, onBack }) {
  const [full, setFull] = useState(doc);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    API.documents.get(doc.id)
      .then((d) => { if (alive) { setFull(d); setError(null); } })
      .catch((e) => { if (alive) setError(e.message); })
      .finally(() => { if (alive) setLoading(false); });
    return () => { alive = false; };
  }, [doc.id]);

  const d = full;
  const s = STATUS[d.status] || { label: d.status, badge: 'indigo' };
  const demo = API.isDemo(); // аудит-проверки и риски — мок, только для демо
  const mock = ANALYSIS[doc.id] || ANALYSIS.d1;

  // Извлечённые данные из реальных полей recognition.
  const extracted = [
    ['Контрагент', d.organization_name],
    ['ИНН', d.inn],
    ['Внешний номер', d.external_number],
    ['Дата документа', d.document_date],
    ['Сроки', (d.deadlines || []).join(', ')],
    ['Суммы', (d.prices || []).join(', ')],
    ['Товары/услуги', (d.product_names || []).join(', ')],
    ['Номера договоров', (d.contract_numbers || []).join(', ')],
    ['Количества', (d.quantities || []).join(', ')],
  ].filter(([, v]) => v != null && v !== '');

  const download = async () => {
    try {
      const blob = await API.documents.downloadBlob(d.id);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url; a.download = d.file_name || 'document';
      document.body.appendChild(a); a.click(); a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) { alert(e.message); }
  };

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
              <IconTile icon={docIcon(d.mime_type)} tint="indigo" size="lg" />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
                  <Badge tone={s.badge}>{s.label}</Badge>
                </div>
                <h1 className="display" style={{ fontSize: 25, fontWeight: 800, letterSpacing: '-.02em', marginTop: 12, textWrap: 'pretty' }}>{d.title || d.file_name}</h1>
                <div style={{ color: 'var(--muted)', marginTop: 6 }}>{typeLabel(d.mime_type)} · {fmtSize(d.file_size)}</div>
              </div>
            </div>
            <div className="grid" style={{ gridTemplateColumns: 'repeat(2,1fr)', gap: 12, marginTop: 18 }}>
              {[['Контрагент', d.organization_name || '—'], ['Дата', fmtDate(d)], ['ИНН', d.inn || '—'], ['Тип', typeLabel(d.mime_type)]].map(([k, v]) => (
                <div key={k} style={{ background: 'var(--surface-2)', borderRadius: 'var(--r-md)', padding: '12px 15px' }}>
                  <div style={{ fontSize: 12, color: 'var(--muted)', fontWeight: 600 }}>{k}</div>
                  <div style={{ fontWeight: 700, marginTop: 3 }}>{v}</div>
                </div>
              ))}
            </div>
          </Card>

          {/* AI summary — реальный recognized_text */}
          <Card style={{ borderLeft: '5px solid var(--accent-2)', paddingLeft: 'calc(var(--pad-card) - 1px)' }}>
            <span className="chip"><Icon name="bolt" s={14} sw={2.6} />ИИ-анализ</span>
            <p style={{ fontSize: 15.5, color: 'var(--ink-2)', marginTop: 13, textWrap: 'pretty', whiteSpace: 'pre-wrap' }}>
              {error ? error : (d.recognized_text || (loading ? 'Загрузка…' : 'Текст ещё не распознан.'))}
            </p>
          </Card>

          {/* extracted fields — реальные */}
          <Card>
            <SectionLabel>Извлечённые данные</SectionLabel>
            <div style={{ marginTop: 10 }}>
              {extracted.length === 0 && <div style={{ color: 'var(--muted)', padding: '8px 0' }}>Нет извлечённых полей.</div>}
              {extracted.map(([k, v]) => (
                <div className="lrow" key={k} style={{ padding: '13px 0' }}>
                  <span style={{ color: 'var(--muted)', fontWeight: 500 }}>{k}</span>
                  <span style={{ marginLeft: 'auto', fontWeight: 700, textAlign: 'right' }}>{v}</span>
                </div>
              ))}
            </div>
          </Card>
        </div>

        {/* right column */}
        <div className="stack">
          <Card>
            <Placeholder label="предпросмотр" h={210} />
            <div className="row" style={{ gap: 10, marginTop: 14 }}>
              <Btn variant="ghost" icon="download" style={{ flex: 1 }} onClick={download}>Скачать</Btn>
            </div>
          </Card>

          {/* Проверки аудита и риски — мок, только для демо-аккаунта */}
          {demo && (
            <Card>
              <SectionLabel>Проверки аудита</SectionLabel>
              <div className="stack" style={{ gap: 11, marginTop: 13 }}>
                {mock.checks.map((c, i) => (
                  <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 11 }}>
                    <span className={`icon-tile sm ${c.ok ? 'green' : 'red'}`} style={{ width: 30, height: 30, borderRadius: 9 }}>
                      <Icon name={c.ok ? 'check' : 'x'} s={16} sw={2.6} />
                    </span>
                    <span style={{ fontWeight: 500, color: c.ok ? 'var(--ink)' : 'var(--ink-2)' }}>{c.label}</span>
                  </div>
                ))}
              </div>
            </Card>
          )}

          {demo && (
            <Card>
              <SectionLabel>Выявленные риски</SectionLabel>
              <div className="stack" style={{ gap: 12, marginTop: 13 }}>
                {mock.risks.map((r, i) => (
                  <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 11 }}>
                    <span className="dot" style={{ width: 9, height: 9, borderRadius: '50%', background: r.color }} />
                    <span style={{ fontWeight: 500 }}>{r.label}</span>
                    <span style={{ marginLeft: 'auto', fontWeight: 700, color: r.color, fontSize: 13 }}>{r.level}</span>
                  </div>
                ))}
              </div>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { UploadModal, DocumentsScreen, DocumentDetail });
