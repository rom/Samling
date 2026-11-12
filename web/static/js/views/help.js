// Online help: a searchable list of topics (#/help) and one page per topic
// (#/help/<topic>). All texts live in i18n/guide-*.js as plain strings in a
// small line-based format that is parsed here and rendered as elements only:
//   "# "  sub-heading        "1. " numbered step      "- "  bullet
//   "> "  note               blank line = new paragraph
//   [[some.key]]  the label of a button or menu, taken from the dictionary
//   **text**      emphasis
import { html, useState, useEffect, useMemo, useStore, t, hasKey, can, getLocale, Icon, Page, Empty } from '../lib.js';
import { startTour } from './tour.js';

// id: also the first route segment for the topics that describe a section.
// open: the section the topic links to. related: other topics worth reading.
const topics = [
  { id: 'start', group: 'basics', icon: 'flag', related: ['sharing', 'settings', 'install'] },
  { id: 'sharing', group: 'basics', icon: 'share', related: ['people', 'privacy', 'chat'] },
  { id: 'home', group: 'sections', icon: 'home', open: 'home', related: ['start', 'chat', 'calendar'] },
  { id: 'chat', group: 'sections', icon: 'chat', open: 'chat', related: ['sharing', 'people', 'camera'] },
  { id: 'documents', group: 'sections', icon: 'doc', open: 'documents', related: ['sharing', 'search'] },
  { id: 'lists', group: 'sections', icon: 'list', open: 'lists', related: ['boards', 'sharing', 'camera'] },
  { id: 'polls', group: 'sections', icon: 'poll', open: 'polls', related: ['decisions', 'sharing'] },
  { id: 'calendar', group: 'sections', icon: 'calendar', open: 'calendar', related: ['meetings', 'sharing'] },
  { id: 'meetings', group: 'sections', icon: 'meeting', open: 'meetings', related: ['decisions', 'calendar'] },
  { id: 'decisions', group: 'sections', icon: 'decision', open: 'decisions', related: ['meetings', 'polls'] },
  { id: 'boards', group: 'sections', icon: 'board', open: 'boards', related: ['lists', 'sharing', 'keyboard'] },
  { id: 'search', group: 'sections', icon: 'search', open: 'search', related: ['documents', 'privacy'] },
  { id: 'people', group: 'sections', icon: 'people', open: 'people', related: ['sharing', 'chat', 'privacy'] },
  { id: 'history', group: 'sections', icon: 'history', open: 'history', related: ['privacy', 'settings'] },
  { id: 'settings', group: 'sections', icon: 'settings', open: 'settings', related: ['accessibility', 'privacy'] },
  { id: 'privacy', group: 'more', icon: 'lock', open: 'settings', related: ['sharing', 'settings', 'history'] },
  { id: 'camera', group: 'more', icon: 'camera', related: ['chat', 'lists', 'privacy'] },
  { id: 'accessibility', group: 'more', icon: 'a11y', open: 'settings', related: ['keyboard', 'settings'] },
  { id: 'install', group: 'more', icon: 'phone', related: ['start', 'accessibility'] },
  { id: 'keyboard', group: 'more', icon: 'keyboard', related: ['accessibility', 'boards'] },
  { id: 'admin', group: 'admin', icon: 'admin', open: 'admin', admin: true, related: ['sharing', 'people', 'privacy'] },
];
const byId = Object.fromEntries(topics.map((x) => [x.id, x]));
const groups = ['basics', 'sections', 'more', 'admin'];
const isAdmin = () => can('users.manage') || can('roles.manage') || can('logs.view') || can('backup.manage');

// A few icons that the shared icon set lacks.
const extra = {
  flag: 'M5 21V4M5 4h13l-2.500 4 2.500 4H5',
  a11y: 'M12 21a9 9 0 100-18 9 9 0 000 18zM12 7.500h.01M7.500 10l4.500 1 4.500-1M12 11v3.500M12 14.500l-2 3.500M12 14.500l2 3.500',
  phone: 'M7.500 3h9v18h-9zM11 18h2',
  keyboard: 'M3 7h18v10H3zM7 10.500h.01M10.300 10.500h.01M13.700 10.500h.01M17 10.500h.01M8 14h8',
};
function TopicIcon({ name }) {
  if (!extra[name]) return html`<${Icon} name=${name} />`;
  return html`<svg class="icon" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.800"
    stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d=${extra[name]} /></svg>`;
}

// ---------- the text format ----------

const label = (key) => (hasKey(key) ? t(key) : key);

// inline renders [[key]] and **text** inside a line.
function inline(text) {
  return text.split(/(\[\[[a-z0-9_.]+\]\]|\*\*[^*]+\*\*)/).map((part) => {
    if (part.startsWith('[[') && part.endsWith(']]')) return html`<b class="help-ui">${label(part.slice(2, -2))}</b>`;
    if (part.startsWith('**') && part.endsWith('**') && part.length > 4) return html`<b>${part.slice(2, -2)}</b>`;
    return part;
  });
}

// parse turns a text into blocks: {type: 'h'|'p'|'note', text} or {type: 'ol'|'ul', items}.
function parse(text) {
  const blocks = [];
  let open = null; // the list or paragraph that following lines continue
  for (const raw of text.split('\n')) {
    const line = raw.trim();
    let m;
    if (!line) open = null;
    else if (line.startsWith('# ')) { blocks.push({ type: 'h', text: line.slice(2) }); open = null; }
    else if (line.startsWith('> ')) { blocks.push({ type: 'note', text: line.slice(2) }); open = null; }
    else if ((m = /^(\d+\.|-) (.*)$/.exec(line))) {
      const type = m[1] === '-' ? 'ul' : 'ol';
      if (!open || open.type !== type) blocks.push(open = { type, items: [] });
      open.items.push(m[2]);
    } else if (open && open.type === 'p') open.text += ' ' + line;
    else blocks.push(open = { type: 'p', text: line });
  }
  return blocks;
}

function Body({ text }) {
  return parse(text).map((b) => {
    if (b.type === 'h') return html`<h2>${inline(b.text)}</h2>`;
    if (b.type === 'ol') return html`<ol>${b.items.map((x) => html`<li>${inline(x)}</li>`)}</ol>`;
    if (b.type === 'ul') return html`<ul>${b.items.map((x) => html`<li>${inline(x)}</li>`)}</ul>`;
    if (b.type === 'note') return html`<p class="help-note"><${Icon} name="help" /> <span>${inline(b.text)}</span></p>`;
    return html`<p>${inline(b.text)}</p>`;
  });
}

// plain is the text as read, for searching.
const plain = (text) => text.replace(/\[\[([a-z0-9_.]+)\]\]/g, (_, k) => label(k)).replace(/\*\*/g, '').replace(/^(# |> |- |\d+\. )/gm, '');

// ---------- views ----------

function TopicLink({ x }) {
  return html`<a class="help-topic" href=${'#/help/' + x.id}>
    <span class="help-ico" aria-hidden="true"><${TopicIcon} name=${x.icon} /></span>
    <span class="grow"><b>${t('help.' + x.id + '.title')}</b><br /><span class="muted small">${t('help.' + x.id + '.intro')}</span></span>
    <${Icon} name="right" /></a>`;
}

function Topic({ x }) {
  useEffect(() => { // start at the top when moving between topics
    const main = document.getElementById('main');
    if (main) main.scrollTop = 0;
  }, [x.id]);
  const related = x.related.map((id) => byId[id]).filter((r) => r && (!r.admin || isAdmin()));
  return html`<${Page} title=${t('help.' + x.id + '.title')} back="#/help">
    <article class="help-article">
      <p class="help-lead">${t('help.' + x.id + '.intro')}</p>
      <${Body} text=${t('help.' + x.id + '.body')} />
      ${x.open && html`<p class="no-print"><a class="btn primary" href=${'#/' + x.open}><${Icon} name="right" /> ${t('help.open', { name: t('nav.' + x.open) })}</a></p>`}
    </article>
    <nav class="help-related no-print" aria-label=${t('help.related')}>
      <h2>${t('help.related')}</h2>
      <div class="rows">${related.map((r) => html`<${TopicLink} x=${r} key=${r.id} />`)}</div>
      <p><a href="#/help"><${Icon} name="left" /> ${t('help.all')}</a></p>
    </nav>
  <//>`;
}

export function Help({ route }) {
  useStore(); // language and permissions may change while the page is open
  const [q, setQ] = useState('');
  const locale = getLocale();
  const texts = useMemo(() => Object.fromEntries(topics.map((x) => [x.id, {
    title: t('help.' + x.id + '.title').toLowerCase(),
    all: (t('help.' + x.id + '.title') + ' ' + t('help.' + x.id + '.intro') + ' ' + plain(t('help.' + x.id + '.body'))).toLowerCase(),
  }])), [locale]);
  const topic = route[1] && byId[route[1]];
  if (topic) return html`<${Topic} x=${topic} key=${topic.id} />`;

  const admin = isAdmin();
  const words = q.toLowerCase().split(/\s+/).filter(Boolean);
  const searching = words.length > 0;
  const hits = searching
    ? topics.filter((x) => words.every((w) => texts[x.id].all.includes(w)))
      .sort((a, b) => words.some((w) => texts[b.id].title.includes(w)) - words.some((w) => texts[a.id].title.includes(w)))
    : [];
  return html`<${Page} title=${t('nav.help')}>
    <div class="help-intro card">
      <img src="/icons/icon.svg" alt="" width="56" height="56" />
      <div class="grow"><p>${t('help.intro')}</p>
        <button type="button" onClick=${startTour}><${TopicIcon} name="flag" /> ${t('help.tour_again')}</button></div>
    </div>
    <div class="help-search" role="search">
      <${Icon} name="search" />
      <input type="search" value=${q} onInput=${(e) => setQ(e.target.value)} placeholder=${t('help.search')} aria-label=${t('help.search')} maxlength="100" />
    </div>
    <p class="sr-only" role="status">${searching ? t('help.count', { n: hits.length }) : ''}</p>
    ${searching && (hits.length === 0 ? html`<${Empty} icon="search" text=${t('help.none')} />`
      : html`<div class="rows">${hits.map((x) => html`<${TopicLink} x=${x} key=${x.id} />`)}</div>`)}
    ${!searching && groups.filter((g) => g !== 'admin' || admin).map((g) => html`<section class="help-group" key=${g}>
      <h2>${t('help.group.' + g)}</h2>
      <div class="rows">${topics.filter((x) => x.group === g).map((x) => html`<${TopicLink} x=${x} key=${x.id} />`)}</div>
    </section>`)}
  <//>`;
}
