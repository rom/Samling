import { html, useState, useRef, useStore, t, api, apiSend, act, download, useLoad, useLive, can, getLocale, userName, linkify, linkLabel, safeUrl, fmtDate, fmtTime,
  toInput, fromInput, toDateInput, fromDateInput, Icon, Modal, Field, useForm, ShareEditor, ShareButton, SharedBadge, Page, Loading, confirmDelete, toast } from '../lib.js';
import { Attachments, ScanInto } from '../media.js';

const DAY = 86400;
const startOfDay = (d) => new Date(d.getFullYear(), d.getMonth(), d.getDate());
// dayKey gives the calendar day (YYYY-MM-DD) an instant falls on: all-day
// events are stored as UTC dates, timed events are shown in local time.
const localKey = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const eventDays = (e) => {
  const keys = [];
  if (e.all_day) {
    for (let ts = e.starts_at; ts <= e.ends_at && keys.length < 400; ts += DAY) keys.push(new Date(ts * 1000).toISOString().slice(0, 10));
  } else {
    const end = new Date(Math.max(e.ends_at - 1, e.starts_at) * 1000);
    for (let d = startOfDay(new Date(e.starts_at * 1000)); d <= end && keys.length < 400; d = new Date(d.getFullYear(), d.getMonth(), d.getDate() + 1)) keys.push(localKey(d));
  }
  return keys;
};
export const eventWhen = (e) => {
  if (e.all_day) return fmtDate(e.starts_at, true) + (e.ends_at > e.starts_at ? ' – ' + fmtDate(e.ends_at, true) : '');
  const same = new Date(e.starts_at * 1000).toDateString() === new Date(e.ends_at * 1000).toDateString();
  return fmtDate(e.starts_at) + ' ' + fmtTime(e.starts_at) + ' – ' + (same ? '' : fmtDate(e.ends_at) + ' ') + fmtTime(e.ends_at);
};

function EventForm({ event, day, onClose }) {
  const base = day ? Math.floor(new Date(day + 'T09:00').getTime() / 1000) : Math.floor(Date.now() / 3600000) * 3600 + 3600;
  const [f, set, bind, check] = useForm(event ? {
    title: event.title, description: event.description, location: event.location, url: event.url, all_day: event.all_day,
    start: event.all_day ? toDateInput(event.starts_at) : toInput(event.starts_at), end: event.all_day ? toDateInput(event.ends_at) : toInput(event.ends_at),
  } : { title: '', description: '', location: '', url: '', all_day: false, start: toInput(base), end: toInput(base + 3600) });
  const [shares, setShares] = useState([]);
  const toggleAllDay = (e) => {
    const on = e.target.checked;
    set('all_day', on);
    set('start', on ? f.start.slice(0, 10) : f.start.slice(0, 10) + 'T09:00');
    set('end', on ? f.end.slice(0, 10) : f.end.slice(0, 10) + 'T10:00');
  };
  const submit = async (e) => {
    e.preventDefault();
    const body = { title: f.title, description: f.description, location: f.location, url: f.url, all_day: f.all_day,
      starts_at: f.all_day ? fromDateInput(f.start) : fromInput(f.start), ends_at: f.all_day ? fromDateInput(f.end || f.start) : fromInput(f.end) };
    const ok = event ? await act(() => api('PATCH', '/calendar/' + event.id, body), t('common.saved')) : await act(() => api('POST', '/calendar', { ...body, shares }), t('common.saved'));
    if (ok) onClose(true);
  };
  const type = f.all_day ? 'date' : 'datetime-local';
  return html`<${Modal} title=${event ? t('cal.edit') : t('cal.new')} onClose=${() => onClose(false)}><form onSubmit=${submit}>
    <${Field} label=${t('common.title')}><input ...${bind('title')} required maxlength="200" /><//>
    <label class="check"><input type="checkbox" checked=${f.all_day} onChange=${toggleAllDay} /> ${t('cal.all_day')}</label>
    <div class="grid2">
      <${Field} label=${t('cal.starts')}><input type=${type} ...${bind('start')} required /><//>
      <${Field} label=${t('cal.ends')}><input type=${type} ...${bind('end')} min=${f.start} required /><//>
    </div>
    <${Field} label=${t('cal.location')}><input ...${bind('location')} maxlength="300" /><//>
    <${Field} label=${t('cal.link')} hint=${t('cal.link_hint')}><input type="url" ...${bind('url')} placeholder="https://" /><//>
    <${Field} label=${t('common.description')}><textarea ...${bind('description')}></textarea><//>
    <${ScanInto} value=${f.description} onChange=${(v) => set('description', v)} />
    ${!event && html`<${Field} group label=${t('share.who')}><${ShareEditor} value=${shares} onChange=${setShares} /><//>`}
    <div class="buttons"><button type="button" onClick=${() => onClose(false)}>${t('common.cancel')}</button><button class="primary">${t('common.save')}</button></div>
  </form><//>`;
}

function EventView({ event: e, me, onClose, onEdit }) {
  const mine = e.owner_id === me.id || (e.shared && can('content.moderate'));
  const url = safeUrl(e.url);
  const del = async () => { if (confirmDelete(e.title) && await act(() => api('DELETE', '/calendar/' + e.id))) onClose(true); };
  const mail = 'mailto:?subject=' + encodeURIComponent(e.title) + '&body=' + encodeURIComponent(`${e.title}\n${eventWhen(e)}\n${e.location}\n${e.url}\n\n${e.description}`);
  return html`<${Modal} title=${e.title} onClose=${() => onClose(false)}>
    <p><${Icon} name="calendar" /> ${eventWhen(e)}</p>
    ${e.location && html`<p><${Icon} name="home" /> ${e.location}</p>`}
    ${url && html`<p><a href=${url} target="_blank" rel="noopener noreferrer" class="meetlink"><${Icon} name="video" /> ${linkLabel(url) ? t('common.join', { name: linkLabel(url) }) : t('cal.open_link')}</a></p>`}
    ${e.description && html`<p class="pre">${linkify(e.description)}</p>`}
    ${!e.meeting_id && html`<${Attachments} type="event" id=${e.id} canEdit=${e.owner_id === me.id} />`}
    <p class="row"><${SharedBadge} shared=${e.shared} /><span class="muted small">${userName(e.owner_id)}</span></p>
    <div class="buttons">
      <button onClick=${() => act(() => download('GET', '/calendar/export.ics?id=' + e.id, undefined, 'event.ics'))}><${Icon} name="download" /> .ics</button>
      <a class="btn" href=${mail}><${Icon} name="mail" /> ${t('cal.mail')}</a>
      ${e.meeting_id ? html`<a class="btn primary" href=${'#/meetings/' + e.meeting_id}><${Icon} name="meeting" /> ${t('cal.open_meeting')}</a>` : mine && html`
        <${ShareButton} type="event" id=${e.id} onDone=${() => onClose(true)} />
        <button onClick=${onEdit}><${Icon} name="edit" /> ${t('common.edit')}</button>
        <button class="danger" onClick=${del}><${Icon} name="trash" /> ${t('common.delete')}</button>`}
    </div><//>`;
}

export function Calendar() {
  const s = useStore();
  const [month, setMonth] = useState(() => { const d = new Date(); return new Date(d.getFullYear(), d.getMonth(), 1); });
  const [modal, setModal] = useState(null);
  const fileInput = useRef();
  // 6 weeks starting on the Monday on or before the 1st
  const first = new Date(month.getFullYear(), month.getMonth(), 1 - ((month.getDay() + 6) % 7));
  const days = [...Array(42)].map((_, i) => new Date(first.getFullYear(), first.getMonth(), first.getDate() + i));
  const from = Math.floor(first.getTime() / 1000) - DAY, to = Math.floor(days[41].getTime() / 1000) + 2 * DAY;
  const st = useLoad(() => api('GET', `/calendar?from=${from}&to=${to}`), [from]);
  useLive(['event', 'meeting'], st.reload);
  const byDay = {};
  (st.data || []).forEach((e) => eventDays(e).forEach((k) => { (byDay[k] = byDay[k] || []).push(e); }));
  const today = localKey(new Date());
  const fmtMonth = month.toLocaleDateString(getLocale() === 'sv' ? 'sv-SE' : 'en-GB', { month: 'long', year: 'numeric' });
  const dow = [...Array(7)].map((_, i) => new Date(2024, 0, 1 + i).toLocaleDateString(getLocale() === 'sv' ? 'sv-SE' : 'en-GB', { weekday: 'short' }));
  const shift = (n) => setMonth(new Date(month.getFullYear(), month.getMonth() + n, 1));
  const importIcs = async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    const text = await file.text();
    const r = await act(() => apiSend('POST', '/calendar/import', text, 'text/calendar'));
    if (r) { toast(t('cal.imported', { n: r.imported })); st.reload(); }
    e.target.value = '';
  };
  const done = (changed) => { setModal(null); if (changed) st.reload(); };
  const upcoming = (st.data || []).filter((e) => e.ends_at + (e.all_day ? DAY : 0) >= Date.now() / 1000).slice(0, 8);
  return html`<${Page} title=${t('nav.calendar')} actions=${html`
      <input type="file" accept=".ics,text/calendar" hidden ref=${fileInput} onChange=${importIcs} />
      <button onClick=${() => fileInput.current.click()}><${Icon} name="upload" /> ${t('cal.import')}</button>
      <button onClick=${() => act(() => download('GET', '/calendar/export.ics', undefined, 'samling.ics'))}><${Icon} name="download" /> ${t('cal.export')}</button>
      <button class="primary" onClick=${() => setModal({ form: true })}><${Icon} name="plus" /> ${t('cal.new')}</button>`}>
    <div class="row" style=${{ marginBottom: '.8rem' }}>
      <button class="icon-btn" onClick=${() => shift(-1)} aria-label=${t('cal.prev')}><${Icon} name="left" /></button>
      <h2 class="grow" aria-live="polite" style=${{ textTransform: 'capitalize' }}>${fmtMonth}</h2>
      <button onClick=${() => { const d = new Date(); setMonth(new Date(d.getFullYear(), d.getMonth(), 1)); }}>${t('cal.today')}</button>
      <button class="icon-btn" onClick=${() => shift(1)} aria-label=${t('cal.next')}><${Icon} name="right" /></button>
    </div>
    ${!st.data && html`<${Loading} state=${st} />`}
    <div class="cal-grid" role="grid" aria-label=${fmtMonth}>
      ${dow.map((d) => html`<div class="dow" role="columnheader">${d}</div>`)}
      ${days.map((d) => {
        const k = localKey(d), evs = byDay[k] || [];
        return html`<div role="gridcell" key=${k} class=${'cal-day' + (d.getMonth() !== month.getMonth() ? ' other' : '') + (k === today ? ' today' : '')}
          onClick=${(ev) => { if (ev.target === ev.currentTarget || ev.target.classList.contains('num')) setModal({ form: true, day: k }); }}>
          <button class="num link" aria-label=${t('cal.add_on', { date: d.toLocaleDateString() })} onClick=${(ev) => { ev.stopPropagation(); setModal({ form: true, day: k }); }}>${d.getDate()}</button>
          ${evs.slice(0, 3).map((e) => html`<button key=${e.id} class=${'cal-ev' + (e.meeting_id ? ' meeting' : e.shared ? '' : ' private')} title=${e.title}
            onClick=${(ev) => { ev.stopPropagation(); setModal({ view: e }); }}>${e.all_day ? '' : fmtTime(e.starts_at) + ' '}${e.title}</button>`)}
          ${evs.length > 3 && html`<span class="cal-more">+${evs.length - 3}</span>`}
        </div>`;
      })}
    </div>
    <h2 style=${{ margin: '1.4rem 0 .6rem' }}>${t('cal.upcoming')}</h2>
    ${upcoming.length === 0 ? html`<p class="muted">${t('cal.none')}</p>` : html`<div class="rows">${upcoming.map((e) => html`<a href="#/calendar" key=${e.id}
      onClick=${(ev) => { ev.preventDefault(); setModal({ view: e }); }}><${Icon} name=${e.meeting_id ? 'meeting' : 'calendar'} />
      <span class="grow"><b>${e.title}</b><br /><span class="muted small">${eventWhen(e)}${e.location ? ' · ' + e.location : ''}</span></span><${SharedBadge} shared=${e.shared} /></a>`)}</div>`}
    ${modal && modal.form && html`<${EventForm} event=${modal.event} day=${modal.day} onClose=${done} />`}
    ${modal && modal.view && html`<${EventView} event=${modal.view} me=${s.me} onClose=${done} onEdit=${() => setModal({ form: true, event: modal.view })} />`}
  <//>`;
}
