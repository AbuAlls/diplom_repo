/* data.jsx — mock domain data for the AI Consultant web app (Russian UI) */

const DOC_TYPES = [
  { key: 'contract', label: 'Договоры', icon: 'doc', tint: 'indigo', count: 6, pct: 40 },
  { key: 'report',   label: 'Отчёты',   icon: 'receipt', tint: '', count: 4, pct: 27 },
  { key: 'license',  label: 'Лицензии', icon: 'card', tint: 'green', count: 3, pct: 20 },
  { key: 'other',    label: 'Прочие',   icon: 'doc', tint: 'amber', count: 2, pct: 13 },
];

const RISKS = [
  { label: 'Высокий', count: 3, color: 'var(--red)',   pct: 25 },
  { label: 'Средний', count: 5, color: 'var(--amber)', pct: 42 },
  { label: 'Низкий',  count: 7, color: 'var(--green)', pct: 70 },
];

const INSIGHTS = [
  {
    id: 'i1', kind: 'Возможность', kindClass: 'amber', icon: 'clock', tile: 'amber',
    accent: 'var(--amber)', tag: 'ИИ-рекомендация',
    title: 'Оптимизация затрат на поставки',
    body: 'Анализ договоров поставки показал возможность снижения расходов на 8% при консолидации заказов.',
    meta: 'Договоры · 3 документа', impact: 'Экономия ≈ 240 000 ₽ / квартал',
  },
  {
    id: 'i2', kind: 'Срочно', kindClass: 'red', icon: 'alert', tile: 'red',
    accent: 'var(--red)', tag: 'Требует внимания',
    title: 'Истекает срок лицензии',
    body: 'Лицензия на ПО истекает через 14 дней. Рекомендуется продлить, чтобы избежать простоя.',
    meta: 'Лицензии · 1 документ', impact: 'Дедлайн: 14 июня 2026',
  },
  {
    id: 'i3', kind: 'Важно', kindClass: 'amber', icon: 'shield', tile: 'indigo',
    accent: 'var(--indigo)', tag: 'Аудит',
    title: 'Несоответствие в отчётности',
    body: 'В квартальном отчёте обнаружены расхождения по статье «Прочие расходы». Требуется проверка.',
    meta: 'Отчёты · 2 документа', impact: 'Риск: средний',
  },
  {
    id: 'i4', kind: 'К сведению', kindClass: 'indigo', icon: 'trend', tile: 'green',
    accent: 'var(--green)', tag: 'Аналитика',
    title: 'Рост выручки по сегменту B2B',
    body: 'По данным за квартал сегмент B2B вырос на 12%. Стоит усилить направление в плане развития.',
    meta: 'Отчёты · 4 документа', impact: 'Тренд: положительный',
  },
];

const FOLDERS = [
  { id: 'f1', name: 'Договоры 2026', icon: 'folder', count: 6, tint: 'indigo', plan: 'Снижение издержек' },
  { id: 'f2', name: 'Финансовая отчётность', icon: 'folder', count: 4, tint: '', plan: 'Рост выручки' },
  { id: 'f3', name: 'Лицензии и разрешения', icon: 'folder', count: 3, tint: 'green', plan: null },
  { id: 'f4', name: 'Кадровые документы', icon: 'folder', count: 5, tint: 'amber', plan: null },
];

const STATUS = {
  // Бэкенд-статусы документа (см. domain.Document.Status).
  uploaded:       { label: 'Загружен',   color: 'var(--indigo)', badge: 'indigo' },
  processing:     { label: 'Обработка',  color: 'var(--amber)',  badge: 'amber' },
  pending_review: { label: 'На проверке', color: 'var(--amber)',  badge: 'amber' },
  confirmed:      { label: 'Подтверждён', color: 'var(--green)',  badge: 'green' },
  rejected:       { label: 'Отклонён',   color: 'var(--red)',    badge: 'red' },
  failed:         { label: 'Ошибка',     color: 'var(--red)',    badge: 'red' },
  // Старый мок-статус (используется FOLDERS/мок-документами).
  processed:      { label: 'Обработан',  color: 'var(--green)',  badge: 'green' },
};

const DOCS = [
  { id: 'd1', title: 'Договор поставки №472-А', type: 'Договор', folder: 'Договоры 2026', status: 'processed', risk: 'Средний', riskColor: 'var(--amber)', date: '12 мая 2026', org: 'ООО «Орбита»', size: '2.4 МБ', pages: 14 },
  { id: 'd2', title: 'Квартальный отчёт Q1 2026', type: 'Отчёт', folder: 'Финансовая отчётность', status: 'processed', risk: 'Высокий', riskColor: 'var(--red)', date: '8 мая 2026', org: 'Внутренний', size: '5.1 МБ', pages: 32 },
  { id: 'd3', title: 'Лицензия на ПО (Enterprise)', type: 'Лицензия', folder: 'Лицензии и разрешения', status: 'processing', risk: '—', riskColor: 'var(--muted)', date: '14 мая 2026', org: 'SoftLine', size: '0.8 МБ', pages: 4 },
  { id: 'd4', title: 'Акт сверки взаиморасчётов', type: 'Договор', folder: 'Договоры 2026', status: 'uploaded', risk: '—', riskColor: 'var(--muted)', date: '15 мая 2026', org: 'ООО «Орбита»', size: '1.1 МБ', pages: 6 },
  { id: 'd5', title: 'Трудовой договор №118', type: 'Прочее', folder: 'Кадровые документы', status: 'processed', risk: 'Низкий', riskColor: 'var(--green)', date: '2 мая 2026', org: 'Внутренний', size: '0.6 МБ', pages: 8 },
  { id: 'd6', title: 'Отчёт о движении средств', type: 'Отчёт', folder: 'Финансовая отчётность', status: 'failed', risk: '—', riskColor: 'var(--muted)', date: '11 мая 2026', org: 'Внутренний', size: '3.2 МБ', pages: 20 },
];

// extracted analysis for detail screen (keyed by doc id; default fallback)
const ANALYSIS = {
  d1: {
    summary: 'Договор поставки на 12 месяцев с автоматической пролонгацией. Выявлен пункт о фиксированной цене без индексации — возможность экономии при пересмотре.',
    fields: [
      { k: 'Контрагент', v: 'ООО «Орбита»' },
      { k: 'Сумма договора', v: '3 200 000 ₽' },
      { k: 'Срок действия', v: '01.05.2026 – 30.04.2027' },
      { k: 'Условия оплаты', v: 'Постоплата 30 дней' },
      { k: 'Штрафные санкции', v: '0,1% за день просрочки' },
    ],
    checks: [
      { label: 'Реквизиты сторон корректны', ok: true },
      { label: 'Срок действия указан', ok: true },
      { label: 'Отсутствует пункт об индексации цены', ok: false },
      { label: 'Условия расторжения определены', ok: true },
    ],
    risks: [
      { label: 'Ценовой риск (нет индексации)', level: 'Средний', color: 'var(--amber)' },
      { label: 'Риск автопролонгации', level: 'Низкий', color: 'var(--green)' },
    ],
  },
};

const PLANS = [
  {
    id: 'p1', title: 'Снижение операционных издержек', tint: 'indigo', icon: 'trend',
    progress: 64, status: 'В работе', period: 'Q2 2026',
    goals: [
      { id: 'g1', title: 'Пересмотр договоров поставки', done: true, items: 4, doneItems: 4 },
      { id: 'g2', title: 'Консолидация заказов', done: false, items: 5, doneItems: 3 },
      { id: 'g3', title: 'Аудит лицензий', done: false, items: 3, doneItems: 1 },
    ],
  },
  {
    id: 'p2', title: 'Рост выручки B2B', tint: 'green', icon: 'target',
    progress: 38, status: 'В работе', period: 'H1 2026',
    goals: [
      { id: 'g4', title: 'Анализ ключевых клиентов', done: true, items: 3, doneItems: 3 },
      { id: 'g5', title: 'Запуск партнёрской программы', done: false, items: 6, doneItems: 1 },
    ],
  },
  {
    id: 'p3', title: 'Соответствие требованиям', tint: 'amber', icon: 'shield',
    progress: 80, status: 'Почти готово', period: 'Q2 2026',
    goals: [
      { id: 'g6', title: 'Обновление лицензий', done: false, items: 2, doneItems: 1 },
      { id: 'g7', title: 'Проверка отчётности', done: true, items: 4, doneItems: 4 },
    ],
  },
];

const NAV = [
  { key: 'overview',  label: 'Обзор',     icon: 'grid' },
  { key: 'insights',  label: 'Инсайты',   icon: 'bulb', dot: 2 },
  { key: 'documents', label: 'Документы', icon: 'doc' },
  { key: 'plans',     label: 'Планы',     icon: 'target' },
  { key: 'settings',  label: 'Настройки', icon: 'gear' },
];

Object.assign(window, { DOC_TYPES, RISKS, INSIGHTS, FOLDERS, DOCS, STATUS, ANALYSIS, PLANS, NAV });
