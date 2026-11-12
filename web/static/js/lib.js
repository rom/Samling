// Shared client infrastructure: rendering, API access, state, i18n, routing
// and the small UI building blocks used by every view.
import { html, render, Component, useState, useEffect, useRef, useMemo, useCallback } from '../vendor/htm-preact-standalone.js';
import en from './i18n/en.js';
import sv from './i18n/sv.js';
import guideEn from './i18n/guide-en.js';
import guideSv from './i18n/guide-sv.js';

export { html, render, useState, useEffect, useRef, useMemo, useCallback };

// ---------- global state ----------

const listeners = new Set();
export const S = {
  me: null, csrf: '', pending: '', org: 'Samling', maxUploadMB: 50, minPassword: 12,
  users: [], userMap: {}, groups: [], roles: [], blocks: [], ready: false,
};

let version = 0; // bumped on every change, so late subscribers can catch up
const notify = () => { version++; listeners.forEach((fn) => fn()); };

export function setState(patch) {
  Object.assign(S, patch);
  notify();
}

export function useStore() {
  const [, tick] = useState(0);
  const seen = version;
  useEffect(() => {
    const fn = () => tick((n) => n + 1);
    listeners.add(fn);
    // The state may have changed between this component's render and the
    // moment the effect subscribed (e.g. a fast response at start-up).
    if (seen !== version) fn();
    return () => listeners.delete(fn);
  }, []);
  return S;
}

export const can = (perm) => !!S.me && S.me.perms.includes(perm);

// ---------- i18n ----------

// The guide files hold the texts of the welcome tour and the online help.
const dict = { en: { ...en, ...guideEn }, sv: { ...sv, ...guideSv } };
export const locales = { sv: 'Svenska', en: 'English' };
let locale = 'sv';
try { locale = localStorage.getItem('samling.locale') || (navigator.language || 'sv').slice(0, 2); } catch { /* ignore */ }
if (!dict[locale]) locale = 'en';

export function t(key, vars) {
  let s = dict[locale][key] ?? dict.en[key] ?? key;
  if (vars) for (const k in vars) s = s.replaceAll('{' + k + '}', vars[k]);
  return s;
}
export const hasKey = (key) => key in dict[locale] || key in dict.en;
export const getLocale = () => locale;
export function setLocale(l) {
  if (!dict[l]) return;
  locale = l;
  try { localStorage.setItem('samling.locale', l); } catch { /* ignore */ }
  document.documentElement.lang = l;
  notify();
}
document.documentElement.lang = locale;

// ---------- appearance (theme, text size, accessibility) ----------

export const themes = ['light', 'dark', 'spring', 'autumn', 'sand', 'contrast'];
export const defaultPrefs = { theme: 'light', size: 'm', motion: false, underline: false, readable: false, focus: false };

export function applyPrefs(p) {
  const d = document.documentElement.dataset;
  const prefs = { ...defaultPrefs, ...p };
  d.theme = themes.includes(prefs.theme) ? prefs.theme : 'light';
  d.size = prefs.size;
  d.motion = prefs.motion ? 'reduce' : '';
  d.underline = prefs.underline ? '1' : '';
  d.readable = prefs.readable ? '1' : '';
  d.focus = prefs.focus ? '1' : '';
  try { localStorage.setItem('samling.prefs', JSON.stringify(prefs)); } catch { /* ignore */ }
}

// ---------- API ----------

export class ApiError extends Error {
  constructor(status, code) { super(code); this.status = status; this.code = code; }
}

export function errText(e) {
  if (e instanceof ApiError) return hasKey('err.' + e.code) ? t('err.' + e.code) : t('err.generic') + ' (' + e.code + ')';
  return t('err.network');
}

// Tells the server whether the person is actually using the application:
// background refreshes (live updates) must not keep an idle session alive.
let lastInteraction = Date.now();
['pointerdown', 'keydown', 'touchstart'].forEach((ev) => window.addEventListener(ev, () => { lastInteraction = Date.now(); }, { capture: true, passive: true }));

// safePath guards every API call: the path part may only contain plain
// segments. Ids from the address bar or from server data can therefore never
// redirect a request to another endpoint ("../" tricks), and neither can an
// encoded slash, because "%" is not allowed either.
function safePath(path) {
  const p = path.split('?')[0];
  return /^(\/[A-Za-z0-9_.-]+)+$/.test(p) && !p.split('/').some((seg) => seg === '.' || seg === '..');
}

async function request(method, path, opts = {}) {
  if (!safePath(path)) throw new ApiError(0, 'bad_request_path');
  const headers = { ...(opts.headers || {}) };
  if (method !== 'GET' && S.csrf) headers['X-CSRF-Token'] = S.csrf;
  if (Date.now() - lastInteraction < 90000) headers['X-User-Active'] = '1';
  let res;
  try {
    res = await fetch('/api' + path, { method, headers, body: opts.body, credentials: 'same-origin' });
  } catch {
    throw new ApiError(0, 'network');
  }
  if (!res.ok) {
    let code = 'generic';
    try { code = (await res.json()).error || code; } catch { /* not json */ }
    if (res.status === 401 && !path.startsWith('/auth/') && S.me) setState({ me: null, csrf: '' });
    if (code === 'password_change_required' || code === 'mfa_setup_required') setState({ pending: code });
    throw new ApiError(res.status, code);
  }
  return res;
}

export async function api(method, path, body) {
  const opts = {};
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
    opts.headers = { 'Content-Type': 'application/json' };
  }
  const res = await request(method, path, opts);
  // Only API answers are parsed as data; a file download never is, whatever
  // it contains.
  const ct = res.headers.get('Content-Type') || '';
  return ct.startsWith('application/json') && !res.headers.get('Content-Disposition') ? res.json() : res;
}

// apiSend posts a raw body (not JSON), for example a calendar file.
export async function apiSend(method, path, body, contentType) {
  return (await request(method, path, { body, headers: { 'Content-Type': contentType } })).json();
}

export async function uploadFile(file) {
  if (file.size > S.maxUploadMB * 1048576) throw new ApiError(413, 'too_large');
  const fd = new FormData();
  fd.append('file', file, file.name);
  return (await request('POST', '/files', { body: fd })).json();
}

// download fetches a (possibly POST generated) file and saves it.
export async function download(method, path, body, fallbackName) {
  const opts = body === undefined ? {} : { body: typeof body === 'string' ? body : JSON.stringify(body), headers: { 'Content-Type': 'application/json' } };
  const res = await request(method, path, opts);
  const cd = res.headers.get('Content-Disposition') || '';
  let name = fallbackName || 'download';
  const ext = /filename\*=UTF-8''([^;]+)/i.exec(cd); // RFC 5987 form: percent-encoded
  const plain = /filename="?([^";]+)"?/i.exec(cd);
  try {
    if (ext) name = decodeURIComponent(ext[1]);
    else if (plain) name = plain[1];
  } catch { /* keep the fallback name */ }
  const url = URL.createObjectURL(await res.blob());
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 10000);
}

// act runs an API action and reports failures as a toast. Returns the result
// or undefined on failure.
export async function act(fn, okMsg) {
  try {
    const r = await fn();
    if (okMsg) toast(okMsg);
    return r === undefined ? true : r;
  } catch (e) {
    toast(errText(e), 'error');
    return undefined;
  }
}

// ---------- session ----------

export async function loadSession() {
  try {
    const r = await api('GET', '/me');
    setState({ me: r.user, csrf: r.csrf, pending: r.pending, org: r.org_name, maxUploadMB: r.max_upload_mb, minPassword: r.min_password,
      idleMinutes: r.idle_timeout_minutes, quotaMB: r.quota_mb, ready: true });
    if (r.user.locale && r.user.locale !== locale) setLocale(r.user.locale);
    applyPrefs(r.user.settings || {});
    if (!r.pending) await loadDirectory();
  } catch {
    setState({ me: null, ready: true });
  }
}

export async function loadDirectory() {
  try {
    const [users, groups, roles, blocks] = await Promise.all([api('GET', '/users'), api('GET', '/groups'), api('GET', '/roles'), api('GET', '/me/blocks')]);
    const userMap = {};
    users.forEach((u) => { userMap[u.id] = u; });
    setState({ users, userMap, groups, roles: roles.roles, permissions: roles.permissions, blocks });
  } catch { /* shown elsewhere */ }
}

// setBlocked stops (or allows again) direct messages from one person.
export async function setBlocked(id, on) {
  if (!(await act(() => api(on ? 'PUT' : 'DELETE', '/me/blocks/' + id), t(on ? 'block.done' : 'block.undone')))) return;
  setState({ blocks: on ? [...S.blocks.filter((x) => x !== id), id] : S.blocks.filter((x) => x !== id) });
}

export const userName = (id) => (S.userMap[id] ? S.userMap[id].display_name : id ? t('common.deleted_user') : '');
export const roleName = (r) => (hasKey('role.' + r.id) ? t('role.' + r.id) : r.name);

// ---------- live updates ----------

const liveHandlers = new Set();
let source = null;
export function startLive() {
  if (source || !window.EventSource) return;
  source = new EventSource('/api/events');
  source.onmessage = (ev) => {
    try {
      const e = JSON.parse(ev.data);
      liveHandlers.forEach((fn) => fn(e));
    } catch { /* ignore */ }
  };
}
export function stopLive() {
  if (source) source.close();
  source = null;
}
// useLive calls fn whenever a resource of one of the given types changes.
export function useLive(types, fn) {
  const ref = useRef(fn);
  ref.current = fn;
  useEffect(() => {
    const h = (e) => { if (types.includes(e.type)) ref.current(e); };
    liveHandlers.add(h);
    return () => liveHandlers.delete(h);
  }, [types.join(',')]);
}

// ---------- routing (hash based) ----------

// The route comes from the address bar, so it can be chosen by whoever made
// a link. Only plain segments are accepted (ids, section and topic names);
// anything else shows the start page.
function parseHash() {
  try {
    const parts = (location.hash.replace(/^#\/?/, '') || 'home').split('/').map(decodeURIComponent);
    if (parts.every((p) => /^[A-Za-z0-9_-]{1,80}$/.test(p))) return parts;
  } catch { /* malformed escape */ }
  return ['home'];
}
export function useRoute() {
  const [route, setRoute] = useState(parseHash());
  useEffect(() => {
    const fn = () => setRoute(parseHash());
    window.addEventListener('hashchange', fn);
    return () => window.removeEventListener('hashchange', fn);
  }, []);
  return route;
}
export const go = (...parts) => { location.hash = '#/' + parts.map(encodeURIComponent).join('/'); };

// ---------- data loading hook ----------

export function useLoad(fn, deps = []) {
  const [state, setLoadState] = useState({ data: null, error: null, loading: true });
  const seq = useRef(0);
  const reload = useCallback(async () => {
    const n = ++seq.current;
    try {
      const data = await fn();
      if (n === seq.current) setLoadState({ data, error: null, loading: false });
    } catch (e) {
      if (n === seq.current) setLoadState((s) => ({ data: s.data, error: e, loading: false }));
    }
  }, deps);
  useEffect(() => { setLoadState((s) => ({ ...s, loading: true })); reload(); }, deps);
  return { ...state, reload };
}

// ---------- formatting ----------

const pad = (n) => String(n).padStart(2, '0');
const loc = () => (locale === 'sv' ? 'sv-SE' : 'en-GB');
export const fmtDate = (ts, utc) => new Date(ts * 1000).toLocaleDateString(loc(), { year: 'numeric', month: 'short', day: 'numeric', timeZone: utc ? 'UTC' : undefined });
export const fmtTime = (ts) => new Date(ts * 1000).toLocaleTimeString(loc(), { hour: '2-digit', minute: '2-digit' });
export const fmtDateTime = (ts) => fmtDate(ts) + ' ' + fmtTime(ts);
export function fmtAgo(ts) {
  const d = new Date(ts * 1000), now = new Date();
  return d.toDateString() === now.toDateString() ? fmtTime(ts) : fmtDateTime(ts);
}
export function fmtSize(n) {
  if (n < 1024) return n + ' B';
  if (n < 1048576) return (n / 1024).toFixed(0) + ' kB';
  return (n / 1048576).toFixed(1) + ' MB';
}
// <input type="datetime-local"> / <input type="date"> conversions
export function toInput(ts) {
  if (!ts) return '';
  const d = new Date(ts * 1000);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
export const fromInput = (s) => (s ? Math.floor(new Date(s).getTime() / 1000) : 0);
export const toDateInput = (ts) => (ts ? new Date(ts * 1000).toISOString().slice(0, 10) : '');
export const fromDateInput = (s) => (s ? Math.floor(Date.parse(s + 'T00:00:00Z') / 1000) : 0);

// Meeting links get a friendly label.
const providers = [[/teams\.microsoft\.com|teams\.live\.com/, 'Microsoft Teams'], [/zoom\.us/, 'Zoom'], [/meet\.google\.com/, 'Google Meet'],
  [/meet\.jit\.si/, 'Jitsi Meet'], [/webex\.com/, 'Webex'], [/whereby\.com/, 'Whereby']];
export function linkLabel(url) {
  for (const [re, name] of providers) if (re.test(url)) return name;
  return null;
}
// safeUrl guards every href built from user content.
export const safeUrl = (u) => (/^(https?:\/\/|mailto:)/i.test(u || '') ? u : null);

// ---------- internal links ----------

// Things inside Samling that a text can link to: kind → [section, icon].
// A link is written [[poll:<id>]]; the address of a page (…/#/polls/<id>)
// pasted into a text works the same way.
export const linkKinds = { channel: ['chat', 'chat'], document: ['documents', 'doc'], list: ['lists', 'list'], poll: ['polls', 'poll'],
  event: ['calendar', 'calendar'], meeting: ['meetings', 'meeting'], decision: ['decisions', 'decision'], board: ['boards', 'board'] };
const sectionKind = Object.fromEntries(Object.entries(linkKinds).map(([k, v]) => [v[0], k]));
export const refToken = (type, id) => `[[${type}:${id}]]`;

// Titles are asked for in one request per screenful and kept for a minute.
// The server only names what this person may read.
const refCache = new Map();
let refQueue = [];
function lookupRef(type, id) {
  const key = type + ':' + id;
  const hit = refCache.get(key);
  if (hit && (hit.pending || Date.now() - hit.at < 60000)) return hit;
  const entry = { pending: true, title: hit ? hit.title : '', ok: hit ? hit.ok : false };
  refCache.set(key, entry);
  if (refQueue.push({ type, id }) === 1) {
    setTimeout(async () => {
      const refs = refQueue.slice(0, 100);
      refQueue = [];
      let found = [];
      try { found = await api('POST', '/links', { refs }); } catch { /* shown as unavailable */ }
      refs.forEach((r) => refCache.set(r.type + ':' + r.id, { at: Date.now(), ok: false, title: '' }));
      found.forEach((r) => refCache.set(r.type + ':' + r.id, { at: Date.now(), ok: true, title: r.title, data: r }));
      notify();
    }, 30);
  }
  return entry;
}

// RefLink shows a link to something in Samling by its current title. When
// the reader has no access to it (or it is gone) the link says only that.
//
// With "embed" (used in chat messages) a linked message is shown as a
// quotation and a linked document as a card with a download button.
export function RefLink({ type, id, embed }) {
  useStore();
  const [section, icon] = linkKinds[type] || ['chat', 'chat'];
  const r = lookupRef(type, id);
  if (!r.ok) {
    return r.pending ? html`<span class="ref"><${Icon} name=${icon} /> …</span>`
      : html`<span class="ref none" title=${t('link.none_hint')}><${Icon} name="lock" /> ${t('link.none', { kind: t('link.kind_' + type) })}</span>`;
  }
  const d = r.data || {};
  if (type === 'message') {
    const href = '#/chat/' + encodeURIComponent(d.channel_id) + '/' + encodeURIComponent(id);
    const where = r.title ? '#' + r.title : t('link.dm');
    if (!embed) return html`<a class="ref" href=${href}><${Icon} name="chat" /> ${t('link.message_by', { name: userName(d.user_id), where })}</a>`;
    return html`<blockquote class="quote">
      <div class="quote-head"><b>${userName(d.user_id)}</b> <span class="muted small">${where} · ${fmtAgo(d.at)}</span>
        <a class="small" href=${href}>${t('link.open_message')}</a></div>
      <div class="pre">${d.text}${d.files > 0 && html` <span class="muted small"><${Icon} name="clip" /> ${t('link.files', { n: d.files })}</span>`}</div></blockquote>`;
  }
  const href = '#/' + section + '/' + encodeURIComponent(id);
  if (type === 'document' && embed && d.file_id) {
    const picture = /^image\/(jpeg|png|gif|webp)$/.test(d.mime || '') && /^[A-Za-z0-9_-]+$/.test(d.file_id);
    return html`<span class="doc-card">
      ${picture ? html`<a href=${href}><img src=${'/api/files/' + d.file_id + '/image'} alt="" loading="lazy" /></a>` : html`<${Icon} name="doc" />`}
      <span class="grow"><a href=${href}><b>${r.title}</b></a><br />
        <span class="muted small">${d.folder ? '/' + d.folder + ' · ' : ''}v${d.version} · ${fmtSize(d.size)}</span></span>
      <button type="button" class="icon-btn" aria-label=${t('common.download') + ': ' + r.title} title=${t('common.download')}
        onClick=${() => act(() => download('GET', '/documents/' + id + '/download', undefined, d.name))}><${Icon} name="download" /></button></span>`;
  }
  return html`<a class="ref" href=${href}><${Icon} name=${icon} /> ${r.title || t('link.dm')}</a>`;
}

// copyText puts a text on the clipboard (or shows it for copying by hand
// where the browser does not allow that).
export async function copyText(text, okMsg) {
  try { await navigator.clipboard.writeText(text); toast(okMsg); } catch { window.prompt(okMsg, text); }
}
export const pageUrl = (...parts) => location.origin + '/#/' + parts.map(encodeURIComponent).join('/');

// internalRef recognises the address of a page of this Samling.
function internalRef(url) {
  try {
    const u = new URL(url);
    if (u.origin !== location.origin) return null;
    const msg = /^#\/chat\/[A-Za-z0-9_-]{1,80}\/([A-Za-z0-9_-]{1,80})$/.exec(u.hash);
    if (msg) return { type: 'message', id: msg[1] };
    const m = /^#\/([a-z]+)\/([A-Za-z0-9_-]{1,80})$/.exec(u.hash);
    return m && sectionKind[m[1]] ? { type: sectionKind[m[1]], id: m[2] } : null;
  } catch { return null; }
}

// refsIn lists the internal links in a text (for fields that are edited as
// plain text, where the links are shown beside the field).
export function refsIn(text) {
  const out = [], seen = new Set();
  for (const part of linkify(text || '')) {
    if (part && part.type === RefLink && !seen.has(part.props.type + part.props.id)) { seen.add(part.props.type + part.props.id); out.push(part); }
  }
  return out;
}

// linkify turns URLs, e-mail addresses and internal links in plain text into
// links. Text is always rendered as text nodes (never as HTML).
export function linkify(text) {
  const out = [];
  const re = /(https?:\/\/[^\s<>"']+[^\s<>"'.,;:!?)\]])|([\w.+-]+@[\w-]+(?:\.[\w-]+)+)|\[\[(channel|document|list|poll|event|meeting|decision|board|message):([A-Za-z0-9_-]{1,80})\]\]/g;
  let last = 0, m;
  while ((m = re.exec(text))) {
    if (m.index > last) out.push(text.slice(last, m.index));
    const ref = m[3] ? { type: m[3], id: m[4] } : m[1] ? internalRef(m[1]) : null;
    if (ref) {
      out.push(html`<${RefLink} type=${ref.type} id=${ref.id} />`);
    } else if (m[1]) {
      const label = linkLabel(m[1]);
      out.push(html`<a href=${m[1]} target="_blank" rel="noopener noreferrer" class=${label ? 'meetlink' : ''}>${label ? html`<${Icon} name="video" /> ${t('common.join', { name: label })}` : m[1]}</a>`);
    } else {
      out.push(html`<a href=${'mailto:' + m[2]}>${m[2]}</a>`);
    }
    last = re.lastIndex;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

// ---------- icons (inline SVG) ----------

const paths = {
  home: 'M3 11l9-8 9 8M5 10v10h5v-6h4v6h5V10',
  chat: 'M4 5h16v11H9l-5 4z',
  doc: 'M6 3h8l4 4v14H6zM14 3v4h4',
  list: 'M9 6h11M9 12h11M9 18h11M4.5 6h.01M4.5 12h.01M4.5 18h.01',
  poll: 'M5 20V10M12 20V4M19 20v-7',
  calendar: 'M4 6h16v14H4zM4 10h16M8 3v4M16 3v4',
  meeting: 'M8 11a3 3 0 100-6 3 3 0 000 6zM2.5 20a5.5 5.5 0 0111 0M17 11a2.5 2.5 0 100-5M16 14.5a5 5 0 015.5 5.5',
  decision: 'M12 3l8 3v6c0 4.5-3.2 7.6-8 9-4.8-1.400-8-4.500-8-9V6zM8.5 12l2.500 2.500 4.500-5',
  board: 'M4 4h5v16H4zM10.500 4h5v10h-5zM17 4h3.500v13H17z',
  search: 'M11 18a7 7 0 100-14 7 7 0 000 14zM20 20l-4-4',
  history: 'M4 12a8 8 0 108-8 8 8 0 00-6.500 3.300M4 4v4h4M12 8v4.500l3 2',
  people: 'M12 11a4 4 0 100-8 4 4 0 000 8zM4 21a8 8 0 0116 0',
  settings: 'M12 15a3 3 0 100-6 3 3 0 000 6zM19 12l2-1.500-2-3.500-2.300 1a7 7 0 00-1.700-1L14.500 4.500h-5L9 7a7 7 0 00-1.700 1L5 7 3 10.500 5 12l-2 1.500L5 17l2.300-1a7 7 0 001.700 1l.500 2.500h5L15 17a7 7 0 001.700-1l2.300 1 2-3.500z',
  admin: 'M12 3l8 3v6c0 4.500-3.200 7.600-8 9-4.800-1.400-8-4.500-8-9V6zM12 8v5M12 16h.01',
  plus: 'M12 5v14M5 12h14',
  x: 'M6 6l12 12M18 6L6 18',
  pin: 'M9 4h6l-1 6 3 3H7l3-3zM12 13v7',
  clip: 'M20 11l-8.500 8.500a5 5 0 01-7-7L13 4a3.500 3.500 0 015 5l-8.500 8.500a2 2 0 01-3-3L14 7',
  send: 'M4 12l16-8-6 16-3-6.500z',
  trash: 'M5 7h14M10 7V4h4v3M7 7l1 13h8l1-13',
  edit: 'M4 20l1-4L16 5l3 3L8 19zM14 7l3 3',
  share: 'M16 8a3 3 0 100-6 3 3 0 000 6zM6 15a3 3 0 100-6 3 3 0 000 6zM16 22a3 3 0 100-6 3 3 0 000 6zM8.600 13.500l4.800 3M13.400 7.500l-4.800 3',
  download: 'M12 4v11M7 11l5 5 5-5M5 20h14',
  upload: 'M12 16V5M7 9l5-5 5 5M5 20h14',
  menu: 'M4 6h16M4 12h16M4 18h16',
  lock: 'M6 11h12v9H6zM8.500 11V8a3.500 3.500 0 017 0v3',
  check: 'M5 12.500l4.500 4.500L19 7.500',
  left: 'M15 5l-7 7 7 7',
  right: 'M9 5l7 7-7 7',
  up: 'M6 15l6-6 6 6',
  down: 'M6 9l6 6 6-6',
  video: 'M4 7h11v10H4zM15 10.500l5-3v9l-5-3',
  logout: 'M14 4h5v16h-5M10 8l-4 4 4 4M6 12h10',
  copy: 'M9 9h10v11H9zM5 15V4h10',
  globe: 'M12 21a9 9 0 100-18 9 9 0 000 18zM3 12h18M12 3c3 3.500 3 14.500 0 18M12 3c-3 3.500-3 14.500 0 18',
  mail: 'M4 6h16v12H4zM4 7l8 6 8-6',
  help: 'M12 21a9 9 0 100-18 9 9 0 000 18zM9.500 9.500a2.500 2.500 0 114 2c-.900.600-1.500 1.200-1.500 2.300M12 17h.01',
  smile: 'M12 21a9 9 0 100-18 9 9 0 000 18zM8.500 14.500a4.500 4.500 0 007 0M9 9.500h.01M15 9.500h.01',
  camera: 'M4 8h3.500L9 5.500h6L16.500 8H20v11H4zM12 16.500a3 3 0 100-6 3 3 0 000 6z',
  scan: 'M4 8V5h3M17 5h3v3M20 16v3h-3M7 19H4v-3M8 10h8M8 14h5',
  reply: 'M9 7l-5 5 5 5M4 12h10a6 6 0 016 6v1',
  block: 'M12 21a9 9 0 100-18 9 9 0 000 18zM5.600 5.600l12.800 12.800',
  image: 'M4 5h16v14H4zM4 16l5-5 4 4 3-3 4 4M15 9.500h.01',
  bell: 'M6 16V11a6 6 0 0112 0v5l1.500 2h-15zM10 20.500a2 2 0 004 0',
  link: 'M10 14a4 4 0 005.700 0l3-3a4 4 0 00-5.700-5.700l-1 1M14 10a4 4 0 00-5.700 0l-3 3a4 4 0 005.700 5.700l1-1',
  sort: 'M8 5v14M5 8l3-3 3 3M16 19V5M13 16l3 3 3-3',
  key: 'M15 9a4 4 0 11-3.900 5H8v2H6v2H3v-3l6.100-6.100A4 4 0 0115 9zM16 8h.01',
};
export function Icon({ name, label }) {
  return html`<svg class="icon" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.800"
    stroke-linecap="round" stroke-linejoin="round" aria-hidden=${label ? null : 'true'} role=${label ? 'img' : null} aria-label=${label || null}>
    <path d=${paths[name] || ''} /></svg>`;
}

// ---------- toasts ----------

let toastSeq = 0;
export function toast(msg, kind = 'ok') {
  const id = ++toastSeq;
  setState({ toasts: [...(S.toasts || []), { id, msg, kind }] });
  setTimeout(() => setState({ toasts: (S.toasts || []).filter((x) => x.id !== id) }), kind === 'error' ? 6000 : 3000);
}
export function Toasts() {
  const s = useStore();
  return html`<div class="toasts" role="status" aria-live="polite">
    ${(s.toasts || []).map((x) => html`<div key=${x.id} class=${'toast ' + x.kind}>${x.msg}</div>`)}</div>`;
}

// ---------- building blocks ----------

export function Modal({ title, onClose, children, wide }) {
  const ref = useRef();
  useEffect(() => {
    const prev = document.activeElement;
    const el = ref.current;
    const first = el.querySelector('input:not([type=hidden]),textarea,select,button.primary') || el;
    first.focus();
    const key = (e) => {
      if (e.key === 'Escape') onClose();
      if (e.key === 'Tab') { // keep focus inside the dialog
        const f = el.querySelectorAll('a[href],button:not([disabled]),input:not([disabled]),select,textarea,[tabindex="0"]');
        if (!f.length) return;
        if (e.shiftKey && document.activeElement === f[0]) { e.preventDefault(); f[f.length - 1].focus(); }
        else if (!e.shiftKey && document.activeElement === f[f.length - 1]) { e.preventDefault(); f[0].focus(); }
      }
    };
    document.addEventListener('keydown', key);
    return () => { document.removeEventListener('keydown', key); if (prev && prev.focus) prev.focus(); };
  }, []);
  return html`<div class="overlay" onMouseDown=${(e) => { if (e.target === e.currentTarget) onClose(); }}>
    <div class=${'modal' + (wide ? ' wide' : '')} role="dialog" aria-modal="true" aria-label=${title} ref=${ref} tabindex="-1">
      <header><h2>${title}</h2>
        <button class="icon-btn" onClick=${onClose} aria-label=${t('common.close')}><${Icon} name="x" /></button></header>
      <div class="modal-body">${children}</div>
    </div></div>`;
}

// Field labels a single control. With "group" it labels a composite widget
// (several controls) instead, where a wrapping <label> would be wrong. The
// hint is kept outside the label so it does not bloat the accessible name.
export function Field({ label, hint, children, group }) {
  const tip = hint && html`<span class="hint">${hint}</span>`;
  if (group) return html`<div class="field" role="group" aria-label=${label}><span class="label">${label}</span>${children}${tip}</div>`;
  return html`<div class="field"><label><span class="label">${label}</span>${children}</label>${tip}</div>`;
}

// useForm keeps a flat form object; bind(name) wires an input.
export function useForm(initial) {
  const [v, setV] = useState(initial);
  const set = (k, val) => setV((old) => ({ ...old, [k]: val }));
  const bind = (k) => ({ value: v[k] ?? '', onInput: (e) => set(k, e.target.value) });
  const check = (k) => ({ checked: !!v[k], onChange: (e) => set(k, e.target.checked) });
  return [v, set, bind, check, setV];
}

export function Page({ title, actions, children, back }) {
  // Every page links to the online help topic of its section.
  const section = (location.hash.replace(/^#\/?/, '') || 'home').split('/')[0];
  // Pages of a single thing (a poll, a board …) offer their address for pasting into a text.
  const parts = (location.hash.replace(/^#\/?/, '') || 'home').split('/');
  const copyLink = parts.length === 2 && sectionKind[section] && parts[1] !== 'new' && html`<button class="icon-btn no-print" aria-label=${t('link.copy')} title=${t('link.copy')}
    onClick=${() => copyText(pageUrl(section, parts[1]), t('link.copied'))}><${Icon} name="link" /></button>`;
  const helpLink = section !== 'help' && html`<a class="icon-btn no-print" href=${'#/help/' + encodeURIComponent(section)} aria-label=${t('nav.help')} title=${t('nav.help')}><${Icon} name="help" /></a>`;
  return html`<section class="page">
    <header class="page-head">
      ${back && html`<a class="icon-btn" href=${back} aria-label=${t('common.back')}><${Icon} name="left" /></a>`}
      <h1>${title}</h1><div class="actions">${actions}${copyLink}${helpLink}</div>
    </header>${children}</section>`;
}

export function Empty({ icon, text, children }) {
  return html`<div class="empty"><${Icon} name=${icon || 'list'} /><p>${text}</p>${children}</div>`;
}

export function Loading({ state }) {
  if (state.error && !state.data) return html`<div class="empty"><p>${errText(state.error)}</p><button onClick=${state.reload}>${t('common.retry')}</button></div>`;
  return html`<div class="loading" role="status">${t('common.loading')}</div>`;
}

export const Badge = ({ kind, children }) => html`<span class=${'badge ' + (kind || '')}>${children}</span>`;
export const SharedBadge = ({ shared }) => (shared
  ? html`<${Badge} kind="info"><${Icon} name="share" /> ${t('share.shared')}<//>`
  : html`<${Badge}><${Icon} name="lock" /> ${t('share.private')}<//>`);

// ---------- sharing ----------

// ShareEditor edits a list of shares: [{type, id, level}].
export function ShareEditor({ value, onChange, levels = true }) {
  const s = useStore();
  const [pick, setPick] = useState('');
  const options = useMemo(() => [
    { key: 'all:', label: t('share.everyone') },
    ...s.groups.map((g) => ({ key: 'group:' + g.id, label: (g.scope === 'local' ? t('share.my_group') : t('share.group')) + ': ' + g.name })),
    ...s.roles.map((r) => ({ key: 'role:' + r.id, label: t('share.role') + ': ' + roleName(r) })),
    ...s.users.filter((u) => u.id !== s.me.id).map((u) => ({ key: 'user:' + u.id, label: u.display_name + ' (@' + u.username + ')' })),
  ], [s.groups, s.roles, s.users]);
  const labelOf = (sh) => {
    if (sh.type === 'group' && !sh.id) return t('share.hidden_group');
    const o = options.find((x) => x.key === sh.type + ':' + sh.id);
    return o ? o.label : sh.name || sh.id;
  };
  const used = new Set(value.map((x) => x.type + ':' + x.id));
  const add = (key) => {
    if (!key) return;
    const i = key.indexOf(':');
    onChange([...value, { type: key.slice(0, i), id: key.slice(i + 1), level: 1 }]);
    setPick('');
  };
  return html`<div class="shares">
    ${value.length === 0 && html`<p class="muted"><${Icon} name="lock" /> ${t('share.private_hint')}</p>`}
    ${value.map((sh, i) => html`<div class="share-row" key=${sh.type + sh.id}>
      <span class="grow">${labelOf(sh)}</span>
      ${levels && html`<select aria-label=${t('share.level')} value=${sh.level} onChange=${(e) => onChange(value.map((x, j) => (j === i ? { ...x, level: +e.target.value } : x)))}>
        <option value="1">${t('share.read')}</option><option value="2">${t('share.write')}</option></select>`}
      <button type="button" class="icon-btn" aria-label=${t('common.remove')} onClick=${() => onChange(value.filter((_, j) => j !== i))}><${Icon} name="x" /></button>
    </div>`)}
    <select aria-label=${t('share.add')} value=${pick} onChange=${(e) => add(e.target.value)}>
      <option value="">${t('share.add')}…</option>
      ${options.filter((o) => !used.has(o.key)).map((o) => html`<option value=${o.key}>${o.label}</option>`)}
    </select></div>`;
}

// ShareDialog edits the shares of an existing resource.
export function ShareDialog({ type, id, onClose, levels = true }) {
  const [shares, setShares] = useState(null);
  useEffect(() => { api('GET', `/acl/${type}/${id}`).then(setShares).catch((e) => { toast(errText(e), 'error'); onClose(); }); }, []);
  const save = async () => {
    if (await act(() => api('PUT', `/acl/${type}/${id}`, shares.map(({ type: ty, id: i, level }) => ({ type: ty, id: i, level }))), t('share.saved'))) onClose(true);
  };
  return html`<${Modal} title=${t('share.title')} onClose=${() => onClose(false)}>
    ${shares ? html`<${ShareEditor} value=${shares} onChange=${setShares} levels=${levels} />
      <div class="buttons"><button onClick=${() => onClose(false)}>${t('common.cancel')}</button>
      <button class="primary" onClick=${save}>${t('common.save')}</button></div>` : html`<div class="loading">${t('common.loading')}</div>`}
  <//>`;
}

export function ShareButton({ type, id, onDone, levels }) {
  const [open, setOpen] = useState(false);
  return html`<button onClick=${() => setOpen(true)}><${Icon} name="share" /> ${t('share.button')}</button>
    ${open && html`<${ShareDialog} type=${type} id=${id} levels=${levels} onClose=${(changed) => { setOpen(false); if (changed && onDone) onDone(); }} />`}`;
}

// ErrorBoundary keeps a failing view from taking the whole application down.
export class ErrorBoundary extends Component {
  constructor() { super(); this.state = { failed: false }; }
  componentDidCatch(error) { console.error(error); this.setState({ failed: true }); }
  render() {
    if (!this.state.failed) return this.props.children;
    return html`<div class="empty" role="alert"><p>${t('err.view')}</p>
      <button class="primary" onClick=${() => location.reload()}>${t('common.reload')}</button></div>`;
  }
}

export function confirmDelete(what) {
  return window.confirm(t('common.confirm_delete', { what }));
}
