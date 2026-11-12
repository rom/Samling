import { html, useState, useStore, t, api, act, loadDirectory, go, can, userName, roleName, Icon, Modal, Field, useForm, Page, Empty, Badge, confirmDelete } from '../lib.js';

function GroupForm({ group, scope, onClose }) {
  const s = useStore();
  const [f, , bind] = useForm({ name: group ? group.name : '', description: group ? group.description : '' });
  const [members, setMembers] = useState(group ? group.members : []);
  const [q, setQ] = useState('');
  const submit = async (e) => {
    e.preventDefault();
    const body = { ...f, members, scope: group ? group.scope : scope };
    const ok = group ? await act(() => api('PUT', '/groups/' + group.id, body), t('common.saved')) : await act(() => api('POST', '/groups', body), t('common.saved'));
    if (ok) onClose(true);
  };
  const del = async () => { if (confirmDelete(group.name) && await act(() => api('DELETE', '/groups/' + group.id))) onClose(true); };
  const people = s.users.filter((u) => (u.display_name + u.username).toLowerCase().includes(q.toLowerCase()));
  const sc = group ? group.scope : scope;
  return html`<${Modal} title=${group ? t('people.edit_group') : t('people.new_group_' + sc)} onClose=${() => onClose(false)}><form onSubmit=${submit}>
    <p class="muted small">${t('people.scope_hint_' + sc)}</p>
    <${Field} label=${t('common.name')}><input ...${bind('name')} required maxlength="80" /><//>
    <${Field} label=${t('common.description')}><input ...${bind('description')} maxlength="300" /><//>
    <${Field} label=${t('people.members') + ' (' + members.length + ')'}><input value=${q} onInput=${(e) => setQ(e.target.value)} placeholder=${t('common.filter')} /><//>
    <div class="rows" style=${{ maxHeight: '16rem', overflowY: 'auto' }}>${people.map((u) => html`<label key=${u.id}>
      <input type="checkbox" checked=${members.includes(u.id)} onChange=${(e) => setMembers(e.target.checked ? [...members, u.id] : members.filter((x) => x !== u.id))} />
      <span>${u.display_name} <span class="muted small">@${u.username}</span></span></label>`)}</div>
    <div class="buttons">${group && html`<button type="button" class="danger" onClick=${del}><${Icon} name="trash" /> ${t('common.delete')}</button>`}
      <button type="button" onClick=${() => onClose(false)}>${t('common.cancel')}</button><button class="primary">${t('common.save')}</button></div>
  </form><//>`;
}

export function People() {
  const s = useStore();
  const [tab, setTab] = useState('people');
  const [q, setQ] = useState('');
  const [modal, setModal] = useState(null);
  const roles = Object.fromEntries(s.roles.map((r) => [r.id, r]));
  const people = s.users.filter((u) => (u.display_name + ' ' + u.username + ' ' + (u.email || '') + ' ' + (u.phone || '')).toLowerCase().includes(q.toLowerCase()));
  const dm = async (u) => { const r = await act(() => api('POST', '/dm', { user_ids: [u.id] })); if (r) go('chat', r.id); };
  const done = (changed) => { setModal(null); if (changed) loadDirectory(); };
  const groupCard = (g) => html`<div class="card" key=${g.id}><div class="row"><h3 class="grow">${g.name}</h3>
      ${g.can_edit && html`<button class="icon-btn" onClick=${() => setModal({ group: g })} aria-label=${t('common.edit') + ': ' + g.name}><${Icon} name="edit" /></button>`}</div>
    ${g.description && html`<p class="muted small">${g.description}</p>`}
    <p class="small">${g.members.length === 0 ? html`<span class="muted">${t('people.no_members')}</span>` : g.members.map(userName).join(', ')}</p></div>`;
  const global = s.groups.filter((g) => g.scope === 'global'), local = s.groups.filter((g) => g.scope === 'local');
  return html`<${Page} title=${t('nav.people')}>
    <div class="tabs" role="tablist">
      <button role="tab" aria-selected=${tab === 'people'} onClick=${() => setTab('people')}>${t('people.directory')} (${s.users.length})</button>
      <button role="tab" aria-selected=${tab === 'groups'} onClick=${() => setTab('groups')}>${t('people.groups')} (${s.groups.length})</button></div>
    ${tab === 'people' && html`
      <input type="search" style=${{ width: '100%', marginBottom: '1rem' }} placeholder=${t('people.search')} aria-label=${t('people.search')} value=${q} onInput=${(e) => setQ(e.target.value)} />
      ${people.length === 0 ? html`<${Empty} icon="people" text=${t('common.nothing')} />` : html`<div class="rows">${people.map((u) => html`<div key=${u.id}>
        <span class="avatar" aria-hidden="true">${u.display_name.slice(0, 1).toUpperCase()}</span>
        <span class="grow"><b>${u.display_name}</b> <span class="muted small">@${u.username}</span>
          ${u.roles.filter((r) => r !== 'member' && roles[r]).map((r) => html` <${Badge} kind="info">${roleName(roles[r])}<//>`)}<br />
          <span class="small">${u.email && html`<a href=${'mailto:' + encodeURIComponent(u.email).replace(/%40/g, '@')}><${Icon} name="mail" /> ${u.email}</a> `}${u.phone && html`<a href=${'tel:' + u.phone.replace(/[^\d+]/g, '')}>${u.phone}</a>`}
            ${!u.email && !u.phone && html`<span class="muted">${t('people.no_contact')}</span>`}</span></span>
        ${u.id !== s.me.id && html`<button onClick=${() => dm(u)}><${Icon} name="chat" /> <span class="sr-only">${u.display_name}: </span>${t('people.message')}</button>`}
      </div>`)}</div>`}`}
    ${tab === 'groups' && html`
      <div class="row" style=${{ marginBottom: '.6rem' }}><h2 class="grow">${t('people.global_groups')}</h2>
        ${can('roles.manage') && html`<button onClick=${() => setModal({ scope: 'global' })}><${Icon} name="plus" /> ${t('people.new_group_global')}</button>`}</div>
      <p class="muted small">${t('people.scope_hint_global')}</p>
      ${global.length === 0 ? html`<p class="muted">${t('people.no_groups')}</p>` : html`<div class="cards">${global.map(groupCard)}</div>`}
      <div class="row" style=${{ margin: '1.6rem 0 .6rem' }}><h2 class="grow">${t('people.local_groups')}</h2>
        <button onClick=${() => setModal({ scope: 'local' })}><${Icon} name="plus" /> ${t('people.new_group_local')}</button></div>
      <p class="muted small">${t('people.scope_hint_local')}</p>
      ${local.length === 0 ? html`<p class="muted">${t('people.no_groups')}</p>` : html`<div class="cards">${local.map(groupCard)}</div>`}`}
    ${modal && html`<${GroupForm} group=${modal.group} scope=${modal.scope} onClose=${done} />`}
  <//>`;
}
