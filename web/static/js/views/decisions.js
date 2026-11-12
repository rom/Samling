import { html, useState, useEffect, t, api, act, download, useLoad, useLive, go, can, userName, linkify, fmtDate, fmtDateTime, toDateInput, fromDateInput,
  Icon, Modal, Field, useForm, ShareEditor, ShareButton, Badge, Page, Empty, Loading } from '../lib.js';
import { optText } from './polls.js';

const statusBadge = (d) => html`<${Badge} kind=${d.status === 'decided' ? 'ok' : 'danger'}>${t('dec.status_' + d.status)}<//>`;

function NewDecision({ pollId, onClose }) {
  const [f, , bind, , setF] = useForm({ title: '', body: '', date: toDateInput(Date.now() / 1000) });
  const [shares, setShares] = useState([{ type: 'all', id: '', level: 1 }]);
  useEffect(() => {
    if (!pollId) return;
    // Pre-fill from the poll result.
    api('GET', '/polls/' + pollId).then((p) => {
      const res = p.results_shown ? [...p.options].sort((a, b) => b.score - a.score).map((o) => `${optText(o)}: ${p.kind === 'ranked' || p.kind === 'score' ? o.score : o.votes}`).join('\n') : '';
      setF((old) => ({ ...old, title: p.title, body: t('dec.from_poll', { n: p.voters }) + '\n' + res }));
    }).catch(() => {});
  }, [pollId]);
  const submit = async (e) => {
    e.preventDefault();
    const r = await act(() => api('POST', '/decisions', { title: f.title, body: f.body, decided_at: fromDateInput(f.date) + 43200, poll_id: pollId || '', shares }), t('dec.recorded'));
    if (r) onClose(r.id);
  };
  return html`<${Modal} title=${t('dec.record')} onClose=${() => onClose(null)} wide><form onSubmit=${submit}>
    <${Field} label=${t('dec.title')}><input ...${bind('title')} required maxlength="200" /><//>
    <${Field} label=${t('dec.text')}><textarea ...${bind('body')} rows="6"></textarea><//>
    <${Field} label=${t('dec.date')}><input type="date" ...${bind('date')} required /><//>
    <${Field} group label=${t('dec.audience')} hint=${t('dec.audience_hint')}><${ShareEditor} value=${shares} onChange=${setShares} levels=${false} /><//>
    <div class="buttons"><button type="button" onClick=${() => onClose(null)}>${t('common.cancel')}</button><button class="primary">${t('dec.record')}</button></div>
  </form><//>`;
}

function Revise({ d, onClose }) {
  const [f, , bind] = useForm({ title: d.title, body: d.body, status: d.status, comment: '' });
  const submit = async (e) => {
    e.preventDefault();
    if (await act(() => api('PATCH', '/decisions/' + d.id, f), t('common.saved'))) onClose(true);
  };
  return html`<${Modal} title=${t('dec.revise')} onClose=${() => onClose(false)} wide><form onSubmit=${submit}>
    <p class="muted">${t('dec.revise_text')}</p>
    <${Field} label=${t('dec.title')}><input ...${bind('title')} required maxlength="200" /><//>
    <${Field} label=${t('dec.text')}><textarea ...${bind('body')} rows="6"></textarea><//>
    <${Field} label=${t('dec.status')}><select ...${bind('status')}>${['decided', 'revoked', 'superseded'].map((k) => html`<option value=${k}>${t('dec.status_' + k)}</option>`)}</select><//>
    <${Field} label=${t('dec.reason')}><input ...${bind('comment')} required maxlength="1000" /><//>
    <div class="buttons"><button type="button" onClick=${() => onClose(false)}>${t('common.cancel')}</button><button class="primary">${t('common.save')}</button></div>
  </form><//>`;
}

function Detail({ id }) {
  const st = useLoad(() => api('GET', '/decisions/' + id), [id]);
  const [revise, setRevise] = useState(false);
  if (!st.data) return html`<${Page} title=${t('nav.decisions')} back="#/decisions"><${Loading} state=${st} /><//>`;
  const d = st.data;
  return html`<${Page} title=${d.number + ' ' + d.title} back="#/decisions" actions=${d.level >= 3 && html`
      ${can('decisions.create') && html`<button onClick=${() => setRevise(true)}><${Icon} name="edit" /> ${t('dec.revise')}</button>`}
      <${ShareButton} type="decision" id=${id} onDone=${st.reload} levels=${false} />`}>
    <div class="stack">
      <div class="card"><p class="row">${statusBadge(d)}<span><${Icon} name="calendar" /> ${fmtDate(d.decided_at)}</span>
        ${d.meeting_id && html`<a href=${'#/meetings/' + d.meeting_id}><${Icon} name="meeting" /> ${t('dec.from_meeting')}</a>`}
        ${d.poll_id && html`<a href=${'#/polls/' + d.poll_id}><${Icon} name="poll" /> ${t('dec.from_poll_link')}</a>`}</p>
        <div class="pre">${linkify(d.body || '')}</div></div>
      <div class="card"><h2>${t('dec.history')}</h2><div class="timeline" style=${{ marginTop: '1rem' }}>
        ${(d.revisions || []).map((r, i) => html`<div key=${i}><b>${fmtDateTime(r.created_at)}</b> · ${userName(r.user_id)} · ${statusBadge(r)}
          <div>${i === d.revisions.length - 1 ? t('dec.created') : r.comment}</div>
          ${i < d.revisions.length - 1 && html`<details><summary class="small muted">${t('dec.show_text')}</summary><p><b>${r.title}</b></p><p class="pre">${r.body}</p></details>`}</div>`)}
      </div></div>
    </div>
    ${revise && html`<${Revise} d=${d} onClose=${(ok) => { setRevise(false); if (ok) st.reload(); }} />`}
  <//>`;
}

export function Decisions({ route }) {
  const st = useLoad(() => api('GET', '/decisions'));
  const polls = useLoad(() => api('GET', '/polls'));
  const [tab, setTab] = useState('log');
  const [q, setQ] = useState('');
  const [year, setYear] = useState('');
  useLive(['decision'], st.reload);
  if (route[1] && route[1] !== 'new') return html`<${Detail} id=${route[1]} key=${route[1]} />`;
  if (!st.data) return html`<${Page} title=${t('nav.decisions')}><${Loading} state=${st} /><//>`;
  const years = [...new Set(st.data.map((d) => d.number.slice(0, 4)))];
  const list = st.data.filter((d) => (!year || d.number.startsWith(year)) && (d.number + ' ' + d.title + ' ' + d.body).toLowerCase().includes(q.toLowerCase()));
  // Decision history: decisions and concluded polls on one timeline.
  const timeline = [
    ...st.data.map((d) => ({ at: d.decided_at, kind: 'decision', d })),
    ...(polls.data || []).filter((p) => p.closed).map((p) => ({ at: p.closed_at, kind: 'poll', p })),
  ].sort((a, b) => b.at - a.at);
  return html`<${Page} title=${t('nav.decisions')} actions=${html`
      <button onClick=${() => act(() => download('GET', '/decisions/export.csv', undefined, 'decisions.csv'))}><${Icon} name="download" /> ${t('dec.export')}</button>
      ${can('decisions.create') && html`<a class="btn primary" href="#/decisions/new"><${Icon} name="plus" /> ${t('dec.record')}</a>`}`}>
    <div class="tabs" role="tablist">
      <button role="tab" aria-selected=${tab === 'log'} onClick=${() => setTab('log')}>${t('dec.log')}</button>
      <button role="tab" aria-selected=${tab === 'history'} onClick=${() => setTab('history')}>${t('dec.timeline')}</button></div>
    ${tab === 'log' && html`
      <div class="row" style=${{ marginBottom: '1rem' }}>
        <select aria-label=${t('dec.year')} value=${year} onChange=${(e) => setYear(e.target.value)}><option value="">${t('dec.all_years')}</option>${years.map((y) => html`<option>${y}</option>`)}</select>
        <input class="grow" type="search" placeholder=${t('common.filter')} aria-label=${t('common.filter')} value=${q} onInput=${(e) => setQ(e.target.value)} /></div>
      ${list.length === 0 ? html`<${Empty} icon="decision" text=${t('dec.empty')} />` : html`<div class="rows">
        ${list.map((d) => html`<a href=${'#/decisions/' + d.id} key=${d.id}><b class="nowrap">${d.number}</b>
          <span class="grow">${d.title}<br /><span class="muted small">${fmtDate(d.decided_at)}</span></span>${statusBadge(d)}</a>`)}</div>`}`}
    ${tab === 'history' && (timeline.length === 0 ? html`<${Empty} icon="history" text=${t('dec.empty')} />` : html`<div class="timeline">
      ${timeline.map((x) => (x.kind === 'decision'
        ? html`<div key=${'d' + x.d.id}><span class="muted small">${fmtDate(x.at)}</span> <${Badge} kind="info"><${Icon} name="decision" /> ${t('dec.one')}<//> ${statusBadge(x.d)}
            <div><a href=${'#/decisions/' + x.d.id}><b>${x.d.number}</b> ${x.d.title}</a></div></div>`
        : html`<div key=${'p' + x.p.id}><span class="muted small">${fmtDate(x.at)}</span> <${Badge}><${Icon} name="poll" /> ${t('polls.one')}<//>
            <div><a href=${'#/polls/' + x.p.id}>${x.p.title}</a> <span class="muted small">${t('polls.voters', { n: x.p.voters })}</span></div></div>`))}</div>`)}
    ${route[1] === 'new' && html`<${NewDecision} pollId=${route[2]} onClose=${(newId) => { st.reload(); if (newId) go('decisions', newId); else go('decisions'); }} />`}
  <//>`;
}
