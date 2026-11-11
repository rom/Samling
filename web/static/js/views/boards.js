import { html, useState, useStore, t, api, act, useLoad, useLive, go, userName, fmtDate, fmtDateTime, toDateInput, fromDateInput, Icon, Modal, Field, useForm,
  ShareEditor, ShareButton, SharedBadge, Page, Empty, Loading, confirmDelete } from '../lib.js';

function NewBoard({ onClose }) {
  const [f, , bind] = useForm({ title: '', description: '', columns: t('boards.default_columns') });
  const [shares, setShares] = useState([]);
  const submit = async (e) => {
    e.preventDefault();
    const r = await act(() => api('POST', '/boards', { title: f.title, description: f.description, columns: f.columns.split(',').map((x) => x.trim()).filter(Boolean), shares }));
    if (r) { onClose(); go('boards', r.id); }
  };
  return html`<${Modal} title=${t('boards.new')} onClose=${onClose}><form onSubmit=${submit}>
    <${Field} label=${t('common.title')}><input ...${bind('title')} required maxlength="120" /><//>
    <${Field} label=${t('common.description')}><textarea ...${bind('description')}></textarea><//>
    <${Field} label=${t('boards.columns')} hint=${t('boards.columns_hint')}><input ...${bind('columns')} required /><//>
    <${Field} group label=${t('share.who')}><${ShareEditor} value=${shares} onChange=${setShares} /><//>
    <div class="buttons"><button type="button" onClick=${onClose}>${t('common.cancel')}</button><button class="primary">${t('common.create')}</button></div>
  </form><//>`;
}

function CardForm({ board, card, columnId, onClose }) {
  const s = useStore();
  const [f, , bind] = useForm(card ? { title: card.title, description: card.description, assignee_id: card.assignee_id, due: toDateInput(card.due_at), column_id: card.column_id }
    : { title: '', description: '', assignee_id: '', due: '', column_id: columnId });
  const body = () => ({ title: f.title, description: f.description, assignee_id: f.assignee_id, due_at: fromDateInput(f.due), column_id: f.column_id });
  const submit = async (e) => {
    e.preventDefault();
    const ok = card ? await act(() => api('PATCH', `/boards/${board.id}/cards/${card.id}`, body())) : await act(() => api('POST', `/boards/${board.id}/cards`, body()));
    if (ok) onClose(true);
  };
  const del = async () => { if (confirmDelete(card.title) && await act(() => api('DELETE', `/boards/${board.id}/cards/${card.id}`))) onClose(true); };
  const ro = board.level < 2;
  return html`<${Modal} title=${card ? t('boards.card') : t('boards.new_card')} onClose=${() => onClose(false)}><form onSubmit=${submit}>
    <${Field} label=${t('common.title')}><input ...${bind('title')} required maxlength="200" disabled=${ro} /><//>
    <${Field} label=${t('common.description')}><textarea ...${bind('description')} rows="4" disabled=${ro}></textarea><//>
    <div class="grid2">
      <${Field} label=${t('boards.column')}><select ...${bind('column_id')} disabled=${ro}>${board.columns.map((c) => html`<option value=${c.id}>${c.title}</option>`)}</select><//>
      <${Field} label=${t('boards.assignee')}><select ...${bind('assignee_id')} disabled=${ro}><option value="">–</option>
        ${s.users.map((u) => html`<option value=${u.id}>${u.display_name}</option>`)}</select><//>
      <${Field} label=${t('lists.due')}><input type="date" ...${bind('due')} disabled=${ro} /><//>
    </div>
    ${card && html`<p class="muted small">${t('boards.created_by', { name: userName(card.created_by) })} · ${fmtDateTime(card.updated_at)}</p>`}
    <div class="buttons">${card && !ro && html`<button type="button" class="danger" onClick=${del}><${Icon} name="trash" /> ${t('common.delete')}</button>`}
      <button type="button" onClick=${() => onClose(false)}>${t('common.close')}</button>${!ro && html`<button class="primary">${t('common.save')}</button>`}</div>
  </form><//>`;
}

function Detail({ id }) {
  const st = useLoad(() => api('GET', '/boards/' + id), [id]);
  const [modal, setModal] = useState(null);
  const [drag, setDrag] = useState(null);
  const [over, setOver] = useState(null);
  useLive(['board'], (e) => { if (e.id === id) st.reload(); });
  if (!st.data) return html`<${Page} title=${t('nav.boards')} back="#/boards"><${Loading} state=${st} /><//>`;
  const b = st.data;
  const canEdit = b.level >= 2, canManage = b.level >= 3;
  const cardsOf = (col) => b.cards.filter((k) => k.column_id === col.id).sort((x, y) => x.position - y.position);
  const moveCard = async (cardId, colId, position) => {
    const body = { column_id: colId };
    if (position !== undefined) body.position = position;
    if (await act(() => api('PATCH', `/boards/${id}/cards/${cardId}`, body))) st.reload();
  };
  const drop = (e, col, position) => {
    e.preventDefault();
    e.stopPropagation();
    setOver(null);
    const cardId = e.dataTransfer.getData('text/plain') || drag;
    if (cardId) moveCard(cardId, col.id, position);
    setDrag(null);
  };
  const addColumn = async () => {
    const title = window.prompt(t('boards.column_name'));
    if (title && await act(() => api('POST', `/boards/${id}/columns`, { title }))) st.reload();
  };
  const renameColumn = async (col) => {
    const title = window.prompt(t('boards.column_name'), col.title);
    if (title && await act(() => api('PATCH', `/boards/${id}/columns/${col.id}`, { title }))) st.reload();
  };
  const shiftColumn = async (col, i, d) => { if (await act(() => api('PATCH', `/boards/${id}/columns/${col.id}`, { position: i + d }))) st.reload(); };
  const delColumn = async (col) => { if (confirmDelete(col.title) && await act(() => api('DELETE', `/boards/${id}/columns/${col.id}`))) st.reload(); };
  const del = async () => { if (confirmDelete(b.title) && await act(() => api('DELETE', '/boards/' + id))) go('boards'); };
  const now = Date.now() / 1000;
  return html`<${Page} title=${b.title} back="#/boards" actions=${canManage && html`
      <button onClick=${addColumn}><${Icon} name="plus" /> ${t('boards.add_column')}</button>
      <${ShareButton} type="board" id=${id} onDone=${st.reload} /><button class="danger" onClick=${del}><${Icon} name="trash" /> ${t('common.delete')}</button>`}>
    ${b.description && html`<p class="muted pre">${b.description}</p>`}
    <div class="kanban">${b.columns.map((col, ci) => {
      const cards = cardsOf(col);
      return html`<section class=${'kcol' + (over === col.id ? ' over' : '')} key=${col.id} aria-label=${col.title}
          onDragOver=${(e) => { if (canEdit) { e.preventDefault(); setOver(col.id); } }} onDragLeave=${() => setOver(null)} onDrop=${(e) => drop(e, col)}>
        <header><span class="grow">${col.title} <span class="muted small">${cards.length}</span></span>
          ${canManage && html`
            <button class="icon-btn" disabled=${ci === 0} onClick=${() => shiftColumn(col, ci, -1)} aria-label=${t('boards.move_left') + ': ' + col.title}><${Icon} name="left" /></button>
            <button class="icon-btn" disabled=${ci === b.columns.length - 1} onClick=${() => shiftColumn(col, ci, 1)} aria-label=${t('boards.move_right') + ': ' + col.title}><${Icon} name="right" /></button>
            <button class="icon-btn" onClick=${() => renameColumn(col)} aria-label=${t('common.edit') + ': ' + col.title}><${Icon} name="edit" /></button>
            ${cards.length === 0 && html`<button class="icon-btn" onClick=${() => delColumn(col)} aria-label=${t('common.delete') + ': ' + col.title}><${Icon} name="trash" /></button>`}`}
        </header>
        ${cards.map((k, i) => html`<div key=${k.id} onDrop=${(e) => drop(e, col, i)} onDragOver=${(e) => { if (canEdit) e.preventDefault(); }}>
          <button class=${'kcard' + (drag === k.id ? ' dragging' : '')} draggable=${canEdit} onClick=${() => setModal({ card: k })}
              onDragStart=${(e) => { e.dataTransfer.setData('text/plain', k.id); e.dataTransfer.effectAllowed = 'move'; setDrag(k.id); }} onDragEnd=${() => { setDrag(null); setOver(null); }}>
            <div class="title">${k.title}</div>
            <div class="meta">${k.assignee_id && html`<span><${Icon} name="people" /> ${userName(k.assignee_id)}</span>`}
              ${k.due_at > 0 && html`<span class=${k.due_at < now ? 'error-text' : ''}><${Icon} name="calendar" /> ${fmtDate(k.due_at, true)}</span>`}</div>
          </button>
          ${canEdit && html`<div class="row small no-print" style=${{ margin: '-.3rem 0 .5rem', justifyContent: 'flex-end' }}>
            <button class="icon-btn" disabled=${ci === 0} onClick=${() => moveCard(k.id, b.columns[ci - 1].id)} aria-label=${t('boards.move_left') + ': ' + k.title}><${Icon} name="left" /></button>
            <button class="icon-btn" disabled=${i === 0} onClick=${() => moveCard(k.id, col.id, i - 1)} aria-label=${t('common.move_up') + ': ' + k.title}><${Icon} name="up" /></button>
            <button class="icon-btn" disabled=${i === cards.length - 1} onClick=${() => moveCard(k.id, col.id, i + 1)} aria-label=${t('common.move_down') + ': ' + k.title}><${Icon} name="down" /></button>
            <button class="icon-btn" disabled=${ci === b.columns.length - 1} onClick=${() => moveCard(k.id, b.columns[ci + 1].id)} aria-label=${t('boards.move_right') + ': ' + k.title}><${Icon} name="right" /></button></div>`}
        </div>`)}
        ${canEdit && html`<button onClick=${() => setModal({ columnId: col.id })}><${Icon} name="plus" /> ${t('boards.new_card')}</button>`}
      </section>`;
    })}</div>
    ${modal && html`<${CardForm} board=${b} card=${modal.card} columnId=${modal.columnId} onClose=${(ok) => { setModal(null); if (ok) st.reload(); }} />`}
  <//>`;
}

export function Boards({ route }) {
  const st = useLoad(() => api('GET', '/boards'));
  const [open, setOpen] = useState(false);
  if (route[1]) return html`<${Detail} id=${route[1]} key=${route[1]} />`;
  if (!st.data) return html`<${Page} title=${t('nav.boards')}><${Loading} state=${st} /><//>`;
  return html`<${Page} title=${t('nav.boards')} actions=${html`<button class="primary" onClick=${() => setOpen(true)}><${Icon} name="plus" /> ${t('boards.new')}</button>`}>
    ${st.data.length === 0 ? html`<${Empty} icon="board" text=${t('boards.empty')} />` : html`<div class="cards">
      ${st.data.map((b) => html`<a class="card" href=${'#/boards/' + b.id} key=${b.id}><h3>${b.title}</h3>
        <div class="row"><${SharedBadge} shared=${b.shared} /></div>
        <p class="muted small">${t('boards.card_count', { n: b.card_count })} · ${userName(b.owner_id)} · ${fmtDateTime(b.updated_at)}</p></a>`)}</div>`}
    ${open && html`<${NewBoard} onClose=${() => setOpen(false)} />`}
  <//>`;
}
