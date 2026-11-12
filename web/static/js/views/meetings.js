import { html, useState, useEffect, t, api, act, download, useLoad, go, can, userName, linkLabel, safeUrl, fmtDateTime, toInput, fromInput,
  Icon, Modal, Field, useForm, ShareEditor, ShareButton, Badge, Page, Empty, Loading, confirmDelete } from '../lib.js';
import { eventWhen } from './calendar.js';
import { Attachments } from '../media.js';

const statusBadge = (m) => html`<${Badge} kind=${m.status === 'finalized' ? 'ok' : m.status === 'in_progress' ? 'warn' : 'info'}>${t('meet.status_' + m.status)}<//>`;

function NewMeeting({ onClose }) {
  const base = Math.floor(Date.now() / 3600000) * 3600 + 86400;
  const [f, , bind, check] = useForm({ title: '', description: '', location: '', url: '', start: toInput(base), end: toInput(base + 5400), template: true });
  const [shares, setShares] = useState([]);
  const submit = async (e) => {
    e.preventDefault();
    const agenda = f.template ? t('meet.template').split('|') : [];
    const r = await act(() => api('POST', '/meetings', { title: f.title, description: f.description, location: f.location, url: f.url,
      starts_at: fromInput(f.start), ends_at: fromInput(f.end), agenda, shares }));
    if (r) { onClose(); go('meetings', r.id); }
  };
  return html`<${Modal} title=${t('meet.new')} onClose=${onClose}><form onSubmit=${submit}>
    <${Field} label=${t('common.title')}><input ...${bind('title')} required maxlength="200" /><//>
    <div class="grid2">
      <${Field} label=${t('cal.starts')}><input type="datetime-local" ...${bind('start')} required /><//>
      <${Field} label=${t('cal.ends')}><input type="datetime-local" ...${bind('end')} min=${f.start} required /><//>
    </div>
    <${Field} label=${t('cal.location')}><input ...${bind('location')} /><//>
    <${Field} label=${t('cal.link')} hint=${t('cal.link_hint')}><input type="url" ...${bind('url')} placeholder="https://" /><//>
    <${Field} label=${t('common.description')}><textarea ...${bind('description')}></textarea><//>
    <label class="check"><input type="checkbox" ...${check('template')} /> ${t('meet.use_template')}</label>
    <${Field} group label=${t('meet.invited')} hint=${t('meet.invited_hint')}><${ShareEditor} value=${shares} onChange=${setShares} /><//>
    <div class="buttons"><button type="button" onClick=${onClose}>${t('common.cancel')}</button><button class="primary">${t('common.create')}</button></div>
  </form><//>`;
}

function DecisionForm({ meeting, item, onClose }) {
  const [f, , bind] = useForm({ title: item.title, body: item.notes || '' });
  const [shares, setShares] = useState([{ type: 'all', id: '', level: 1 }]);
  const submit = async (e) => {
    e.preventDefault();
    const r = await act(() => api('POST', '/decisions', { ...f, decided_at: meeting.starts_at, meeting_id: meeting.id, agenda_item_id: item.id, shares }));
    if (r) onClose(r);
  };
  return html`<${Modal} title=${t('dec.record')} onClose=${() => onClose(null)}><form onSubmit=${submit}>
    <${Field} label=${t('dec.title')}><input ...${bind('title')} required maxlength="200" /><//>
    <${Field} label=${t('dec.text')}><textarea ...${bind('body')} rows="5"></textarea><//>
    <${Field} group label=${t('dec.audience')} hint=${t('dec.audience_hint')}><${ShareEditor} value=${shares} onChange=${setShares} levels=${false} /><//>
    <div class="buttons"><button type="button" onClick=${() => onClose(null)}>${t('common.cancel')}</button><button class="primary">${t('dec.record')}</button></div>
  </form><//>`;
}

function Detail({ id }) {
  const st = useLoad(() => api('GET', '/meetings/' + id), [id]);
  const [tab, setTab] = useState(0);
  const [m, setM] = useState(null); // editable copy
  const [agenda, setAgenda] = useState([]);
  const [decFor, setDecFor] = useState(null);
  const [dirty, setDirty] = useState(false);
  useEffect(() => {
    if (!st.data) return;
    setM({ ...st.data, start: toInput(st.data.starts_at), end: toInput(st.data.ends_at) });
    setAgenda(st.data.agenda || []);
    setDirty(false);
    if (st.data.status === 'finalized') setTab(3);
  }, [st.data]);
  if (!st.data || !m) return html`<${Page} title=${t('nav.meetings')} back="#/meetings"><${Loading} state=${st} /><//>`;
  const locked = m.status === 'finalized' || m.level < 2;
  const set = (k) => (e) => { setM({ ...m, [k]: e.target.value }); setDirty(true); };
  const setItem = (i, k, v) => { setAgenda(agenda.map((x, j) => (j === i ? { ...x, [k]: v } : x))); setDirty(true); };
  const move = (i, d) => { const a = [...agenda]; const j = i + d; if (j < 0 || j >= a.length) return; [a[i], a[j]] = [a[j], a[i]]; setAgenda(a); setDirty(true); };
  const save = async (status) => {
    const body = { title: m.title, description: m.description, location: m.location, url: m.url, starts_at: fromInput(m.start), ends_at: fromInput(m.end),
      status: status || m.status, attendees: m.attendees, chair: m.chair, secretary: m.secretary, notes: m.notes };
    const ok = await act(async () => {
      await api('PUT', `/meetings/${id}/agenda`, agenda.filter((x) => x.title.trim()).map((x) => ({ id: x.id || '', title: x.title, description: x.description || '',
        presenter: x.presenter || '', duration_min: +x.duration_min || 0, notes: x.notes || '' })));
      await api('PATCH', '/meetings/' + id, body);
    }, t('common.saved'));
    if (ok) await st.reload();
    return ok;
  };
  const finalize = async () => {
    if (!window.confirm(t('meet.confirm_finalize'))) return;
    if (dirty && !(await save())) return;
    if (await act(() => api('POST', `/meetings/${id}/finalize`), t('meet.finalized'))) st.reload();
  };
  const reopen = async () => { if (await act(() => api('POST', `/meetings/${id}/reopen`))) { await st.reload(); setTab(2); } };
  const del = async () => { if (confirmDelete(m.title) && await act(() => api('DELETE', '/meetings/' + id))) go('meetings'); };
  const recordDecision = async (item) => {
    // The decision is linked to a stored agenda item: save pending edits first.
    if (dirty || !item.id) { if (!(await save('in_progress'))) return; }
    const fresh = (await api('GET', '/meetings/' + id)).agenda.find((x) => x.title === item.title);
    if (fresh) setDecFor(fresh);
  };
  const total = agenda.reduce((n, x) => n + (+x.duration_min || 0), 0);
  const url = safeUrl(m.url);
  const saveBar = !locked && html`<div class="buttons no-print"><button class="primary" disabled=${!dirty} onClick=${() => save(tab === 2 ? 'in_progress' : undefined)}>${t('common.save')}</button></div>`;

  return html`<${Page} title=${m.title} back="#/meetings" actions=${m.level >= 3 && html`
      <${ShareButton} type="meeting" id=${id} onDone=${st.reload} /><button class="danger" onClick=${del}><${Icon} name="trash" /> ${t('common.delete')}</button>`}>
    <p class="row">${statusBadge(m)}<span><${Icon} name="calendar" /> ${eventWhen(st.data)}</span>${m.location && html`<span><${Icon} name="home" /> ${m.location}</span>`}
      ${url && html`<a href=${url} target="_blank" rel="noopener noreferrer" class="meetlink"><${Icon} name="video" /> ${linkLabel(url) ? t('common.join', { name: linkLabel(url) }) : t('cal.open_link')}</a>`}</p>
    <div class="steps" role="tablist">${['plan', 'agenda', 'protocol', 'finalize'].map((k, i) => html`<button role="tab" aria-selected=${tab === i} onClick=${() => setTab(i)}>${t('meet.step_' + k)}</button>`)}</div>

    ${tab === 0 && html`<div class="card" role="tabpanel">
      <${Field} label=${t('common.title')}><input value=${m.title} onInput=${set('title')} disabled=${locked} maxlength="200" /><//>
      <div class="grid2">
        <${Field} label=${t('cal.starts')}><input type="datetime-local" value=${m.start} onInput=${set('start')} disabled=${locked} /><//>
        <${Field} label=${t('cal.ends')}><input type="datetime-local" value=${m.end} onInput=${set('end')} disabled=${locked} /><//>
      </div>
      <${Field} label=${t('cal.location')}><input value=${m.location} onInput=${set('location')} disabled=${locked} /><//>
      <${Field} label=${t('cal.link')} hint=${t('cal.link_hint')}><input type="url" value=${m.url} onInput=${set('url')} disabled=${locked} placeholder="https://" /><//>
      <${Field} label=${t('common.description')}><textarea value=${m.description} onInput=${set('description')} disabled=${locked}></textarea><//>
      <p class="muted small"><${Icon} name="calendar" /> ${t('meet.calendar_note')}</p>${saveBar}</div>
      <div class="card"><${Attachments} type="meeting" id=${id} canEdit=${m.level >= 2 && m.status !== 'finalized'} /></div>`}

    ${tab === 1 && html`<div role="tabpanel">
      ${agenda.length === 0 && html`<${Empty} icon="list" text=${t('meet.no_agenda')} />`}
      ${agenda.map((it, i) => html`<div class="agenda-item" key=${it.id || 'new' + i}>
        <div class="head"><span class="num">§ ${i + 1}</span>
          <input class="grow" value=${it.title} onInput=${(e) => setItem(i, 'title', e.target.value)} disabled=${locked} aria-label=${t('common.title')} maxlength="200" />
          ${!locked && html`<button class="icon-btn" onClick=${() => move(i, -1)} disabled=${i === 0} aria-label=${t('common.move_up')}><${Icon} name="up" /></button>
            <button class="icon-btn" onClick=${() => move(i, 1)} disabled=${i === agenda.length - 1} aria-label=${t('common.move_down')}><${Icon} name="down" /></button>
            <button class="icon-btn" onClick=${() => { setAgenda(agenda.filter((_, j) => j !== i)); setDirty(true); }} disabled=${!!it.decision_id} aria-label=${t('common.remove')}><${Icon} name="trash" /></button>`}</div>
        <div class="grid2">
          <${Field} label=${t('meet.presenter')}><input value=${it.presenter || ''} onInput=${(e) => setItem(i, 'presenter', e.target.value)} disabled=${locked} /><//>
          <${Field} label=${t('meet.minutes')}><input type="number" min="0" max="600" value=${it.duration_min || ''} onInput=${(e) => setItem(i, 'duration_min', e.target.value)} disabled=${locked} /><//>
        </div>
        <${Field} label=${t('meet.background')}><textarea value=${it.description || ''} onInput=${(e) => setItem(i, 'description', e.target.value)} disabled=${locked}></textarea><//>
      </div>`)}
      <div class="row">${!locked && html`<button onClick=${() => { setAgenda([...agenda, { title: '', description: '', presenter: '', duration_min: 0, notes: '' }]); setDirty(true); }}>
        <${Icon} name="plus" /> ${t('meet.add_item')}</button>`}<span class="muted right">${t('meet.total', { n: total })}</span></div>
      ${saveBar}</div>`}

    ${tab === 2 && html`<div role="tabpanel">
      <div class="card"><div class="grid2">
        <${Field} label=${t('meet.chair')}><input value=${m.chair} onInput=${set('chair')} disabled=${locked} /><//>
        <${Field} label=${t('meet.secretary')}><input value=${m.secretary} onInput=${set('secretary')} disabled=${locked} /><//></div>
        <${Field} label=${t('meet.attendees')} hint=${t('meet.attendees_hint')}><textarea value=${m.attendees} onInput=${set('attendees')} disabled=${locked}></textarea><//></div>
      ${agenda.map((it, i) => html`<div class="agenda-item" key=${it.id || 'new' + i}>
        <div class="head"><span class="num">§ ${i + 1}</span><h3 class="grow">${it.title}</h3></div>
        ${it.description && html`<p class="muted small pre">${it.description}</p>`}
        <${Field} label=${t('meet.notes')}><textarea rows="3" value=${it.notes || ''} onInput=${(e) => setItem(i, 'notes', e.target.value)} disabled=${locked}></textarea><//>
        ${it.decision_id ? html`<a href=${'#/decisions/' + it.decision_id} class="badge info"><${Icon} name="decision" /> ${t('meet.decision_recorded')}</a>`
          : !locked && can('decisions.create') && html`<button onClick=${() => recordDecision(it)}><${Icon} name="decision" /> ${t('dec.record')}</button>`}
      </div>`)}
      <div class="card"><${Field} label=${t('meet.other_notes')}><textarea rows="4" value=${m.notes} onInput=${set('notes')} disabled=${locked}></textarea><//></div>
      ${saveBar}</div>`}

    ${tab === 3 && html`<div class="card" role="tabpanel">
      <h2>${t('meet.step_finalize')}</h2>
      ${m.status === 'finalized'
        ? html`<p><${Icon} name="lock" /> ${t('meet.finalized_by', { name: userName(m.finalized_by), when: fmtDateTime(m.finalized_at) })}</p>`
        : html`<p class="muted">${t('meet.finalize_text')}</p>`}
      <div class="row">
        <button onClick=${() => act(() => download('GET', `/meetings/${id}/protocol.md`, undefined, 'protocol.md'))}><${Icon} name="download" /> ${t('meet.export_md')}</button>
        <button onClick=${() => { setTab(2); setTimeout(() => window.print(), 100); }}>${t('meet.print')}</button>
        ${m.level >= 3 && (m.status === 'finalized' ? html`<button onClick=${reopen}>${t('meet.reopen')}</button>`
          : html`<button class="primary" onClick=${finalize}><${Icon} name="lock" /> ${t('meet.finalize')}</button>`)}
      </div></div>`}
    ${decFor && html`<${DecisionForm} meeting=${st.data} item=${decFor} onClose=${(r) => { setDecFor(null); if (r) st.reload(); }} />`}
  <//>`;
}

export function Meetings({ route }) {
  const st = useLoad(() => api('GET', '/meetings'));
  const [open, setOpen] = useState(false);
  if (route[1]) return html`<${Detail} id=${route[1]} key=${route[1]} />`;
  if (!st.data) return html`<${Page} title=${t('nav.meetings')}><${Loading} state=${st} /><//>`;
  return html`<${Page} title=${t('nav.meetings')} actions=${can('meetings.create') && html`<button class="primary" onClick=${() => setOpen(true)}><${Icon} name="plus" /> ${t('meet.new')}</button>`}>
    ${st.data.length === 0 ? html`<${Empty} icon="meeting" text=${t('meet.empty')} />` : html`<div class="rows">
      ${st.data.map((m) => html`<a href=${'#/meetings/' + m.id} key=${m.id}><${Icon} name="meeting" />
        <span class="grow"><b>${m.title}</b><br /><span class="muted small">${eventWhen(m)}${m.location ? ' · ' + m.location : ''}</span></span>${statusBadge(m)}</a>`)}</div>`}
    ${open && html`<${NewMeeting} onClose=${() => setOpen(false)} />`}
  <//>`;
}
