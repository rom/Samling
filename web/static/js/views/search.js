import { html, useState, t, api, errText, fmtDateTime, fromDateInput, Icon, Field, useForm, Page, Empty, Badge } from '../lib.js';

const types = ['message', 'channel', 'document', 'list', 'poll', 'event', 'meeting', 'decision', 'board', 'card', 'user'];
const icons = { message: 'chat', channel: 'chat', document: 'doc', list: 'list', poll: 'poll', event: 'calendar', meeting: 'meeting', decision: 'decision', board: 'board', card: 'board', user: 'people' };
const href = (h) => ({
  message: '#/chat/' + h.link, channel: '#/chat/' + h.id, document: '#/documents/' + h.id, list: '#/lists/' + h.link, poll: '#/polls/' + h.id, event: '#/calendar',
  meeting: '#/meetings/' + h.link, decision: '#/decisions/' + h.id, board: '#/boards/' + h.id, card: '#/boards/' + h.link, user: '#/people',
}[h.type]);

export function Search() {
  const [f, , bind, check] = useForm({ q: '', mode: 'plain', case: false, from: '', to: '' });
  const [sel, setSel] = useState([]);
  const [advanced, setAdvanced] = useState(false);
  const [state, setState] = useState({ hits: null, error: '', busy: false });
  const submit = async (e) => {
    e.preventDefault();
    if (!f.q.trim()) return;
    const p = new URLSearchParams({ q: f.q, mode: f.mode });
    if (f.case) p.set('case', '1');
    if (sel.length) p.set('types', sel.join(','));
    if (f.from) p.set('from', fromDateInput(f.from));
    if (f.to) p.set('to', fromDateInput(f.to) + 86399);
    setState({ hits: null, error: '', busy: true });
    try {
      setState({ hits: await api('GET', '/search?' + p), error: '', busy: false });
    } catch (err) {
      setState({ hits: null, error: errText(err), busy: false });
    }
  };
  const groups = {};
  (state.hits || []).forEach((h) => { (groups[h.type] = groups[h.type] || []).push(h); });
  return html`<${Page} title=${t('nav.search')}>
    <form class="card" onSubmit=${submit} role="search">
      <div class="row">
        <input class="grow" type="search" ...${bind('q')} placeholder=${t('search.placeholder')} aria-label=${t('search.placeholder')} autofocus maxlength="300" />
        <button class="primary" disabled=${state.busy}><${Icon} name="search" /> ${t('search.go')}</button>
      </div>
      <p><button type="button" class="link" aria-expanded=${advanced} onClick=${() => setAdvanced(!advanced)}>${t('search.advanced')} ${advanced ? '▴' : '▾'}</button></p>
      ${advanced && html`<div>
        <div class="grid2">
          <${Field} label=${t('search.mode')} hint=${t('search.mode_hint_' + f.mode)}><select ...${bind('mode')}>
            ${['plain', 'wildcard', 'regex'].map((m) => html`<option value=${m}>${t('search.mode_' + m)}</option>`)}</select><//>
          <div><${Field} label=${t('search.from')}><input type="date" ...${bind('from')} /><//>
            <${Field} label=${t('search.to')}><input type="date" ...${bind('to')} /><//></div>
        </div>
        <label class="check"><input type="checkbox" ...${check('case')} /> ${t('search.case')}</label>
        <fieldset><legend>${t('search.where')}</legend><div class="row">
          ${types.map((ty) => html`<label class="check"><input type="checkbox" checked=${sel.includes(ty)}
            onChange=${(e) => setSel(e.target.checked ? [...sel, ty] : sel.filter((x) => x !== ty))} /> ${t('search.type_' + ty)}</label>`)}</div>
          <p class="muted small">${t('search.where_hint')}</p></fieldset>
      </div>`}
    </form>
    <div aria-live="polite" style=${{ marginTop: '1rem' }}>
      ${state.error && html`<p class="error-text" role="alert">${state.error}</p>`}
      ${state.busy && html`<div class="loading">${t('common.loading')}</div>`}
      ${state.hits && state.hits.length === 0 && html`<${Empty} icon="search" text=${t('search.none')} />`}
      ${state.hits && state.hits.length > 0 && html`<p class="muted">${t('search.count', { n: state.hits.length })}</p>`}
      ${Object.entries(groups).map(([ty, hits]) => html`<h2 style=${{ margin: '1rem 0 .5rem' }}><${Icon} name=${icons[ty]} /> ${t('search.type_' + ty)} <${Badge}>${hits.length}<//></h2>
        <div class="rows">${hits.map((h) => html`<a class="hit" href=${href(h)} key=${h.type + h.id}>
          <span class="grow">${h.title && html`<b>${h.title}</b><br />`}<span class="snippet">${h.snippet}</span></span>
          ${h.at > 0 && html`<span class="muted small nowrap">${fmtDateTime(h.at)}</span>`}</a>`)}</div>`)}
    </div>
  <//>`;
}
