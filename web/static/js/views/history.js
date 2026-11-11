import { html, useState, useEffect, t, hasKey, api, act, fmtDate, fmtTime, Icon, Page, Empty } from '../lib.js';

export const actionText = (a) => (hasKey('act.' + a) ? t('act.' + a) : a);
const links = { channel: 'chat', document: 'documents', list: 'lists', poll: 'polls', meeting: 'meetings', decision: 'decisions', board: 'boards' };
const icons = { channel: 'chat', document: 'doc', list: 'list', poll: 'poll', event: 'calendar', meeting: 'meeting', decision: 'decision', board: 'board', user: 'people' };

// Personal history: what you have done in the application over time.
export function History() {
  const [rows, setRows] = useState(null);
  const [more, setMore] = useState(true);
  const [filter, setFilter] = useState('');
  const load = async (before) => {
    const r = await act(() => api('GET', '/me/history' + (before ? '?before=' + before : '')));
    if (!r) return;
    setRows((old) => (before ? [...old, ...r] : r));
    setMore(r.length >= 200);
  };
  useEffect(() => { load(); }, []);
  if (!rows) return html`<${Page} title=${t('nav.history')}><div class="loading">${t('common.loading')}</div><//>`;
  const kinds = [...new Set(rows.map((r) => r.res_type).filter(Boolean))];
  const shown = rows.filter((r) => !filter || r.res_type === filter);
  const days = {};
  shown.forEach((r) => { const k = fmtDate(r.created_at); (days[k] = days[k] || []).push(r); });
  return html`<${Page} title=${t('nav.history')}>
    <p class="muted">${t('history.text')}</p>
    <div class="row" style=${{ marginBottom: '1rem' }}><label>${t('history.show')} <select value=${filter} onChange=${(e) => setFilter(e.target.value)}>
      <option value="">${t('history.all')}</option>${kinds.map((k) => html`<option value=${k}>${hasKey('search.type_' + k) ? t('search.type_' + k) : k}</option>`)}</select></label></div>
    ${shown.length === 0 && html`<${Empty} icon="history" text=${t('history.empty')} />`}
    ${Object.entries(days).map(([day, list]) => html`<h2 style=${{ margin: '1rem 0 .5rem', fontSize: '1rem' }}>${day}</h2><div class="rows">
      ${list.map((r) => {
        const body = html`<${Icon} name=${icons[r.res_type] || 'history'} /><span class="grow">${actionText(r.action)}${r.summary && html` <span class="muted">– ${r.summary}</span>`}</span>
          <span class="muted small nowrap">${fmtTime(r.created_at)}</span>`;
        return links[r.res_type] && r.res_id && !r.action.endsWith('.deleted')
          ? html`<a key=${r.id} href=${'#/' + links[r.res_type] + '/' + r.res_id}>${body}</a>` : html`<div key=${r.id}>${body}</div>`;
      })}</div>`)}
    ${more && html`<p><button onClick=${() => load(rows[rows.length - 1].id)}>${t('history.more')}</button></p>`}
  <//>`;
}
