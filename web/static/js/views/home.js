import { html, useStore, t, api, useLoad, useLive, fmtDate, Icon, Page, Badge } from '../lib.js';
import { eventWhen } from './calendar.js';

export function Home() {
  const s = useStore();
  const now = Math.floor(Date.now() / 1000);
  const st = useLoad(async () => {
    const [channels, events, polls, decisions] = await Promise.all([
      api('GET', '/channels'), api('GET', `/calendar?from=${now}&to=${now + 30 * 86400}`), api('GET', '/polls'), api('GET', '/decisions')]);
    return { channels, events, polls, decisions };
  });
  useLive(['channel', 'event', 'poll', 'decision', 'meeting'], st.reload);
  const d = st.data || { channels: [], events: [], polls: [], decisions: [] };
  const unread = d.channels.filter((c) => c.unread > 0);
  const toVote = d.polls.filter((p) => !p.closed && !p.voted);
  const hour = new Date().getHours();
  const greeting = hour < 10 ? t('home.morning') : hour < 18 ? t('home.day') : t('home.evening');
  const section = (icon, title, href, body) => html`<div class="card"><div class="row" style=${{ marginBottom: '.5rem' }}>
    <h2 class="grow"><${Icon} name=${icon} /> ${title}</h2><a href=${href} class="small">${t('home.all')}</a></div>${body}</div>`;
  const none = html`<p class="muted">${t('home.nothing')}</p>`;
  return html`<${Page} title=${greeting + ', ' + s.me.display_name.split(' ')[0] + '!'}>
    <div class="cards" style=${{ gridTemplateColumns: 'repeat(auto-fill, minmax(20rem, 1fr))' }}>
      ${section('chat', t('home.unread'), '#/chat', unread.length === 0 ? none : html`<div class="rows">${unread.map((c) => html`<a href=${'#/chat/' + c.id} key=${c.id}>
        <span class="grow">${c.kind === 'dm' ? (c.members || []).filter((m) => m !== s.me.id).map((m) => (s.userMap[m] || {}).display_name).join(', ') : '# ' + c.name}</span>
        <${Badge} kind="info">${c.unread}<//></a>`)}</div>`)}
      ${section('calendar', t('home.upcoming'), '#/calendar', d.events.length === 0 ? none : html`<div class="rows">${d.events.slice(0, 6).map((e) => html`
        <a href=${e.meeting_id ? '#/meetings/' + e.meeting_id : '#/calendar'} key=${e.id}><span class="grow"><b>${e.title}</b><br /><span class="muted small">${eventWhen(e)}</span></span></a>`)}</div>`)}
      ${section('poll', t('home.to_vote'), '#/polls', toVote.length === 0 ? none : html`<div class="rows">${toVote.slice(0, 6).map((p) => html`
        <a href=${'#/polls/' + p.id} key=${p.id}><span class="grow">${p.title}</span><${Badge} kind="warn">${t('polls.not_voted')}<//></a>`)}</div>`)}
      ${section('decision', t('home.decisions'), '#/decisions', d.decisions.length === 0 ? none : html`<div class="rows">${d.decisions.slice(0, 5).map((x) => html`
        <a href=${'#/decisions/' + x.id} key=${x.id}><b class="nowrap">${x.number}</b><span class="grow">${x.title}</span><span class="muted small nowrap">${fmtDate(x.decided_at)}</span></a>`)}</div>`)}
    </div>
  <//>`;
}
