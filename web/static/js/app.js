// Application shell: sign-in, navigation and routing to the views.
import { html, render, useState, useEffect, useStore, S, setState, t, api, act, errText, ApiError, loadSession, loadDirectory, startLive, stopLive,
  useRoute, useLive, userName, toast, Icon, Toasts, ErrorBoundary, Field, useForm, can, locales, getLocale, setLocale } from './lib.js';
import { Avatar } from './media.js';
import { Home } from './views/home.js';
import { Chat } from './views/chat.js';
import { Documents } from './views/documents.js';
import { Lists } from './views/lists.js';
import { Polls } from './views/polls.js';
import { Calendar } from './views/calendar.js';
import { Meetings } from './views/meetings.js';
import { Decisions } from './views/decisions.js';
import { Boards } from './views/boards.js';
import { Search } from './views/search.js';
import { People } from './views/people.js';
import { History } from './views/history.js';
import { Mentions } from './views/mentions.js';
import { Settings, PasswordForm, MfaSetup, Passkeys } from './views/settings.js';
import { passkeysSupported, signInWithPasskey } from './passkey.js';
import { Admin } from './views/admin.js';
import { Help } from './views/help.js';
import { Tour } from './views/tour.js';

const views = { home: Home, chat: Chat, documents: Documents, lists: Lists, polls: Polls, calendar: Calendar, meetings: Meetings,
  decisions: Decisions, boards: Boards, search: Search, people: People, history: History, mentions: Mentions, settings: Settings, admin: Admin, help: Help };

const navItems = [
  ['home', 'home'], ['chat', 'chat'], ['mentions', 'bell'], ['documents', 'doc'], ['lists', 'list'], ['polls', 'poll'], ['calendar', 'calendar'],
  ['meetings', 'meeting'], ['decisions', 'decision'], ['boards', 'board'], ['search', 'search'], ['people', 'people'], ['history', 'history'],
];

function LangSwitch() {
  useStore();
  return html`<label class="row small"><${Icon} name="globe" /><span class="sr-only">${t('settings.language')}</span>
    <select value=${getLocale()} onChange=${(e) => setLocale(e.target.value)}>
      ${Object.entries(locales).map(([k, v]) => html`<option value=${k}>${v}</option>`)}</select></label>`;
}

function Login() {
  const [f, , bind] = useForm({ username: '', password: '', code: '' });
  const [needCode, setNeedCode] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const submit = async (e) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      await api('POST', '/auth/login', { username: f.username, password: f.password, code: f.code });
      await loadSession();
    } catch (err) {
      if (err instanceof ApiError && err.code === 'mfa_required') setNeedCode(true);
      else setError(errText(err));
    }
    setBusy(false);
  };
  const passkey = async () => {
    setBusy(true);
    setError('');
    try {
      await signInWithPasskey();
      await loadSession();
    } catch (err) {
      // Closing the browser's passkey dialog is not an error worth showing.
      setError(err instanceof ApiError ? errText(err) : '');
    }
    setBusy(false);
  };
  return html`<main class="login"><form class="card" onSubmit=${submit}>
    <div class="lang"><${LangSwitch} /></div>
    <img src="/icons/icon.svg" alt="" width="56" height="56" />
    <h1>${S.org}</h1><p class="muted">${t('login.subtitle')}</p>
    <${Field} label=${t('login.username')}><input ...${bind('username')} autocomplete="username" autocapitalize="none" required /><//>
    <${Field} label=${t('login.password')}><input type="password" ...${bind('password')} autocomplete="current-password" required /><//>
    ${needCode && html`<${Field} label=${t('login.code')} hint=${t('login.code_hint')}>
      <input ...${bind('code')} autocomplete="one-time-code" autocapitalize="none" spellcheck="false" maxlength="24" autofocus /><//>`}
    ${error && html`<p class="error-text" role="alert">${error}</p>`}
    <button class="primary" disabled=${busy}>${t('login.submit')}</button>
    ${passkeysSupported() && html`<div class="or">${t('login.or')}</div>
      <button type="button" class="wide" onClick=${passkey} disabled=${busy}><${Icon} name="key" /> ${t('login.passkey')}</button>`}
  </form></main>`;
}

// Shown while the account must change its password or enrol two-factor.
function Pending() {
  const s = useStore();
  const logout = async () => { await act(() => api('POST', '/auth/logout')); setState({ me: null }); };
  return html`<main class="login"><div class="card">
    <div class="lang"><${LangSwitch} /></div>
    ${s.pending === 'password_change_required'
      ? html`<h1>${t('pending.password_title')}</h1><p class="muted">${t('pending.password_text')}</p><${PasswordForm} onDone=${loadSession} />`
      : html`<h1>${t('pending.mfa_title')}</h1><p class="muted">${t('pending.mfa_text')}</p>
          <h3>${t('pending.mfa_app')}</h3><${MfaSetup} onDone=${loadSession} />
          <h3 style=${{ marginTop: '1.2rem' }}>${t('pending.mfa_passkey')}</h3><${Passkeys} onChange=${loadSession} />`}
    <p><button class="link" onClick=${logout}>${t('nav.logout')}</button></p>
  </div></main>`;
}

function Shell() {
  const s = useStore();
  const route = useRoute();
  const [open, setOpen] = useState(false);
  const [unread, setUnread] = useState(0);
  const refreshUnread = () => api('GET', '/channels').then((cs) => setUnread(cs.reduce((n, c) => n + c.unread, 0))).catch(() => {});
  useEffect(() => { startLive(); refreshUnread(); return stopLive; }, []);
  useLive(['channel'], refreshUnread);
  // Mentions: the number in the menu, and a short notice when a new one arrives.
  const [mentions, setMentions] = useState(0);
  const refreshMentions = (announce) => api('GET', '/notifications').then((r) => {
    setMentions((old) => {
      if (announce && r.unread > old && r.items[0] && !r.items[0].read) toast(t('mention.toast', { name: userName(r.items[0].actor_id) }));
      return r.unread;
    });
  }).catch(() => {});
  useEffect(() => { refreshMentions(false); }, []);
  useLive(['mention'], () => refreshMentions(true));
  useEffect(() => { setOpen(false); if (route[0] !== 'chat') refreshUnread(); }, [route.join('/')]);
  useEffect(() => { document.title = (hasNav(route[0]) ? t('nav.' + route[0]) + ' – ' : '') + s.org; });
  const View = views[route[0]] || Home;
  const admin = can('users.manage') || can('roles.manage') || can('logs.view') || can('backup.manage');
  const logout = async () => { await act(() => api('POST', '/auth/logout')); stopLive(); setState({ me: null, csrf: '' }); };
  const link = (key, icon) => html`<a href=${'#/' + key} aria-current=${route[0] === key ? 'page' : null}>
    <${Icon} name=${icon} /> ${t('nav.' + key)}${key === 'chat' && unread > 0 && html`<span class="count" aria-label=${t('chat.unread', { n: unread })}>${unread}</span>`}
    ${key === 'mentions' && mentions > 0 && html`<span class="count" aria-label=${t('mention.unread', { n: mentions })}>${mentions}</span>`}</a>`;
  return html`<div class="shell">
    <a class="skip" href="#main">${t('nav.skip')}</a>
    <div class="topbar">
      <button class="icon-btn" onClick=${() => setOpen(!open)} aria-label=${t('nav.menu')} aria-expanded=${open}><${Icon} name="menu" /></button>
      <span class="brand">${s.org}</span>
      <a class="icon-btn" href="#/search" aria-label=${t('nav.search')}><${Icon} name="search" /></a>
    </div>
    <nav class=${'nav' + (open ? ' open' : '')} aria-label=${t('nav.main')} onClick=${(e) => { if (e.target === e.currentTarget) setOpen(false); }}>
      <div class="brand"><img src="/icons/icon.svg" alt="" width="28" height="28" /> ${s.org}</div>
      ${navItems.map(([k, i]) => link(k, i))}
      <div class="sep"></div>
      ${admin && link('admin', 'admin')}
      ${link('help', 'help')}
      ${link('settings', 'settings')}
      <div class="who"><${Avatar} user=${s.me} />
        <span class="grow">${s.me.display_name}<br /><span class="small">@${s.me.username}</span></span>
        <button class="icon-btn" onClick=${logout} aria-label=${t('nav.logout')} title=${t('nav.logout')}><${Icon} name="logout" /></button></div>
    </nav>
    ${open && html`<div class="scrim no-print" onClick=${() => setOpen(false)}></div>`}
    <main class="main" id="main" tabindex="-1"><${ErrorBoundary} key=${route[0]}><${View} route=${route} key=${route[0]} /><//></main>
    <${Tour} />
  </div>`;
}
const hasNav = (k) => k in views;

function App() {
  const s = useStore();
  if (!s.ready) return html`<div class="loading">…</div>`;
  let body;
  if (!s.me) body = html`<${Login} />`;
  else if (s.pending) body = html`<${Pending} />`;
  else body = html`<${Shell} />`;
  return html`${body}<${Toasts} />`;
}

render(html`<${App} />`, document.getElementById('app'));
loadSession();
export { loadDirectory };
