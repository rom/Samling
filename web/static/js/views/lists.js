import { html, useState, t, api, act, useLoad, useLive, go, userName, fmtDate, fmtDateTime, toDateInput, fromDateInput, Icon, Modal, Field, useForm,
  ShareEditor, ShareButton, SharedBadge, Page, Empty, Loading, confirmDelete } from '../lib.js';

const kinds = ['todo', 'priority', 'plain'];

function NewList({ onClose }) {
  const [f, , bind] = useForm({ title: '', kind: 'todo' });
  const [shares, setShares] = useState([]);
  const submit = async (e) => {
    e.preventDefault();
    const r = await act(() => api('POST', '/lists', { ...f, shares }));
    if (r) { onClose(); go('lists', r.id); }
  };
  return html`<${Modal} title=${t('lists.new')} onClose=${onClose}><form onSubmit=${submit}>
    <${Field} label=${t('common.title')}><input ...${bind('title')} required maxlength="120" /><//>
    <${Field} label=${t('lists.kind')}><select ...${bind('kind')}>${kinds.map((k) => html`<option value=${k}>${t('lists.kind_' + k)}</option>`)}</select><//>
    <${Field} group label=${t('share.who')}><${ShareEditor} value=${shares} onChange=${setShares} /><//>
    <div class="buttons"><button type="button" onClick=${onClose}>${t('common.cancel')}</button><button class="primary">${t('common.create')}</button></div>
  </form><//>`;
}

function Detail({ id }) {
  const st = useLoad(() => api('GET', '/lists/' + id), [id]);
  const [text, setText] = useState('');
  const [prio, setPrio] = useState(0);
  const [due, setDue] = useState('');
  useLive(['list'], (e) => { if (e.id === id) st.reload(); });
  if (!st.data) return html`<${Page} title=${t('nav.lists')} back="#/lists"><${Loading} state=${st} /><//>`;
  const l = st.data;
  const canEdit = l.level >= 2;
  let items = l.items;
  if (l.kind === 'priority') items = [...items].sort((a, b) => a.done - b.done || b.priority - a.priority || a.position - b.position);
  const add = async (e) => {
    e.preventDefault();
    if (!text.trim()) return;
    if (await act(() => api('POST', `/lists/${id}/items`, { text, priority: +prio, due_at: fromDateInput(due) }))) { setText(''); setDue(''); st.reload(); }
  };
  const patch = async (item, body) => { if (await act(() => api('PATCH', `/lists/${id}/items/${item.id}`, body))) st.reload(); };
  const remove = async (item) => { if (await act(() => api('DELETE', `/lists/${id}/items/${item.id}`))) st.reload(); };
  const del = async () => { if (confirmDelete(l.title) && await act(() => api('DELETE', '/lists/' + id))) go('lists'); };
  const move = (item, delta) => patch(item, { position: l.items.findIndex((x) => x.id === item.id) + delta });
  const now = Date.now() / 1000;
  return html`<${Page} title=${l.title} back="#/lists" actions=${l.level >= 3 && html`<${ShareButton} type="list" id=${id} onDone=${st.reload} />
      <button class="danger" onClick=${del}><${Icon} name="trash" /> ${t('common.delete')}</button>`}>
    <p class="muted">${t('lists.kind_' + l.kind)} · ${t('docs.owner')}: ${userName(l.owner_id)}${!canEdit ? ' · ' + t('share.read_only') : ''}</p>
    <div class="card">
      ${items.length === 0 ? html`<${Empty} icon="list" text=${t('lists.empty_items')} />` : html`<ul class="items">
        ${items.map((it, i) => html`<li key=${it.id} class=${it.done ? 'done' : ''}>
          ${l.kind !== 'plain' && html`<input type="checkbox" checked=${it.done} disabled=${!canEdit} aria-label=${t('lists.done') + ': ' + it.text} onChange=${(e) => patch(it, { done: e.target.checked })} />`}
          ${l.kind === 'priority' && html`<span class=${'prio p' + it.priority} title=${t('lists.prio_' + it.priority)} role="img" aria-label=${t('lists.prio_' + it.priority)}></span>`}
          <span class="text">${it.text}${it.due_at > 0 && html` <span class=${'badge ' + (!it.done && it.due_at < now ? 'danger' : '')}><${Icon} name="calendar" /> ${fmtDate(it.due_at, true)}</span>`}</span>
          ${canEdit && html`
            ${l.kind === 'priority' && html`<select aria-label=${t('lists.priority')} value=${it.priority} onChange=${(e) => patch(it, { priority: +e.target.value })}>
              ${[0, 1, 2, 3].map((p) => html`<option value=${p}>${t('lists.prio_' + p)}</option>`)}</select>`}
            ${l.kind !== 'priority' && html`<button class="icon-btn" disabled=${i === 0} onClick=${() => move(it, -1)} aria-label=${t('common.move_up')}><${Icon} name="up" /></button>
              <button class="icon-btn" disabled=${i === items.length - 1} onClick=${() => move(it, 1)} aria-label=${t('common.move_down')}><${Icon} name="down" /></button>`}
            <button class="icon-btn" onClick=${() => remove(it)} aria-label=${t('common.delete') + ': ' + it.text}><${Icon} name="trash" /></button>`}
        </li>`)}</ul>`}
      ${canEdit && html`<form class="add-line" onSubmit=${add}>
        <input type="text" value=${text} onInput=${(e) => setText(e.target.value)} placeholder=${t('lists.add_placeholder')} aria-label=${t('lists.add_placeholder')} maxlength="500" />
        ${l.kind === 'priority' && html`<select aria-label=${t('lists.priority')} value=${prio} onChange=${(e) => setPrio(e.target.value)}>
          ${[0, 1, 2, 3].map((p) => html`<option value=${p}>${t('lists.prio_' + p)}</option>`)}</select>`}
        ${l.kind !== 'plain' && html`<input type="date" value=${due} onInput=${(e) => setDue(e.target.value)} aria-label=${t('lists.due')} title=${t('lists.due')} />`}
        <button class="primary"><${Icon} name="plus" /> ${t('common.add')}</button></form>`}
    </div>
  <//>`;
}

export function Lists({ route }) {
  const st = useLoad(() => api('GET', '/lists'));
  const [open, setOpen] = useState(false);
  useLive(['list'], st.reload);
  if (route[1]) return html`<${Detail} id=${route[1]} key=${route[1]} />`;
  if (!st.data) return html`<${Page} title=${t('nav.lists')}><${Loading} state=${st} /><//>`;
  return html`<${Page} title=${t('nav.lists')} actions=${html`<button class="primary" onClick=${() => setOpen(true)}><${Icon} name="plus" /> ${t('lists.new')}</button>`}>
    ${st.data.length === 0 ? html`<${Empty} icon="list" text=${t('lists.empty')} />` : html`<div class="cards">
      ${st.data.map((l) => html`<a class="card" href=${'#/lists/' + l.id} key=${l.id}><h3>${l.title}</h3>
        <div class="row"><${SharedBadge} shared=${l.shared} /><span class="badge">${t('lists.kind_' + l.kind)}</span></div>
        <p class="muted small">${l.kind === 'plain' ? t('lists.count', { n: l.total }) : t('lists.open_count', { open: l.open, total: l.total })} · ${fmtDateTime(l.updated_at)}</p></a>`)}</div>`}
    ${open && html`<${NewList} onClose=${() => setOpen(false)} />`}
  <//>`;
}

export { toDateInput };
