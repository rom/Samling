import { html, useState, useEffect, t, api, act, useLoad, useLive, go, can, userName, linkify, fmtDateTime, toInput, fromInput, Icon, Modal, Field, useForm,
  ShareEditor, ShareButton, SharedBadge, Badge, Page, Empty, Loading, confirmDelete } from '../lib.js';
import { Attachments, ScanInto } from '../media.js';

const kinds = ['single', 'multi', 'yesno', 'ranked', 'score'];
export const optText = (o) => (o.text.startsWith('@') ? t('polls.' + o.text.slice(1)) : o.text);

function NewPoll({ onClose }) {
  const [f, set, bind, check] = useForm({ title: '', description: '', kind: 'single', anonymous: false, results_visible: 'after_vote', allow_change: true, max_choices: 0, closes: '' });
  const [options, setOptions] = useState(['', '']);
  const [shares, setShares] = useState([{ type: 'all', id: '', level: 1 }]);
  const submit = async (e) => {
    e.preventDefault();
    const r = await act(() => api('POST', '/polls', { title: f.title, description: f.description, kind: f.kind, anonymous: f.anonymous, results_visible: f.results_visible,
      allow_change: f.allow_change && !f.anonymous, max_choices: +f.max_choices || 0, closes_at: fromInput(f.closes), options: options.filter((o) => o.trim()), shares }));
    if (r) { onClose(); go('polls', r.id); }
  };
  return html`<${Modal} title=${t('polls.new')} onClose=${onClose} wide><form onSubmit=${submit}>
    <${Field} label=${t('polls.question')}><input ...${bind('title')} required maxlength="200" /><//>
    <${Field} label=${t('common.description')}><textarea ...${bind('description')}></textarea><//>
    <${ScanInto} value=${f.description} onChange=${(v) => set('description', v)} />
    <${Field} label=${t('polls.kind')} hint=${t('polls.kind_hint_' + f.kind)}>
      <select ...${bind('kind')}>${kinds.map((k) => html`<option value=${k}>${t('polls.kind_' + k)}</option>`)}</select><//>
    ${f.kind !== 'yesno' && html`<fieldset><legend>${t('polls.options')}</legend>
      ${options.map((o, i) => html`<div class="row" style=${{ marginBottom: '.4rem' }} key=${i}>
        <input class="grow" value=${o} aria-label=${t('polls.option_n', { n: i + 1 })} placeholder=${t('polls.option_n', { n: i + 1 })} maxlength="200"
          onInput=${(e) => setOptions(options.map((x, j) => (j === i ? e.target.value : x)))} />
        <button type="button" class="icon-btn" disabled=${options.length <= 2} aria-label=${t('common.remove')} onClick=${() => setOptions(options.filter((_, j) => j !== i))}><${Icon} name="x" /></button></div>`)}
      <button type="button" onClick=${() => setOptions([...options, ''])}><${Icon} name="plus" /> ${t('polls.add_option')}</button></fieldset>`}
    <div class="grid2">
      ${f.kind === 'multi' && html`<${Field} label=${t('polls.max_choices')} hint=${t('polls.max_choices_hint')}><input type="number" min="0" max="30" ...${bind('max_choices')} /><//>`}
      <${Field} label=${t('polls.results')} hint=${f.anonymous ? t('polls.results_anonymous') : ''}><select ...${bind('results_visible')} value=${f.anonymous ? 'after_close' : f.results_visible} disabled=${f.anonymous}>
        ${['always', 'after_vote', 'after_close'].map((k) => html`<option value=${k}>${t('polls.results_' + k)}</option>`)}</select><//>
      <${Field} label=${t('polls.closes')} hint=${t('polls.closes_hint')}><input type="datetime-local" ...${bind('closes')} /><//>
    </div>
    <label class="check"><input type="checkbox" ...${check('anonymous')} /><span>${t('polls.anonymous')}<br /><span class="muted small">${t('polls.anonymous_hint')}</span></span></label>
    ${!f.anonymous && html`<label class="check"><input type="checkbox" ...${check('allow_change')} /> ${t('polls.allow_change')}</label>`}
    <${Field} group label=${t('polls.who')} hint=${t('polls.who_hint')}><${ShareEditor} value=${shares} onChange=${setShares} levels=${false} /><//>
    <div class="buttons"><button type="button" onClick=${onClose}>${t('common.cancel')}</button><button class="primary">${t('common.create')}</button></div>
  </form><//>`;
}

function status(p) {
  if (p.closed) return html`<${Badge}>${t('polls.closed')}<//>`;
  return html`<${Badge} kind="ok">${t('polls.open')}<//>`;
}

function Detail({ id }) {
  const st = useLoad(() => api('GET', '/polls/' + id), [id]);
  const [choice, setChoice] = useState({});
  const [changing, setChanging] = useState(false);
  useLive(['poll'], (e) => { if (e.id === id) st.reload(); });
  const p = st.data;
  useEffect(() => {
    if (!p) return;
    if (p.my_vote) setChoice(p.my_vote);
    else if (p.kind === 'ranked') setChoice(Object.fromEntries(p.options.map((o, i) => [o.id, i + 1])));
    else if (p.kind === 'score') setChoice(Object.fromEntries(p.options.map((o) => [o.id, 0])));
  }, [p && p.id, p && p.voted]);
  if (!p) return html`<${Page} title=${t('nav.polls')} back="#/polls"><${Loading} state=${st} /><//>`;
  const canVote = !p.closed && (!p.voted || (changing && p.allow_change));
  const vote = async () => { if (await act(() => api('POST', `/polls/${id}/vote`, { choices: choice }), t('polls.voted'))) { setChanging(false); st.reload(); } };
  const close = async () => { if (window.confirm(t('polls.confirm_close')) && await act(() => api('POST', `/polls/${id}/close`))) st.reload(); };
  const del = async () => { if (confirmDelete(p.title) && await act(() => api('DELETE', '/polls/' + id))) go('polls'); };
  const total = p.kind === 'ranked' || p.kind === 'score' ? Math.max(1, ...p.options.map((o) => o.score)) : Math.max(1, p.voters);
  const ranked = p.kind === 'ranked' ? [...p.options].sort((a, b) => choice[a.id] - choice[b.id]) : p.options;
  const moveRank = (o, delta) => {
    const order = ranked.map((x) => x.id);
    const i = order.indexOf(o.id), j = i + delta;
    if (j < 0 || j >= order.length) return;
    [order[i], order[j]] = [order[j], order[i]];
    setChoice(Object.fromEntries(order.map((oid, k) => [oid, k + 1])));
  };
  const valid = p.kind === 'ranked' || p.kind === 'score' || Object.keys(choice).length > 0;

  const input = (o) => {
    if (p.kind === 'single' || p.kind === 'yesno') return html`<label><input type="radio" name="opt" checked=${!!choice[o.id]} onChange=${() => setChoice({ [o.id]: 1 })} /> ${optText(o)}</label>`;
    if (p.kind === 'multi') {
      const full = p.max_choices > 0 && Object.keys(choice).length >= p.max_choices && !choice[o.id];
      return html`<label><input type="checkbox" checked=${!!choice[o.id]} disabled=${full} onChange=${(e) => {
        const c = { ...choice }; if (e.target.checked) c[o.id] = 1; else delete c[o.id]; setChoice(c); }} /> ${optText(o)}</label>`;
    }
    if (p.kind === 'ranked') return html`<div class="row"><b class="nowrap">${choice[o.id]}.</b><span class="grow">${optText(o)}</span>
      <button class="icon-btn" onClick=${() => moveRank(o, -1)} aria-label=${t('common.move_up') + ': ' + optText(o)}><${Icon} name="up" /></button>
      <button class="icon-btn" onClick=${() => moveRank(o, 1)} aria-label=${t('common.move_down') + ': ' + optText(o)}><${Icon} name="down" /></button></div>`;
    return html`<label><span class="grow">${optText(o)}</span>
      <input type="range" min="0" max="5" step="1" value=${choice[o.id] || 0} onInput=${(e) => setChoice({ ...choice, [o.id]: +e.target.value })} aria-label=${optText(o)} />
      <b>${choice[o.id] || 0}</b></label>`;
  };

  return html`<${Page} title=${p.title} back="#/polls" actions=${html`
      ${p.closed && can('decisions.create') && html`<a class="btn" href=${'#/decisions/new/' + p.id}><${Icon} name="decision" /> ${t('polls.to_decision')}</a>`}
      ${p.level >= 3 && html`${!p.closed && html`<button onClick=${close}><${Icon} name="lock" /> ${t('polls.close')}</button>`}
        <${ShareButton} type="poll" id=${id} onDone=${st.reload} levels=${false} />
        <button class="danger" onClick=${del}><${Icon} name="trash" /> ${t('common.delete')}</button>`}`}>
    <div class="row">${status(p)}<${Badge}>${t('polls.kind_' + p.kind)}<//>${p.anonymous && html`<${Badge} kind="info"><${Icon} name="lock" /> ${t('polls.anonymous_badge')}<//>`}
      <span class="muted small">${t('polls.by', { name: userName(p.owner_id) })} · ${t('polls.voters', { n: p.voters })}
        ${p.closes_at > 0 && !p.closed ? ' · ' + t('polls.closes_at', { when: fmtDateTime(p.closes_at) }) : ''}${p.closed ? ' · ' + t('polls.closed_at', { when: fmtDateTime(p.closed_at) }) : ''}</span></div>
    ${p.description && html`<p class="pre">${linkify(p.description)}</p>`}
    <${Attachments} type="poll" id=${id} canEdit=${p.level >= 3} />
    <div class="stack">
      ${canVote && html`<div class="card"><h2>${t('polls.your_vote')}</h2>
        <p class="muted small">${t('polls.kind_hint_' + p.kind)}${p.kind === 'multi' && p.max_choices > 0 ? ' ' + t('polls.max_n', { n: p.max_choices }) : ''}</p>
        ${ranked.map((o) => html`<div class="opt" key=${o.id}>${input(o)}</div>`)}
        <div class="buttons">${changing && html`<button onClick=${() => setChanging(false)}>${t('common.cancel')}</button>`}
          <button class="primary" disabled=${!valid} onClick=${vote}><${Icon} name="check" /> ${t('polls.vote')}</button></div>
        ${p.anonymous && html`<p class="muted small"><${Icon} name="lock" /> ${t('polls.anonymous_note')}</p>`}</div>`}
      ${p.voted && !canVote && !p.closed && html`<div class="card"><${Icon} name="check" /> ${t('polls.thanks')}
        ${p.allow_change && html` <button class="link" onClick=${() => setChanging(true)}>${t('polls.change')}</button>`}</div>`}
      <div class="card"><h2>${t('polls.results_title')}</h2>
        ${!p.results_shown ? html`<p class="muted"><${Icon} name="lock" /> ${t('polls.hidden_' + p.results_visible)}</p>`
          : [...p.options].sort((a, b) => b.score - a.score).map((o) => html`<div class="opt" key=${o.id}>
            <div class="res"><span>${optText(o)}${p.my_vote && p.my_vote[o.id] !== undefined && (p.kind === 'ranked' || p.kind === 'score' || p.my_vote[o.id] > 0)
              ? html` <span class="badge info">${t('polls.mine')}${p.kind === 'ranked' || p.kind === 'score' ? ': ' + p.my_vote[o.id] : ''}</span>` : ''}</span>
              <b>${p.kind === 'ranked' || p.kind === 'score' ? t('polls.points', { n: o.score }) : `${o.votes} (${Math.round((100 * o.votes) / total)} %)`}</b></div>
            <div class="bar" role="img" aria-label=${optText(o) + ': ' + o.score}><div style=${{ width: (100 * (p.kind === 'ranked' || p.kind === 'score' ? o.score : o.votes)) / total + '%' }}></div></div></div>`)}
      </div>
    </div>
  <//>`;
}

export function Polls({ route }) {
  const st = useLoad(() => api('GET', '/polls'));
  const [open, setOpen] = useState(false);
  useLive(['poll'], st.reload);
  if (route[1]) return html`<${Detail} id=${route[1]} key=${route[1]} />`;
  if (!st.data) return html`<${Page} title=${t('nav.polls')}><${Loading} state=${st} /><//>`;
  const card = (p) => html`<a class="card" href=${'#/polls/' + p.id} key=${p.id}><h3>${p.title}</h3>
    <div class="row">${status(p)}<${Badge}>${t('polls.kind_' + p.kind)}<//><${SharedBadge} shared=${p.shared} />
      ${!p.closed && (p.voted ? html`<${Badge} kind="ok"><${Icon} name="check" /> ${t('polls.you_voted')}<//>` : html`<${Badge} kind="warn">${t('polls.not_voted')}<//>`)}</div>
    <p class="muted small">${t('polls.voters', { n: p.voters })}${p.closes_at > 0 && !p.closed ? ' · ' + t('polls.closes_at', { when: fmtDateTime(p.closes_at) }) : ''}</p></a>`;
  const active = st.data.filter((p) => !p.closed), closed = st.data.filter((p) => p.closed);
  return html`<${Page} title=${t('nav.polls')} actions=${html`<button class="primary" onClick=${() => setOpen(true)}><${Icon} name="plus" /> ${t('polls.new')}</button>`}>
    ${st.data.length === 0 && html`<${Empty} icon="poll" text=${t('polls.empty')} />`}
    ${active.length > 0 && html`<h2 style=${{ margin: '0 0 .6rem' }}>${t('polls.open_polls')}</h2><div class="cards">${active.map(card)}</div>`}
    ${closed.length > 0 && html`<h2 style=${{ margin: '1.4rem 0 .6rem' }}>${t('polls.closed_polls')}</h2><div class="cards">${closed.map(card)}</div>`}
    ${open && html`<${NewPoll} onClose=${() => setOpen(false)} />`}
  <//>`;
}
export { toInput };
