import { html, t, api, act, useLoad, useLive, userName, fmtAgo, Icon, Page, Empty, Loading } from '../lib.js';
import { Avatar, rich } from '../media.js';

// Mentions: the messages in which somebody wrote @you, in conversations you
// can read.
export function Mentions() {
  const st = useLoad(() => api('GET', '/notifications'));
  useLive(['mention'], st.reload);
  if (!st.data) return html`<${Page} title=${t('nav.mentions')}><${Loading} state=${st} /><//>`;
  const { items, unread } = st.data;
  const readAll = async () => { if (await act(() => api('POST', '/notifications/read', {}))) st.reload(); };
  return html`<${Page} title=${t('nav.mentions')} actions=${unread > 0 && html`<button onClick=${readAll}><${Icon} name="check" /> ${t('mention.read_all')}</button>`}>
    <p class="muted">${t('mention.intro')}</p>
    ${items.length === 0 ? html`<${Empty} icon="bell" text=${t('mention.empty')} />` : html`<div class="rows">
      ${items.map((n) => html`<a key=${n.id} href=${'#/chat/' + n.channel_id} class=${n.read ? '' : 'unread'}>
        <${Avatar} id=${n.actor_id} />
        <span class="grow"><b>${userName(n.actor_id)}</b> <span class="muted small">${n.channel ? t('mention.in_channel', { name: n.channel }) : t('mention.in_dm')} · ${fmtAgo(n.created_at)}</span>
          ${!n.read && html` <span class="badge info">${t('mention.new')}</span>`}
          <br /><span class="pre">${rich(n.snippet)}</span></span></a>`)}</div>`}
  <//>`;
}
