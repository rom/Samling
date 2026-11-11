import { html, useState, useEffect, useRef, useStore, t, api, act, download, uploadFile, useLoad, useLive, go, can, userName, linkify, fmtAgo, fmtSize,
  Icon, Modal, Field, useForm, ShareEditor, ShareButton, Empty, Loading, confirmDelete, toast, errText } from '../lib.js';

const dmName = (ch, me) => (ch.members || []).filter((id) => id !== me.id).map(userName).join(', ') || userName(me.id);
const chName = (ch, me) => (ch.kind === 'dm' ? dmName(ch, me) : ch.name);

function NewChannel({ onClose }) {
  const [f, , bind] = useForm({ name: '', topic: '' });
  const [shares, setShares] = useState([{ type: 'all', id: '', level: 2 }]);
  const submit = async (e) => {
    e.preventDefault();
    const r = await act(() => api('POST', '/channels', { ...f, shares }));
    if (r) { onClose(); go('chat', r.id); }
  };
  return html`<${Modal} title=${t('chat.new_channel')} onClose=${onClose}><form onSubmit=${submit}>
    <${Field} label=${t('common.name')}><input ...${bind('name')} required maxlength="60" /><//>
    <${Field} label=${t('chat.topic')}><input ...${bind('topic')} maxlength="300" /><//>
    <${Field} group label=${t('share.who')} hint=${t('chat.share_hint')}><${ShareEditor} value=${shares} onChange=${setShares} /><//>
    <div class="buttons"><button type="button" onClick=${onClose}>${t('common.cancel')}</button><button class="primary">${t('common.create')}</button></div>
  </form><//>`;
}

function NewDM({ onClose }) {
  const s = useStore();
  const [sel, setSel] = useState([]);
  const [q, setQ] = useState('');
  const submit = async (e) => {
    e.preventDefault();
    const r = await act(() => api('POST', '/dm', { user_ids: sel }));
    if (r) { onClose(); go('chat', r.id); }
  };
  const people = s.users.filter((u) => u.id !== s.me.id && (u.display_name + u.username).toLowerCase().includes(q.toLowerCase()));
  return html`<${Modal} title=${t('chat.new_dm')} onClose=${onClose}><form onSubmit=${submit}>
    <${Field} label=${t('common.filter')}><input value=${q} onInput=${(e) => setQ(e.target.value)} /><//>
    <div class="rows" role="group" aria-label=${t('chat.recipients')}>${people.map((u) => html`<label key=${u.id}>
      <input type="checkbox" checked=${sel.includes(u.id)} onChange=${(e) => setSel(e.target.checked ? [...sel, u.id] : sel.filter((x) => x !== u.id))} />
      <span>${u.display_name} <span class="muted small">@${u.username}</span></span></label>`)}
      ${people.length === 0 && html`<div class="muted">${t('common.nothing')}</div>`}</div>
    <div class="buttons"><button type="button" onClick=${onClose}>${t('common.cancel')}</button>
      <button class="primary" disabled=${sel.length === 0}>${t('chat.start')}</button></div>
  </form><//>`;
}

export function FileChip({ file }) {
  return html`<button type="button" class="file-chip" title=${t('common.download')} onClick=${() => act(() => download('GET', '/files/' + file.id, undefined, file.name))}>
    <${Icon} name="clip" /><span>${file.name}</span><span class="muted nowrap">${fmtSize(file.size)}</span></button>`;
}

function Message({ m, ch, me, reload }) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(m.body);
  const mine = m.user_id === me.id;
  const save = async () => { if (await act(() => api('PATCH', '/messages/' + m.id, { body: text }))) { setEditing(false); reload(); } };
  const del = async () => { if (confirmDelete(t('chat.this_message')) && await act(() => api('DELETE', '/messages/' + m.id))) reload(); };
  const pin = async () => { if (await act(() => api(m.pinned_at ? 'DELETE' : 'POST', `/messages/${m.id}/pin`))) reload(); };
  if (m.deleted) return html`<div class="msg"><span class="avatar">–</span><div class="body muted"><i>${t('chat.deleted')}</i></div></div>`;
  return html`<div class=${'msg' + (m.pinned_at ? ' pinned' : '')}>
    <span class="avatar" aria-hidden="true">${userName(m.user_id).slice(0, 1).toUpperCase()}</span>
    <div class="body">
      <div class="meta"><b>${userName(m.user_id)}</b><time>${fmtAgo(m.created_at)}</time>
        ${m.edited_at > 0 && html` · ${t('chat.edited')}`}${m.pinned_at > 0 && html` · <${Icon} name="pin" /> ${t('chat.pinned')}`}</div>
      ${editing
        ? html`<textarea value=${text} onInput=${(e) => setText(e.target.value)} aria-label=${t('common.edit')}></textarea>
            <div class="row"><button class="primary" onClick=${save}>${t('common.save')}</button><button onClick=${() => setEditing(false)}>${t('common.cancel')}</button></div>`
        : html`<div class="pre">${linkify(m.body)}</div>`}
      ${m.files.length > 0 && html`<div class="files">${m.files.map((f) => html`<${FileChip} file=${f} key=${f.id} />`)}</div>`}
    </div>
    <div class="tools">
      ${ch.level >= 2 && html`<button class="icon-btn" onClick=${pin} aria-label=${m.pinned_at ? t('chat.unpin') : t('chat.pin')} title=${m.pinned_at ? t('chat.unpin') : t('chat.pin')}><${Icon} name="pin" /></button>`}
      ${mine && html`<button class="icon-btn" onClick=${() => { setText(m.body); setEditing(true); }} aria-label=${t('common.edit')} title=${t('common.edit')}><${Icon} name="edit" /></button>`}
      ${(mine || ch.level >= 3) && html`<button class="icon-btn" onClick=${del} aria-label=${t('common.delete')} title=${t('common.delete')}><${Icon} name="trash" /></button>`}
    </div>
  </div>`;
}

function Conversation({ ch, me, onChanged }) {
  const [msgs, setMsgs] = useState(null);
  const [more, setMore] = useState(true);
  const [pins, setPins] = useState([]);
  const [showPins, setShowPins] = useState(false);
  const [text, setText] = useState('');
  const [files, setFiles] = useState([]);
  const [busy, setBusy] = useState(false);
  const box = useRef();
  const fileInput = useRef();
  const stick = useRef(true);

  const load = async () => {
    try {
      const [m, p] = await Promise.all([api('GET', `/channels/${ch.id}/messages?limit=60`), api('GET', `/channels/${ch.id}/pins`)]);
      setMsgs((old) => {
        // keep older pages that were already loaded
        if (!old || !m.length) return m;
        const older = old.filter((x) => x.id < m[0].id);
        return [...older, ...m];
      });
      if (m.length < 60) setMore(false);
      setPins(p);
      api('POST', `/channels/${ch.id}/read`).then(onChanged).catch(() => {});
    } catch (e) { toast(errText(e), 'error'); }
  };
  useEffect(() => { setMsgs(null); setMore(true); stick.current = true; load(); }, [ch.id]);
  useLive(['channel'], (e) => { if (e.id === ch.id) load(); });
  useEffect(() => { if (stick.current && box.current) box.current.scrollTop = box.current.scrollHeight; }, [msgs]);
  const onScroll = () => { const el = box.current; stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80; };
  const loadOlder = async () => {
    const older = await act(() => api('GET', `/channels/${ch.id}/messages?limit=60&before=${msgs[0].id}`));
    if (older) { stick.current = false; setMsgs([...older, ...msgs]); if (older.length < 60) setMore(false); }
  };
  const attach = async (e) => {
    setBusy(true);
    for (const file of e.target.files) {
      const r = await act(() => uploadFile(file));
      if (r) setFiles((old) => [...old, r]);
    }
    e.target.value = '';
    setBusy(false);
  };
  const send = async (e) => {
    if (e) e.preventDefault();
    if (!text.trim() && !files.length) return;
    setBusy(true);
    stick.current = true;
    if (await act(() => api('POST', `/channels/${ch.id}/messages`, { body: text, file_ids: files.map((f) => f.id) }))) { setText(''); setFiles([]); load(); }
    setBusy(false);
  };
  const keydown = (e) => { if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) send(e); };
  const del = async () => { if (confirmDelete(ch.name) && await act(() => api('DELETE', '/channels/' + ch.id))) { onChanged(); go('chat'); } };

  return html`<div class="conv">
    <div class="conv-head">
      <a class="icon-btn chat-back" href="#/chat" aria-label=${t('common.back')}><${Icon} name="left" /></a>
      <div class="grow"><h1>${ch.kind === 'dm' ? '' : '# '}${chName(ch, me)}</h1>${ch.topic && html`<div class="muted small">${linkify(ch.topic)}</div>`}</div>
      <button onClick=${() => setShowPins(!showPins)} aria-expanded=${showPins}><${Icon} name="pin" /> ${t('chat.pins')} (${pins.length})</button>
      ${ch.kind === 'channel' && ch.level >= 3 && html`<${ShareButton} type="channel" id=${ch.id} onDone=${onChanged} />
        <button class="icon-btn danger" onClick=${del} aria-label=${t('common.delete')} title=${t('common.delete')}><${Icon} name="trash" /></button>`}
    </div>
    ${showPins && html`<div class="pins" role="region" aria-label=${t('chat.pins')}>
      ${pins.length === 0 ? html`<span class="muted">${t('chat.no_pins')}</span>` : pins.map((p) => html`<div class="pin" key=${p.id}>
        <${Icon} name="pin" /><div class="grow"><b>${userName(p.user_id)}</b> <span class="muted small">${fmtAgo(p.created_at)}</span>
          <div class="pre">${linkify(p.body)}</div>
          ${p.files.length > 0 && html`<div class="files">${p.files.map((f) => html`<${FileChip} file=${f} key=${f.id} />`)}</div>`}</div></div>`)}
    </div>`}
    <div class="msgs" ref=${box} onScroll=${onScroll} role="log" aria-live="polite" aria-label=${t('chat.messages')}>
      ${msgs === null ? html`<div class="loading">${t('common.loading')}</div>` : html`
        ${more && msgs.length > 0 && html`<button class="link" onClick=${loadOlder}>${t('chat.older')}</button>`}
        ${msgs.length === 0 && html`<${Empty} icon="chat" text=${t('chat.empty')} />`}
        ${msgs.map((m) => html`<${Message} key=${m.id} m=${m} ch=${ch} me=${me} reload=${load} />`)}`}
    </div>
    ${ch.level >= 2 ? html`<form class="composer" onSubmit=${send}>
      ${files.length > 0 && html`<div class="files">${files.map((f) => html`<span class="file-chip" key=${f.id}><${Icon} name="clip" /><span>${f.name}</span>
        <button type="button" class="link" aria-label=${t('common.remove')} onClick=${() => setFiles(files.filter((x) => x.id !== f.id))}><${Icon} name="x" /></button></span>`)}</div>`}
      <div class="line">
        <input type="file" multiple hidden ref=${fileInput} onChange=${attach} />
        <button type="button" class="icon-btn" onClick=${() => fileInput.current.click()} aria-label=${t('chat.attach')} title=${t('chat.attach')} disabled=${busy}><${Icon} name="clip" /></button>
        <textarea rows="1" value=${text} onInput=${(e) => setText(e.target.value)} onKeyDown=${keydown} placeholder=${t('chat.placeholder')} aria-label=${t('chat.placeholder')} maxlength="10000"></textarea>
        <button class="primary" disabled=${busy} aria-label=${t('chat.send')}><${Icon} name="send" /></button>
      </div></form>` : html`<div class="composer muted"><${Icon} name="lock" /> ${t('chat.readonly')}</div>`}
  </div>`;
}

export function Chat({ route }) {
  const s = useStore();
  const st = useLoad(() => api('GET', '/channels'));
  const [modal, setModal] = useState(null);
  useLive(['channel'], st.reload);
  if (!st.data) return html`<${Loading} state=${st} />`;
  const channels = st.data.filter((c) => c.kind === 'channel');
  const dms = st.data.filter((c) => c.kind === 'dm').sort((a, b) => b.last_at - a.last_at);
  const current = st.data.find((c) => c.id === route[1]);
  const item = (c) => html`<a href=${'#/chat/' + c.id} key=${c.id} aria-current=${current && current.id === c.id ? 'true' : null} class=${c.unread ? 'unread' : ''}>
    ${c.kind === 'dm' ? html`<${Icon} name="people" />` : html`<span aria-hidden="true">#</span>`}
    <span class="grow">${chName(c, s.me)}</span>${c.level < 2 && html`<${Icon} name="lock" label=${t('chat.readonly')} />`}
    ${c.unread > 0 && html`<span class="count" aria-label=${t('chat.unread', { n: c.unread })}>${c.unread}</span>`}</a>`;
  return html`<div class=${'chat' + (current ? ' has-conv' : '')}>
    <nav class="chan-list" aria-label=${t('nav.chat')}>
      <h2><span class="grow">${t('chat.channels')}</span>${can('channels.create') && html`<button class="icon-btn" onClick=${() => setModal('channel')}
        aria-label=${t('chat.new_channel')} title=${t('chat.new_channel')}><${Icon} name="plus" /></button>`}</h2>
      ${channels.map(item)}
      <h2><span class="grow">${t('chat.dms')}</span><button class="icon-btn" onClick=${() => setModal('dm')} aria-label=${t('chat.new_dm')} title=${t('chat.new_dm')}><${Icon} name="plus" /></button></h2>
      ${dms.map(item)}
      ${dms.length === 0 && html`<p class="muted small" style=${{ padding: '0 1rem' }}>${t('chat.no_dms')}</p>`}
    </nav>
    ${current ? html`<${Conversation} ch=${current} me=${s.me} onChanged=${st.reload} key=${current.id} />`
      : html`<div class="conv"><${Empty} icon="chat" text=${t('chat.pick')} /></div>`}
    ${modal === 'channel' && html`<${NewChannel} onClose=${() => { setModal(null); st.reload(); }} />`}
    ${modal === 'dm' && html`<${NewDM} onClose=${() => { setModal(null); st.reload(); }} />`}
  </div>`;
}
