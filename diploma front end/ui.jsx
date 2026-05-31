/* ui.jsx — shared UI primitives */
const { useState, useEffect, useRef } = React;

const Card = ({ className = '', tap, children, ...p }) => (
  <div className={`card ${tap ? 'tap' : ''} ${className}`} {...p}>{children}</div>
);

const SectionLabel = ({ children, style }) => (
  <div className="section-label" style={style}>{children}</div>
);

const IconTile = ({ icon, tint = '', size = '', s }) => (
  <div className={`icon-tile ${size} ${tint}`}>
    <Icon name={icon} s={s || (size === 'lg' ? 26 : size === 'sm' ? 19 : 22)} sw={2} />
  </div>
);

const Badge = ({ children, tone = '', icon }) => (
  <span className={`badge ${tone}`}>{icon && <Icon name={icon} s={14} sw={2.4} />}{children}</span>
);

const Btn = ({ variant = 'primary', size = '', icon, iconR, children, className = '', ...p }) => (
  <button className={`btn btn-${variant} ${size} ${className}`} {...p}>
    {icon && <Icon name={icon} s={size === 'sm' ? 16 : 18} sw={2.2} />}
    {children}
    {iconR && <Icon name={iconR} s={size === 'sm' ? 16 : 18} sw={2.2} />}
  </button>
);

const Avatar = ({ name = 'АК', sm }) => (
  <div className={`avatar ${sm ? 'sm' : ''}`}>{name}</div>
);

const Progress = ({ pct, color = 'var(--accent-2)', h = 9 }) => (
  <div className="track" style={{ height: h }}>
    <div className="fill" style={{ width: `${pct}%`, background: color }} />
  </div>
);

const StatusPill = ({ status }) => {
  const s = STATUS[status];
  return <span className="status" style={{ color: s.color }}><span className="dot" style={{ background: s.color }} />{s.label}</span>;
};

// A ring / donut progress using conic-gradient
const Ring = ({ pct, size = 92, color = 'var(--accent-2)', track = 'var(--surface-inset)', children }) => (
  <div style={{ width: size, height: size, borderRadius: '50%', display: 'grid', placeItems: 'center',
    background: `conic-gradient(${color} ${pct * 3.6}deg, ${track} 0)` }}>
    <div style={{ width: size - 18, height: size - 18, borderRadius: '50%', background: 'var(--surface)',
      display: 'grid', placeItems: 'center', textAlign: 'center' }}>{children}</div>
  </div>
);

// striped image placeholder
const Placeholder = ({ label = 'превью документа', h = 180, r = 'var(--r-md)' }) => (
  <div style={{ height: h, borderRadius: r, display: 'grid', placeItems: 'center',
    background: 'repeating-linear-gradient(135deg, var(--surface-inset) 0 12px, var(--surface-2) 12px 24px)',
    border: '1px solid var(--line)', color: 'var(--muted)' }}>
    <span style={{ fontFamily: 'ui-monospace, monospace', fontSize: 12, letterSpacing: '.04em' }}>{label}</span>
  </div>
);

const Modal = ({ onClose, children, wide }) => {
  useEffect(() => {
    const h = (e) => e.key === 'Escape' && onClose();
    window.addEventListener('keydown', h);
    return () => window.removeEventListener('keydown', h);
  }, []);
  return (
    <div className="overlay" onClick={onClose}>
      <div className="modal" style={wide ? { width: 'min(720px, 100%)' } : null} onClick={(e) => e.stopPropagation()}>
        {children}
      </div>
    </div>
  );
};

Object.assign(window, { Card, SectionLabel, IconTile, Badge, Btn, Avatar, Progress, StatusPill, Ring, Placeholder, Modal });
