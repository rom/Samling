import { html, useState, useStore, S, setState, t, api, act, errText, toast, download, loadSession, useLoad, fmtDate, Page, Field, useForm, Icon, Badge,
  themes, defaultPrefs, applyPrefs, locales, getLocale, setLocale } from '../lib.js';
import { passkeysSupported, registerPasskey } from '../passkey.js';

export function PasswordForm({ onDone }) {
  const [f, , bind, , setF] = useForm({ current: '', new: '', repeat: '' });
  const [error, setError] = useState('');
  const submit = async (e) => {
    e.preventDefault();
    if (f.new !== f.repeat) { setError(t('settings.password_mismatch')); return; }
    setError('');
    if (await act(() => api('POST', '/me/password', { current: f.current, new: f.new }), t('settings.password_changed'))) {
      setF({ current: '', new: '', repeat: '' });
      if (onDone) onDone();
    }
  };
  return html`<form onSubmit=${submit}>
    <${Field} label=${t('settings.current_password')}><input type="password" ...${bind('current')} autocomplete="current-password" required /><//>
    <${Field} label=${t('settings.new_password')} hint=${t('settings.password_hint', { n: S.minPassword })}>
      <input type="password" ...${bind('new')} autocomplete="new-password" minlength=${S.minPassword} required /><//>
    <${Field} label=${t('settings.repeat_password')}><input type="password" ...${bind('repeat')} autocomplete="new-password" required /><//>
    ${error && html`<p class="error-text" role="alert">${error}</p>`}
    <div class="buttons"><button class="primary">${t('settings.change_password')}</button></div>
  </form>`;
}

// RecoveryCodes shows freshly issued one-time codes. They are shown once.
export function RecoveryCodes({ codes, onDone }) {
  const text = codes.join('\n');
  const copy = async () => { try { await navigator.clipboard.writeText(text); toast(t('settings.copied')); } catch { /* clipboard unavailable */ } };
  const save = () => {
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([t('settings.recovery_file_head', { org: S.org, user: S.me.username }) + '\n\n' + text + '\n'], { type: 'text/plain' }));
    a.download = 'samling-recovery-codes.txt';
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 10000);
  };
  return html`<div class="notice" role="status">
    <h3>${t('settings.recovery_title')}</h3>
    <p>${t('settings.recovery_text')}</p>
    <ul class="codes">${codes.map((c) => html`<li key=${c}>${c}</li>`)}</ul>
    <div class="row"><button type="button" onClick=${copy}><${Icon} name="copy" /> ${t('settings.copy')}</button>
      <button type="button" onClick=${save}><${Icon} name="download" /> ${t('common.download')}</button>
      <button type="button" onClick=${() => window.print()}>${t('meet.print')}</button>
      <button type="button" class="primary right" onClick=${onDone}>${t('settings.recovery_saved')}</button></div>
  </div>`;
}

// MfaSetup enrols an authenticator app. The password is asked again: a
// session alone must not be enough to add a sign-in method.
export function MfaSetup({ onDone }) {
  const [setup, setSetup] = useState(null);
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [codes, setCodes] = useState(null);
  const start = async (e) => {
    e.preventDefault();
    const r = await act(() => api('POST', '/me/mfa/setup', { password }));
    if (r) { setSetup(r); setPassword(''); }
  };
  const enable = async (e) => {
    e.preventDefault();
    const r = await act(() => api('POST', '/me/mfa/enable', { code }), t('settings.mfa_enabled'));
    if (r) setCodes(r.recovery_codes || []);
  };
  if (codes) return html`<${RecoveryCodes} codes=${codes} onDone=${onDone} />`;
  if (!setup) return html`<form class="row" onSubmit=${start}>
    <input type="password" placeholder=${t('settings.current_password')} aria-label=${t('settings.current_password')} value=${password}
      onInput=${(e) => setPassword(e.target.value)} required autocomplete="current-password" />
    <button class="primary"><${Icon} name="lock" /> ${t('settings.mfa_start')}</button></form>`;
  return html`<form onSubmit=${enable}>
    <p>${t('settings.mfa_scan')}</p>
    ${setup.qr && html`<div class="qr"><img src=${setup.qr} alt=${t('settings.mfa_qr_alt')} /></div>`}
    <p class="small">${t('settings.mfa_manual')}<br /><code class="secret">${setup.secret}</code></p>
    <p class="small"><a href=${setup.uri}>${t('settings.mfa_open_app')}</a></p>
    <${Field} label=${t('login.code')}><input value=${code} onInput=${(e) => setCode(e.target.value)} inputmode="numeric" autocomplete="one-time-code" maxlength="7" required /><//>
    <div class="buttons"><button class="primary">${t('settings.mfa_confirm')}</button></div>
  </form>`;
}

// Passkeys: list, add (password required) and remove.
export function Passkeys({ onChange }) {
  const st = useLoad(() => api('GET', '/me/passkeys'));
  const [f, , bind, , setF] = useForm({ password: '', name: '' });
  const [busy, setBusy] = useState(false);
  const supported = passkeysSupported();
  const add = async (e) => {
    e.preventDefault();
    setBusy(true);
    try {
      await registerPasskey(f.password, f.name || t('settings.passkey_default_name'));
      toast(t('settings.passkey_added'));
      setF({ password: '', name: '' });
      st.reload();
      if (onChange) onChange();
    } catch (err) {
      // The person cancelling the browser dialog is not an error to report loudly.
      toast(err && err.code ? errText(err) : t('settings.passkey_cancelled'), 'error');
    }
    setBusy(false);
  };
  const remove = async (p) => {
    const password = window.prompt(t('settings.passkey_remove_prompt', { name: p.name }));
    if (!password) return;
    if (await act(() => api('POST', `/me/passkeys/${p.id}/delete`, { password }), t('settings.passkey_removed'))) { st.reload(); if (onChange) onChange(); }
  };
  return html`<div>
    ${st.data && st.data.length > 0 && html`<div class="rows" style=${{ marginBottom: '.8rem' }}>${st.data.map((p) => html`<div key=${p.id}>
      <${Icon} name="key" /><span class="grow"><b>${p.name}</b><br /><span class="muted small">${t('settings.passkey_created', { when: fmtDate(p.created_at) })}${
        p.last_used_at ? ' · ' + t('settings.passkey_used', { when: fmtDate(p.last_used_at) }) : ''}</span></span>
      <button class="icon-btn" onClick=${() => remove(p)} aria-label=${t('common.delete') + ': ' + p.name}><${Icon} name="trash" /></button></div>`)}</div>`}
    ${supported ? html`<form class="row" onSubmit=${add}>
      <input placeholder=${t('settings.passkey_name')} aria-label=${t('settings.passkey_name')} ...${bind('name')} maxlength="60" />
      <input type="password" placeholder=${t('settings.current_password')} aria-label=${t('settings.current_password')} ...${bind('password')} required autocomplete="current-password" />
      <button class="primary" disabled=${busy}><${Icon} name="key" /> ${t('settings.passkey_add')}</button></form>`
      : html`<p class="muted small">${t('settings.passkey_unsupported')}</p>`}
  </div>`;
}

const themeColors = {
  light: ['#f4f6f8', '#ffffff', '#2456c8', '#17243a'], dark: ['#12161c', '#1b212a', '#7aa7ff', '#0d1116'],
  spring: ['#f3f8f1', '#ffffff', '#2e7d4f', '#234a35'], autumn: ['#faf5ef', '#fffdfa', '#b3501b', '#4a2a1a'],
  sand: ['#f6f2e9', '#fdfbf6', '#7a6230', '#5b5140'], contrast: ['#000000', '#1a1a1a', '#ffe600', '#ffffff'],
};

export function Settings() {
  const s = useStore();
  const me = s.me;
  const prefs = { ...defaultPrefs, ...(me.settings || {}) };
  const [f, , bind, check] = useForm({ display_name: me.display_name, email: me.email, phone: me.phone, contact_visible: me.contact_visible });
  const [pw, setPw] = useState('');

  const savePrefs = async (patch) => {
    const next = { ...prefs, ...patch };
    applyPrefs(next); // apply at once, then persist to the account
    const u = await act(() => api('PATCH', '/me', { settings: next }));
    if (u) setState({ me: { ...S.me, settings: next } });
  };
  const saveProfile = async (e) => {
    e.preventDefault();
    const u = await act(() => api('PATCH', '/me', f), t('common.saved'));
    if (u) { setState({ me: { ...S.me, ...u } }); loadSession(); }
  };
  const changeLang = async (l) => {
    setLocale(l);
    await act(() => api('PATCH', '/me', { locale: l }));
    setState({ me: { ...S.me, locale: l } });
  };
  const disableMfa = async (e) => {
    e.preventDefault();
    if (await act(() => api('POST', '/me/mfa/disable', { password: pw }), t('settings.mfa_disabled'))) { setPw(''); loadSession(); }
  };
  const [newCodes, setNewCodes] = useState(null);
  const regenerate = async () => {
    const r = await act(() => api('POST', '/me/mfa/recovery-codes', { password: pw }));
    if (r) { setPw(''); setNewCodes(r.recovery_codes); }
  };
  const endOthers = async () => { if (await act(() => api('POST', '/me/sessions/end-others'), t('settings.sessions_ended'))) loadSession(); };
  const toggle = (key, label, hint) => html`<label class="check">
    <input type="checkbox" checked=${!!prefs[key]} onChange=${(e) => savePrefs({ [key]: e.target.checked })} />
    <span>${label}${hint && html`<br /><span class="muted small">${hint}</span>`}</span></label>`;

  return html`<${Page} title=${t('nav.settings')}><div class="stack">
    <div class="card"><h2>${t('settings.appearance')}</h2>
      <fieldset><legend>${t('settings.language')}</legend>
        <div class="row">${Object.entries(locales).map(([k, v]) => html`<label class="check"><input type="radio" name="lang" checked=${getLocale() === k}
          onChange=${() => changeLang(k)} /> ${v}</label>`)}</div></fieldset>
      <fieldset><legend>${t('settings.theme')}</legend>
        <div class="swatches">${themes.map((th) => html`<button type="button" class="swatch" aria-pressed=${prefs.theme === th} onClick=${() => savePrefs({ theme: th })}>
          <span class="prev">${themeColors[th].map((c) => html`<i style=${{ background: c }}></i>`)}</span>${t('theme.' + th)}</button>`)}</div></fieldset>
      <fieldset><legend>${t('settings.text_size')}</legend>
        <div class="row">${['s', 'm', 'l', 'xl'].map((sz) => html`<label class="check"><input type="radio" name="size" checked=${prefs.size === sz}
          onChange=${() => savePrefs({ size: sz })} /> ${t('size.' + sz)}</label>`)}</div></fieldset>
      <fieldset><legend>${t('settings.a11y')}</legend>
        ${toggle('motion', t('a11y.motion'), t('a11y.motion_hint'))}
        ${toggle('underline', t('a11y.underline'))}
        ${toggle('readable', t('a11y.readable'), t('a11y.readable_hint'))}
        ${toggle('focus', t('a11y.focus'))}
      </fieldset>
    </div>

    <form class="card" onSubmit=${saveProfile}><h2>${t('settings.profile')}</h2>
      <div class="grid2">
        <${Field} label=${t('people.name')}><input ...${bind('display_name')} required maxlength="100" /><//>
        <${Field} label=${t('login.username')}><input value=${'@' + me.username} disabled /><//>
        <${Field} label=${t('people.email')}><input type="email" ...${bind('email')} autocomplete="email" /><//>
        <${Field} label=${t('people.phone')}><input type="tel" ...${bind('phone')} autocomplete="tel" /><//>
      </div>
      <label class="check"><input type="checkbox" ...${check('contact_visible')} />
        <span>${t('settings.contact_visible')}<br /><span class="muted small">${t('settings.contact_visible_hint')}</span></span></label>
      <div class="buttons"><button class="primary">${t('common.save')}</button></div>
    </form>

    <div class="card"><h2>${t('settings.security')}</h2>
      <h3>${t('settings.mfa')} ${me.totp_enabled ? html`<${Badge} kind="ok"><${Icon} name="check" /> ${t('common.on')}<//>` : html`<${Badge} kind="warn">${t('common.off')}<//>`}</h3>
      <p class="muted">${t('settings.mfa_text')}</p>
      ${newCodes && html`<${RecoveryCodes} codes=${newCodes} onDone=${() => { setNewCodes(null); loadSession(); }} />`}
      ${me.totp_enabled ? html`
        <p>${t('settings.recovery_left', { n: me.recovery_codes })}</p>
        <form class="row" onSubmit=${disableMfa}>
          <input type="password" placeholder=${t('settings.current_password')} aria-label=${t('settings.current_password')} value=${pw} onInput=${(e) => setPw(e.target.value)} required autocomplete="current-password" />
          <button type="button" onClick=${regenerate} disabled=${!pw}>${t('settings.recovery_new')}</button>
          ${me.mfa_required && me.passkeys === 0 ? html`<span class="muted small">${t('settings.mfa_required')}</span>` : html`<button class="danger">${t('settings.mfa_disable')}</button>`}
        </form>` : !newCodes && html`<${MfaSetup} onDone=${loadSession} />`}

      <h3 style=${{ marginTop: '1.4rem' }}>${t('settings.passkeys')} ${me.passkeys > 0 && html`<${Badge} kind="ok">${me.passkeys}<//>`}</h3>
      <p class="muted">${t('settings.passkeys_text')}</p>
      <${Passkeys} onChange=${loadSession} />

      <h3 style=${{ marginTop: '1.4rem' }}>${t('settings.sessions')}</h3>
      <p class="muted">${t('settings.sessions_text', { n: me.sessions })}${s.idleMinutes > 0 ? ' ' + t('settings.idle_text', { n: s.idleMinutes }) : ''}</p>
      <button onClick=${endOthers} disabled=${me.sessions < 2}><${Icon} name="logout" /> ${t('settings.sessions_end')}</button>

      <h3 style=${{ marginTop: '1.4rem' }}>${t('settings.change_password')}</h3>
      <${PasswordForm} />
    </div>

    <div class="card"><h2>${t('settings.privacy')}</h2>
      <p class="muted">${t('settings.export_text')}</p>
      <button onClick=${() => act(() => download('GET', '/me/export', undefined, 'samling-my-data.json'))}><${Icon} name="download" /> ${t('settings.export')}</button>
    </div>
  </div><//>`;
}
