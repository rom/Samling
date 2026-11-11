import { html, useState, useStore, S, setState, t, api, act, download, loadSession, Page, Field, useForm, Icon, Badge, themes, defaultPrefs,
  applyPrefs, locales, getLocale, setLocale } from '../lib.js';

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

export function MfaSetup({ onDone }) {
  const [setup, setSetup] = useState(null);
  const [code, setCode] = useState('');
  const start = async () => { const r = await act(() => api('POST', '/me/mfa/setup')); if (r) setSetup(r); };
  const enable = async (e) => {
    e.preventDefault();
    if (await act(() => api('POST', '/me/mfa/enable', { code }), t('settings.mfa_enabled'))) onDone();
  };
  if (!setup) return html`<button class="primary" onClick=${start}><${Icon} name="lock" /> ${t('settings.mfa_start')}</button>`;
  return html`<form onSubmit=${enable}>
    <p>${t('settings.mfa_scan')}</p>
    ${setup.qr && html`<div class="qr"><img src=${setup.qr} alt=${t('settings.mfa_qr_alt')} /></div>`}
    <p class="small">${t('settings.mfa_manual')}<br /><code class="secret">${setup.secret}</code></p>
    <p class="small"><a href=${setup.uri}>${t('settings.mfa_open_app')}</a></p>
    <${Field} label=${t('login.code')}><input value=${code} onInput=${(e) => setCode(e.target.value)} inputmode="numeric" autocomplete="one-time-code" maxlength="7" required /><//>
    <div class="buttons"><button class="primary">${t('settings.mfa_confirm')}</button></div>
  </form>`;
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
      ${me.totp_enabled
        ? (me.mfa_required ? html`<p class="muted">${t('settings.mfa_required')}</p>` : html`<form class="row" onSubmit=${disableMfa}>
            <input type="password" placeholder=${t('settings.current_password')} aria-label=${t('settings.current_password')} value=${pw} onInput=${(e) => setPw(e.target.value)} required autocomplete="current-password" />
            <button class="danger">${t('settings.mfa_disable')}</button></form>`)
        : html`<${MfaSetup} onDone=${loadSession} />`}
      <h3 style=${{ marginTop: '1.4rem' }}>${t('settings.change_password')}</h3>
      <${PasswordForm} />
    </div>

    <div class="card"><h2>${t('settings.privacy')}</h2>
      <p class="muted">${t('settings.export_text')}</p>
      <button onClick=${() => act(() => download('GET', '/me/export', undefined, 'samling-my-data.json'))}><${Icon} name="download" /> ${t('settings.export')}</button>
    </div>
  </div><//>`;
}
