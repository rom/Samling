// Welcome tour ("show and tell") for first-time users. It starts by itself
// until the account has finished or skipped it once (settings.tour_done) and
// can be started again from the help page with startTour().
import { html, useState, useEffect, useRef, useStore, S, setState, t, api, Icon, SharedBadge } from '../lib.js';
import { useLayoutEffect } from '../../vendor/htm-preact-standalone.js';

// Each step: an icon and, on wide screens, the navigation links it points at.
const steps = [
  { id: 'welcome', logo: true },
  { id: 'nav', icon: 'menu', targets: ['nav.nav'], pad: 0 },
  { id: 'chat', icon: 'chat', targets: ['#/chat'] },
  { id: 'documents', icon: 'doc', targets: ['#/documents'] },
  { id: 'sharing', icon: 'share', badges: true },
  { id: 'lists', icon: 'list', targets: ['#/lists', '#/polls', '#/boards'] },
  { id: 'calendar', icon: 'calendar', targets: ['#/calendar', '#/meetings', '#/decisions'] },
  { id: 'search', icon: 'search', targets: ['#/search'] },
  { id: 'settings', icon: 'settings', targets: ['#/settings'] },
  { id: 'help', icon: 'help', targets: ['#/help'] },
];
const WIDE = '(min-width: 901px)'; // below this the navigation is an off-canvas drawer
const find = (sel) => document.querySelector(sel.startsWith('#') ? `nav.nav a[href="${sel}"]` : sel);

// startTour opens the tour from anywhere (used by the help page).
export function startTour() {
  setState({ tourRun: (S.tourRun || 0) + 1 });
}

// A device can opt out of the automatic start (used by automated browser
// tests): localStorage "samling.tour" = "off". startTour() still works.
function autoStartOff() {
  try { return localStorage.getItem('samling.tour') === 'off'; } catch { return false; }
}

async function markDone() {
  if (!S.me || (S.me.settings || {}).tour_done) return;
  // The server stores the settings object as a whole: keep everything else.
  const settings = { ...(S.me.settings || {}), tour_done: true };
  try {
    await api('PATCH', '/me', { settings });
    if (S.me) setState({ me: { ...S.me, settings: { ...(S.me.settings || {}), tour_done: true } } });
  } catch { /* not saved: the tour is simply offered again next time */ }
}

export function Tour() {
  const s = useStore();
  const [open, setOpen] = useState(false);
  const run = s.tourRun || 0;
  const seen = useRef(run);
  useEffect(() => {
    if ((S.me.settings || {}).tour_done || autoStartOff()) return undefined;
    const id = setTimeout(() => setOpen(true), 500);
    return () => clearTimeout(id);
  }, []);
  useEffect(() => {
    if (run !== seen.current) { seen.current = run; setOpen(true); }
  }, [run]);
  if (!open) return null;
  return html`<${TourDialog} key=${run} onClose=${() => { setOpen(false); markDone(); }} />`;
}

function TourDialog({ onClose }) {
  const [step, setStep] = useState(0);
  const [wide, setWide] = useState(() => window.matchMedia(WIDE).matches);
  const [tick, setTick] = useState(0);
  const [layout, setLayout] = useState(null); // null = centred card without pointing
  const root = useRef();
  const card = useRef();
  const primary = useRef();
  const cur = steps[step];
  const last = step === steps.length - 1;
  const go = (n) => setStep(Math.max(0, Math.min(steps.length - 1, n)));
  const live = useRef({});
  live.current = { step, go, onClose };

  // Follow the viewport: pointing only makes sense while the menu is visible.
  useEffect(() => {
    const mq = window.matchMedia(WIDE);
    const onMq = () => setWide(mq.matches);
    const onResize = () => setTick((n) => n + 1);
    mq.addEventListener('change', onMq);
    window.addEventListener('resize', onResize);
    return () => { mq.removeEventListener('change', onMq); window.removeEventListener('resize', onResize); };
  }, []);

  // Modal behaviour: the rest of the page is inert, focus stays in the card,
  // Escape skips, and focus returns to where it was afterwards.
  useEffect(() => {
    const prev = document.activeElement;
    const others = [...root.current.parentNode.children].filter((el) => el !== root.current && !el.inert);
    others.forEach((el) => { el.inert = true; });
    primary.current.focus();
    const key = (e) => {
      const { step: n, go: move, onClose: close } = live.current;
      if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); close(); return; }
      if (e.key === 'ArrowRight') move(n + 1);
      if (e.key === 'ArrowLeft') move(n - 1);
      if (e.key === 'Tab') {
        const f = [...card.current.querySelectorAll('button:not([disabled]),a[href]')];
        if (!f.length) return;
        const i = f.indexOf(document.activeElement);
        e.preventDefault();
        f[(i + (e.shiftKey ? -1 : 1) + f.length) % f.length].focus();
      }
    };
    document.addEventListener('keydown', key, true);
    return () => {
      document.removeEventListener('keydown', key, true);
      others.forEach((el) => { el.inert = false; });
      const back = prev && prev !== document.body && document.contains(prev) ? prev : document.getElementById('main');
      if (back && back.focus) back.focus();
    };
  }, []);

  // A button that had focus may have disappeared with the step (Back, Skip).
  useEffect(() => {
    if (!card.current.contains(document.activeElement)) primary.current.focus();
  }, [step, layout]);

  // Place the card beside the navigation links of the step and cut holes in
  // the dimmed backdrop around them.
  useLayoutEffect(() => {
    const els = wide && cur.targets ? cur.targets.map(find).filter(Boolean) : [];
    if (!els.length) { setLayout(null); return; }
    els[0].scrollIntoView({ block: 'nearest' });
    els[els.length - 1].scrollIntoView({ block: 'nearest' });
    const vw = window.innerWidth, vh = window.innerHeight;
    const pad = cur.pad ?? 3;
    const holes = [];
    els.forEach((el) => {
      const r = el.getBoundingClientRect();
      const y = Math.max(0, r.top - pad), x = Math.max(0, r.left - pad);
      const h = { x, y, w: Math.min(vw, r.right + pad) - x, h: Math.min(vh, r.bottom + pad) - y };
      const prev = holes[holes.length - 1];
      if (prev && h.y - (prev.y + prev.h) < 8) prev.h = h.y + h.h - prev.y; // neighbours share one frame
      else holes.push(h);
    });
    const w = card.current.offsetWidth, h = card.current.offsetHeight;
    const left = Math.max(...holes.map((r) => r.x + r.w)) + 20;
    if (left + w > vw - 12) { setLayout(null); return; }
    const first = holes[0];
    const mid = cur.pad === 0 ? Math.min(vh / 2, 260) : first.y + first.h / 2;
    const top = Math.max(12, Math.min(mid - Math.min(h / 2, 90), vh - h - 12));
    setLayout({ holes, vw, vh, left, top, arrow: Math.max(18, Math.min(mid - top - 8, h - 34)) });
  }, [step, wide, tick]);

  const hole = (r) => `M${r.x + 8} ${r.y}h${r.w - 16}a8 8 0 0 1 8 8v${r.h - 16}a8 8 0 0 1 -8 8h${16 - r.w}a8 8 0 0 1 -8 -8v${16 - r.h}a8 8 0 0 1 8 -8z`;
  const text = t('tour.' + cur.id + (cur.id === 'nav' && !wide ? '.text_narrow' : '.text'));
  return html`<div class=${'tour' + (layout ? '' : ' centred')} ref=${root}>
    <div class="tour-spot" key="spot">
      ${layout && html`<svg class="tour-dim" aria-hidden="true" width=${layout.vw} height=${layout.vh}>
        <path fill-rule="evenodd" d=${`M0 0h${layout.vw}v${layout.vh}h${-layout.vw}z` + (cur.pad === 0 ? layout.holes.map((r) => `M${r.x} ${r.y}h${r.w}v${r.h}h${-r.w}z`) : layout.holes.map(hole)).join('')} /></svg>`}
      ${layout && cur.pad !== 0 && layout.holes.map((r) => html`<div class="tour-ring" style=${{ left: r.x + 'px', top: r.y + 'px', width: r.w + 'px', height: r.h + 'px' }}></div>`)}
    </div>
    <div class="tour-card" key="card" role="dialog" aria-modal="true" aria-labelledby="tour-title" aria-describedby="tour-text" ref=${card}
        style=${layout ? { left: layout.left + 'px', top: layout.top + 'px' } : {}}>
      <div class="tour-body" aria-live="polite" aria-atomic="true">
        <div class="tour-art" aria-hidden="true">
          ${cur.logo ? html`<img src="/icons/icon.svg" alt="" width="56" height="56" />` : html`<span class="tour-icon"><${Icon} name=${cur.icon} /></span>`}
          ${cur.badges && html`<span class="tour-badges"><${SharedBadge} shared=${false} /><${Icon} name="right" /><${SharedBadge} shared=${true} /></span>`}
        </div>
        <p class="tour-count">${t('tour.step', { n: step + 1, total: steps.length })}</p>
        <h2 id="tour-title">${t('tour.' + cur.id + '.title')}</h2>
        <p id="tour-text">${text}</p>
      </div>
      <div class="tour-dots" aria-hidden="true">${steps.map((x, i) => html`<i class=${i === step ? 'on' : i < step ? 'past' : ''}></i>`)}</div>
      <div class="tour-buttons">
        ${!last && html`<button type="button" class="link" onClick=${onClose}>${t('tour.skip')}</button>`}
        <span class="grow"></span>
        ${step > 0 && html`<button type="button" onClick=${() => go(step - 1)}>${t('tour.back')}</button>`}
        <button type="button" class="primary" ref=${primary} onClick=${() => (last ? onClose() : go(step + 1))}>${last ? t('tour.done') : t('tour.next')}</button>
      </div>
    </div>
    ${layout && html`<span class="tour-arrow" key="arrow" style=${{ left: layout.left - 7 + 'px', top: layout.top + layout.arrow + 'px' }}></span>`}
  </div>`;
}
