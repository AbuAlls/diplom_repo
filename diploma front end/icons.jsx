/* icons.jsx — simple line-icon set (Lucide-style single/double path) */
const Svg = ({ s = 22, sw = 2, children, ...p }) => (
  <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke="currentColor"
       strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round" {...p}>{children}</svg>
);

const I = {
  grid: (p) => <Svg {...p}><rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/></Svg>,
  bulb: (p) => <Svg {...p}><path d="M9 18h6"/><path d="M10 22h4"/><path d="M12 2a7 7 0 0 0-4 12.7c.6.5 1 1.3 1 2.1v.2h6v-.2c0-.8.4-1.6 1-2.1A7 7 0 0 0 12 2Z"/></Svg>,
  doc: (p) => <Svg {...p}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Z"/><path d="M14 2v6h6"/><path d="M8 13h8M8 17h6"/></Svg>,
  folder: (p) => <Svg {...p}><path d="M4 5a2 2 0 0 1 2-2h3.2a2 2 0 0 1 1.6.8L12 5.5h6a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H4Z"/></Svg>,
  target: (p) => <Svg {...p}><circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="5"/><circle cx="12" cy="12" r="1"/></Svg>,
  gear: (p) => <Svg {...p}><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.6 1.6 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.6 1.6 0 0 0-2.7 1.1V21a2 2 0 0 1-4 0v-.1a1.6 1.6 0 0 0-2.7-1.1l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1A1.6 1.6 0 0 0 4.6 15H4.5a2 2 0 0 1 0-4h.1a1.6 1.6 0 0 0 1.1-2.7l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.6 1.6 0 0 0 2.7-1.1V4.5a2 2 0 0 1 4 0v.1a1.6 1.6 0 0 0 2.7 1.1l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.6 1.6 0 0 0-.3 1.8 1.6 1.6 0 0 0 1.5 1h.1a2 2 0 0 1 0 4h-.1a1.6 1.6 0 0 0-1.5 1Z"/></Svg>,
  search: (p) => <Svg {...p}><circle cx="11" cy="11" r="7"/><path d="m21 21-3.5-3.5"/></Svg>,
  bolt: (p) => <Svg {...p}><path d="M13 2 4.5 13.5a.7.7 0 0 0 .5 1.1H11l-1 7.4 8.5-11.5a.7.7 0 0 0-.5-1.1H12Z"/></Svg>,
  clock: (p) => <Svg {...p}><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.5 2"/></Svg>,
  shield: (p) => <Svg {...p}><path d="M12 2 4 5v6c0 5 3.4 8.5 8 10 4.6-1.5 8-5 8-10V5Z"/></Svg>,
  download: (p) => <Svg {...p}><path d="M12 3v12m0 0 4.5-4.5M12 15l-4.5-4.5"/><path d="M4 20h16"/></Svg>,
  upload: (p) => <Svg {...p}><path d="M12 16V4m0 0L7.5 8.5M12 4l4.5 4.5"/><path d="M4 20h16"/></Svg>,
  plus: (p) => <Svg {...p}><path d="M12 5v14M5 12h14"/></Svg>,
  chevR: (p) => <Svg {...p}><path d="m9 6 6 6-6 6"/></Svg>,
  chevL: (p) => <Svg {...p}><path d="m15 6-6 6 6 6"/></Svg>,
  arrowR: (p) => <Svg {...p}><path d="M4 12h16m0 0-6-6m6 6-6 6"/></Svg>,
  check: (p) => <Svg {...p}><path d="M20 6 9 17l-5-5"/></Svg>,
  checkCircle: (p) => <Svg {...p}><circle cx="12" cy="12" r="9"/><path d="m8.5 12 2.5 2.5L16 9"/></Svg>,
  alert: (p) => <Svg {...p}><path d="M12 9v4m0 4h.01"/><path d="M10.3 3.9 2 18.4A2 2 0 0 0 3.7 21.5h16.6A2 2 0 0 0 22 18.4L13.7 3.9a2 2 0 0 0-3.4 0Z"/></Svg>,
  menu: (p) => <Svg {...p}><path d="M3 6h18M3 12h18M3 18h18"/></Svg>,
  x: (p) => <Svg {...p}><path d="M6 6l12 12M18 6 6 18"/></Svg>,
  bell: (p) => <Svg {...p}><path d="M18 8a6 6 0 1 0-12 0c0 7-3 8-3 8h18s-3-1-3-8"/><path d="M10.5 21a1.8 1.8 0 0 0 3 0"/></Svg>,
  filter: (p) => <Svg {...p}><path d="M3 5h18l-7 8v6l-4-2v-4Z"/></Svg>,
  dots: (p) => <Svg {...p}><circle cx="5" cy="12" r="1.4"/><circle cx="12" cy="12" r="1.4"/><circle cx="19" cy="12" r="1.4"/></Svg>,
  receipt: (p) => <Svg {...p}><path d="M5 3v18l2-1.3L9 21l2-1.3L13 21l2-1.3L17 21l2-1.3V3l-2 1.3L15 3l-2 1.3L11 3 9 4.3 7 3Z"/><path d="M8.5 8h7M8.5 12h7"/></Svg>,
  card: (p) => <Svg {...p}><rect x="3" y="5" width="18" height="14" rx="2.5"/><path d="M3 10h18"/></Svg>,
  user: (p) => <Svg {...p}><circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/></Svg>,
  logout: (p) => <Svg {...p}><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="M16 17l5-5-5-5M21 12H9"/></Svg>,
  eye: (p) => <Svg {...p}><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/></Svg>,
  sparkles: (p) => <Svg {...p}><path d="M12 3l1.7 4.6L18 9l-4.3 1.4L12 15l-1.7-4.6L6 9l4.3-1.4Z"/><path d="M19 14l.8 2.2L22 17l-2.2.8L19 20l-.8-2.2L16 17l2.2-.8Z"/></Svg>,
  trend: (p) => <Svg {...p}><path d="M3 17l6-6 4 4 7-7"/><path d="M17 7h4v4"/></Svg>,
  building: (p) => <Svg {...p}><rect x="4" y="3" width="16" height="18" rx="1.5"/><path d="M9 7h.01M14 7h.01M9 11h.01M14 11h.01M9 15h.01M14 15h.01M10 21v-3h4v3"/></Svg>,
  calendar: (p) => <Svg {...p}><rect x="3" y="4.5" width="18" height="17" rx="2.5"/><path d="M3 9h18M8 2.5v4M16 2.5v4"/></Svg>,
  pin: (p) => <Svg {...p}><path d="M12 21s7-5.5 7-11a7 7 0 1 0-14 0c0 5.5 7 11 7 11Z"/><circle cx="12" cy="10" r="2.5"/></Svg>,
  scan: (p) => <Svg {...p}><path d="M4 8V5.5A1.5 1.5 0 0 1 5.5 4H8M16 4h2.5A1.5 1.5 0 0 1 20 5.5V8M20 16v2.5a1.5 1.5 0 0 1-1.5 1.5H16M8 20H5.5A1.5 1.5 0 0 1 4 18.5V16M4 12h16"/></Svg>,
  moon: (p) => <Svg {...p}><path d="M21 12.8A8 8 0 1 1 11.2 3a6.2 6.2 0 0 0 9.8 9.8Z"/></Svg>,
};

window.Icon = ({ name, ...p }) => { const C = I[name] || I.doc; return <C {...p} />; };
window.I = I;
