import { html, useState, useStore, S, t, hasKey, api, act, download, useLoad, loadDirectory, can, roleName, fmtDateTime, Icon, Modal, Field, useForm, Page,
  Loading, Badge, confirmDelete, toast } from '../lib.js';
import { actionText } from './history.js';

const genPassword = () => {
  const a = new Uint8Array(15);
  crypto.getRandomValues(a);
  return btoa(String.fromCharCode(...a)).replace(/\+/g, '-').replace(/\//g, '_');
};

function RoleChecks({ value, onChange }) {
  const s = useStore();
  return html`<fieldset><legend>${t('admin.roles')}</legend>${s.roles.map((r) => html`<label class="check" key=${r.id}>
    <input type="checkbox" checked=${value.includes(r.id)} onChange=${(e) => onChange(e.target.checked ? [...value, r.id] : value.filter((x) => x !== r.id))} />
    <span>${roleName(r)}</span></label>`)}</fieldset>`;
}

function NewUser({ onClose }) {
  const [f, set, bind] = useForm({ username: '', display_name: '', email: '', password: genPassword() });
  const [roles, setRoles] = useState(['member']);
  const submit = async (e) => {
    e.preventDefault();
    if (await act(() => api('POST', '/admin/users', { ...f, roles }), t('admin.user_created'))) onClose(true);
  };
  return html`<${Modal} title=${t('admin.new_user')} onClose=${() => onClose(false)}><form onSubmit=${submit}>
    <${Field} label=${t('login.username')} hint=${t('admin.username_hint')}><input ...${bind('username')} required pattern="[a-z0-9][a-z0-9._\\-]{1,31}" autocapitalize="none" /><//>
    <${Field} label=${t('people.name')}><input ...${bind('display_name')} maxlength="100" /><//>
    <${Field} label=${t('people.email')}><input type="email" ...${bind('email')} /><//>
    <${Field} group label=${t('admin.temp_password')} hint=${t('admin.temp_password_hint')}>
      <div class="row"><input class="grow" ...${bind('password')} aria-label=${t('admin.temp_password')} required minlength=${S.minPassword} autocomplete="off" />
        <button type="button" onClick=${() => set('password', genPassword())}>${t('admin.generate')}</button></div><//>
    ${can('roles.manage') && html`<${RoleChecks} value=${roles} onChange=${setRoles} />`}
    <div class="buttons"><button type="button" onClick=${() => onClose(false)}>${t('common.cancel')}</button><button class="primary">${t('common.create')}</button></div>
  </form><//>`;
}

function EditUser({ user, onClose }) {
  const [f, , bind, check] = useForm({ username: user.username, display_name: user.display_name, email: user.email, disabled: user.disabled, mfa_required: user.mfa_required });
  const [roles, setRoles] = useState(user.roles);
  const [pw, setPw] = useState('');
  const submit = async (e) => {
    e.preventDefault();
    const body = { ...f };
    if (can('roles.manage')) body.roles = roles;
    if (await act(() => api('PATCH', '/admin/users/' + user.id, body), t('common.saved'))) onClose(true);
  };
  const action = async (body, msg) => { if (await act(() => api('PATCH', '/admin/users/' + user.id, body), msg)) onClose(true); };
  const signOut = async () => { if (await act(() => api('POST', `/admin/users/${user.id}/sign-out`), t('admin.signed_out'))) onClose(true); };
  const setPassword = async () => { if (await act(() => api('POST', `/admin/users/${user.id}/password`, { password: pw }), t('admin.password_set'))) onClose(true); };
  const del = async () => { if (window.confirm(t('admin.confirm_delete_user', { name: user.username })) && await act(() => api('DELETE', '/admin/users/' + user.id), t('admin.user_deleted'))) onClose(true); };
  return html`<${Modal} title=${t('admin.edit_user') + ': @' + user.username} onClose=${() => onClose(false)} wide>
    <form onSubmit=${submit}>
      <div class="grid2">
        <${Field} label=${t('login.username')} hint=${t('admin.rename_hint')}><input ...${bind('username')} required autocapitalize="none" /><//>
        <${Field} label=${t('people.name')}><input ...${bind('display_name')} /><//>
        <${Field} label=${t('people.email')}><input type="email" ...${bind('email')} /><//>
      </div>
      ${can('roles.manage') && html`<${RoleChecks} value=${roles} onChange=${setRoles} />`}
      <label class="check"><input type="checkbox" ...${check('mfa_required')} /><span>${t('admin.mfa_required')}<br /><span class="muted small">${t('admin.mfa_required_hint')}</span></span></label>
      <label class="check"><input type="checkbox" ...${check('disabled')} /><span>${t('admin.disabled')}<br /><span class="muted small">${t('admin.disabled_hint')}</span></span></label>
      <div class="buttons"><button class="primary">${t('common.save')}</button></div>
    </form>
    <fieldset><legend>${t('admin.password')}</legend>
      <div class="row"><input class="grow" value=${pw} onInput=${(e) => setPw(e.target.value)} aria-label=${t('admin.temp_password')} placeholder=${t('admin.temp_password')} autocomplete="off" />
        <button type="button" onClick=${() => setPw(genPassword())}>${t('admin.generate')}</button>
        <button type="button" disabled=${pw.length < S.minPassword} onClick=${setPassword}>${t('admin.set_password')}</button></div>
      <p class="muted small">${t('admin.temp_password_hint')}</p></fieldset>
    <fieldset><legend>${t('admin.account_actions')}</legend><div class="row">
      <button type="button" disabled=${user.sessions === 0} onClick=${signOut}><${Icon} name="logout" /> ${t('admin.sign_out', { n: user.sessions })}</button>
      ${(user.totp_enabled || user.passkeys > 0) && html`<button type="button" onClick=${() => action({ reset_mfa: true }, t('admin.mfa_reset_done'))}>${t('admin.reset_mfa')}</button>`}
      <button type="button" onClick=${() => action({ unlock: true }, t('admin.unlocked'))}>${t('admin.unlock')}</button>
      <button type="button" class="danger" onClick=${del}><${Icon} name="trash" /> ${t('admin.delete_user')}</button></div>
      <p class="muted small">${t('admin.delete_hint')}</p></fieldset>
  <//>`;
}

function Accounts() {
  const st = useLoad(() => api('GET', '/admin/users'));
  const [modal, setModal] = useState(null);
  const s = useStore();
  const roles = Object.fromEntries(s.roles.map((r) => [r.id, r]));
  if (!st.data) return html`<${Loading} state=${st} />`;
  const done = (changed) => { setModal(null); if (changed) { st.reload(); loadDirectory(); } };
  return html`<div class="row" style=${{ marginBottom: '.8rem' }}><span class="grow muted">${t('admin.user_count', { n: st.data.length })}</span>
      <button class="primary" onClick=${() => setModal({ create: true })}><${Icon} name="plus" /> ${t('admin.new_user')}</button></div>
    <div class="table-wrap"><table><thead><tr><th>${t('login.username')}</th><th>${t('people.name')}</th><th>${t('admin.roles')}</th><th>${t('settings.mfa')}</th><th>${t('admin.last_login')}</th><th></th></tr></thead>
      <tbody>${st.data.map((u) => html`<tr key=${u.id}><td>@${u.username} ${u.disabled && html`<${Badge} kind="danger">${t('admin.disabled')}<//>`}</td>
        <td>${u.display_name}<br /><span class="muted small">${u.email}</span></td>
        <td>${u.roles.map((r) => html`<${Badge} kind=${r === 'admin' ? 'info' : ''}>${roles[r] ? roleName(roles[r]) : r}<//> `)}</td>
        <td>${u.totp_enabled || u.passkeys > 0 ? html`<${Badge} kind="ok">${u.totp_enabled ? t('common.on') : t('admin.passkey')}<//>` : u.mfa_required ? html`<${Badge} kind="warn">${t('admin.mfa_pending')}<//>` : html`<span class="muted">${t('common.off')}</span>`}</td>
        <td class="nowrap small">${u.last_login_at ? fmtDateTime(u.last_login_at) : '–'}</td>
        <td><button onClick=${() => setModal({ user: u })}><${Icon} name="edit" /> ${t('common.edit')}<span class="sr-only"> @${u.username}</span></button></td></tr>`)}</tbody></table></div>
    ${modal && modal.create && html`<${NewUser} onClose=${done} />`}
    ${modal && modal.user && html`<${EditUser} user=${modal.user} onClose=${done} />`}`;
}

function RoleForm({ role, onClose }) {
  const s = useStore();
  const [f, , bind] = useForm({ name: role ? role.name : '', description: role ? role.description : '' });
  const [perms, setPerms] = useState(role ? role.perms : []);
  const submit = async (e) => {
    e.preventDefault();
    const body = { ...f, perms };
    const ok = role ? await act(() => api('PUT', '/roles/' + role.id, body), t('common.saved')) : await act(() => api('POST', '/roles', body), t('common.saved'));
    if (ok) onClose(true);
  };
  const del = async () => { if (confirmDelete(role.name) && await act(() => api('DELETE', '/roles/' + role.id))) onClose(true); };
  const lockedPerms = role && role.id === 'admin';
  return html`<${Modal} title=${role ? t('admin.edit_role') : t('admin.new_role')} onClose=${() => onClose(false)}><form onSubmit=${submit}>
    <${Field} label=${t('common.name')}><input ...${bind('name')} required maxlength="60" disabled=${role && role.builtin} /><//>
    <${Field} label=${t('common.description')}><input ...${bind('description')} maxlength="300" disabled=${role && role.builtin} /><//>
    <fieldset><legend>${t('admin.permissions')}</legend>${(s.permissions || []).map((p) => html`<label class="check" key=${p}>
      <input type="checkbox" checked=${perms.includes(p)} disabled=${lockedPerms} onChange=${(e) => setPerms(e.target.checked ? [...perms, p] : perms.filter((x) => x !== p))} />
      <span>${t('perm.' + p)}<br /><span class="muted small">${t('perm.' + p + '.hint')}</span></span></label>`)}</fieldset>
    <div class="buttons">${role && !role.builtin && html`<button type="button" class="danger" onClick=${del}><${Icon} name="trash" /> ${t('common.delete')}</button>`}
      <button type="button" onClick=${() => onClose(false)}>${t('common.cancel')}</button><button class="primary" disabled=${lockedPerms}>${t('common.save')}</button></div>
  </form><//>`;
}

function Roles() {
  const s = useStore();
  const [modal, setModal] = useState(null);
  const done = (changed) => { setModal(null); if (changed) loadDirectory(); };
  return html`<div class="row" style=${{ marginBottom: '.8rem' }}><span class="grow muted">${t('admin.roles_text')}</span>
      <button class="primary" onClick=${() => setModal({})}><${Icon} name="plus" /> ${t('admin.new_role')}</button></div>
    <div class="cards">${s.roles.map((r) => html`<div class="card" key=${r.id}><div class="row"><h3 class="grow">${roleName(r)}</h3>
        ${r.builtin && html`<${Badge}>${t('admin.builtin')}<//>`}
        <button class="icon-btn" onClick=${() => setModal({ role: r })} aria-label=${t('common.edit') + ': ' + roleName(r)}><${Icon} name="edit" /></button></div>
      <p class="small">${r.perms.length === 0 ? html`<span class="muted">${t('admin.no_perms')}</span>` : r.perms.map((p) => html`<${Badge} kind="info">${t('perm.' + p)}<//> `)}</p></div>`)}</div>
    <p class="muted small" style=${{ marginTop: '1rem' }}>${t('admin.groups_hint')} <a href="#/people">${t('nav.people')}</a></p>
    ${modal && html`<${RoleForm} role=${modal.role} onClose=${done} />`}`;
}

function Logs() {
  const [action, setAction] = useState('');
  const st = useLoad(() => api('GET', '/admin/logs' + (action ? '?action=' + encodeURIComponent(action) : '')), [action]);
  const groups = ['', 'login', 'user.', 'role.', 'group.', 'mfa.', 'password.', 'backup.', 'export.', 'sharing.', 'decision.', 'meeting.'];
  return html`<div class="row" style=${{ marginBottom: '.8rem' }}><label>${t('admin.log_filter')} <select value=${action} onChange=${(e) => setAction(e.target.value)}>
      ${groups.map((g) => html`<option value=${g}>${g ? g.replace(/\.$/, '') : t('history.all')}</option>`)}</select></label>
      <button onClick=${st.reload}>${t('common.refresh')}</button></div>
    <p class="muted small">${t('admin.log_text')}</p>
    ${!st.data ? html`<${Loading} state=${st} />` : html`<div class="table-wrap"><table>
      <thead><tr><th>${t('common.when')}</th><th>${t('common.who')}</th><th>${t('admin.event')}</th><th>${t('admin.details')}</th><th>${t('admin.ip')}</th></tr></thead>
      <tbody>${st.data.map((l) => html`<tr key=${l.id}><td class="nowrap small">${fmtDateTime(l.created_at)}</td><td>${l.username ? '@' + l.username : '–'}</td>
        <td>${l.action === 'login.failed' ? html`<${Badge} kind="danger">${actionText(l.action)}<//>` : actionText(l.action)}</td>
        <td class="small">${l.summary}</td><td class="small nowrap">${l.ip}</td></tr>`)}
        ${st.data.length === 0 && html`<tr><td colspan="5" class="muted">${t('common.nothing')}</td></tr>`}</tbody></table></div>`}`;
}

function Backup() {
  const [f, , bind, , setF] = useForm({ pass: '', repeat: '' });
  const [ex, setEx] = useState('');
  const [busy, setBusy] = useState(false);
  const backup = async (e) => {
    e.preventDefault();
    if (f.pass !== f.repeat) { toast(t('admin.pass_mismatch'), 'error'); return; }
    setBusy(true);
    if (await act(() => download('POST', '/admin/backup', { passphrase: f.pass }, 'samling-backup.bak'), t('admin.backup_done'))) setF({ pass: '', repeat: '' });
    setBusy(false);
  };
  const members = (e) => { e.preventDefault(); act(() => download('POST', '/admin/export', { kind: 'members', passphrase: ex }, 'members.csv')); };
  return html`<div class="stack">
    <form class="card" onSubmit=${backup}><h2>${t('admin.backup')}</h2><p class="muted">${t('admin.backup_text')}</p>
      <div class="grid2">
        <${Field} label=${t('admin.passphrase')} hint=${t('admin.passphrase_hint')}><input type="password" ...${bind('pass')} required minlength="12" autocomplete="new-password" /><//>
        <${Field} label=${t('admin.passphrase_repeat')}><input type="password" ...${bind('repeat')} required autocomplete="new-password" /><//></div>
      <div class="buttons"><button class="primary" disabled=${busy}><${Icon} name="download" /> ${busy ? t('common.working') : t('admin.backup_create')}</button></div>
      <p class="muted small">${t('admin.restore_text')} <code>samling restore --in FILE</code></p></form>
    ${can('users.manage') && html`<form class="card" onSubmit=${members}><h2>${t('admin.export_members')}</h2><p class="muted">${t('admin.export_members_text')}</p>
      <${Field} label=${t('admin.passphrase') + ' (' + t('common.optional') + ')'} hint=${t('admin.export_pass_hint')}>
        <input type="password" value=${ex} onInput=${(e) => setEx(e.target.value)} minlength="12" autocomplete="new-password" /><//>
      <div class="buttons"><button><${Icon} name="download" /> ${t('admin.export_download')}</button></div></form>`}
  </div>`;
}

export function Admin() {
  const tabs = [['accounts', 'users.manage', Accounts], ['roles', 'roles.manage', Roles], ['logs', 'logs.view', Logs], ['backup', 'backup.manage', Backup]].filter((x) => can(x[1]));
  const [tab, setTab] = useState(tabs.length ? tabs[0][0] : '');
  const cur = tabs.find((x) => x[0] === tab);
  if (!cur) return html`<${Page} title=${t('nav.admin')}><p>${hasKey('err.forbidden') ? t('err.forbidden') : ''}</p><//>`;
  const View = cur[2];
  return html`<${Page} title=${t('nav.admin')}>
    <div class="tabs" role="tablist">${tabs.map(([k]) => html`<button role="tab" aria-selected=${tab === k} onClick=${() => setTab(k)}>${t('admin.tab_' + k)}</button>`)}</div>
    <${View} />
  <//>`;
}
