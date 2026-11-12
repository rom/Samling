import { html, useState, useEffect, useRef, useStore, S, t, api, act, uploadFile, useLoad, useLive, go, can, userName, linkify, fmtAgo, setBlocked, copyText, pageUrl,
  Icon, Modal, Field, useForm, ShareEditor, ShareButton, Empty, Loading, confirmDelete, toast, errText } from '../lib.js';
import { Avatar, rich, FormatBar, formatKey, EmojiButton, reactionEmojis, CameraButtons, FileView, LinkButton, insertAtCursor } from '../media.js';

const dmName = (ch, me) => (ch.members || []).filter((id) => id !== me.id).map(userName).join(', ') || userName(me.id);
const chName = (ch, me) => (ch.kind === 'dm' ? dmName(ch, me) : ch.name);

function NewChannel({ onClose }) {
  const [f, , bind] = useForm({ name: '', topic: '', description: '' });
  const [shares, setShares] = useState([{ type: 'all', id: '', level: 2 }]);
  const submit = async (e) => {
    e.preventDefault();
    const r = await act(() => api('POST', '/channels', { ...f, shares }));
    if (r) { onClose(); go('chat', r.id); }
  };
  return html`<${Modal} title=${t('chat.new_channel')} onClose=${onClose}><form onSubmit=${submit}>
    <${Field} label=${t('common.name')}><input ...${bind('name')} required maxlength="60" /><//>
    <${Field} label=${t('chat.topic')}><input ...${bind('topic')} maxlength="300" /><//>
    <${Field} label=${t('common.description')} hint=${t('chat.description_hint')}><textarea ...${bind('description')} rows="3" maxlength="2000"></textarea><//>
    <${Field} group label=${t('share.who')} hint=${t('chat.share_hint')}><${ShareEditor} value=${shares} onChange=${setShares} /><//>
    <div class="buttons"><button type="button" onClick=${onClose}>${t('common.cancel')}</button><button class="primary">${t('common.create')}</button></div>
  </form><//>`;
}

// EditChannel: name, topic and description, for the owner of the channel
// and for administrators.
function EditChannel({ ch, onClose }) {
  const [f, , bind] = useForm({ name: ch.name, topic: ch.topic, description: ch.description || '' });
  const submit = async (e) => {
    e.preventDefault();
    if (await act(() => api('PATCH', '/channels/' + ch.id, f), t('common.saved'))) onClose(true);
  };
  return html`<${Modal} title=${t('chat.edit_channel')} onClose=${() => onClose(false)}><form onSubmit=${submit}>
    <${Field} label=${t('common.name')}><input ...${bind('name')} required maxlength="60" /><//>
    <${Field} label=${t('chat.topic')}><input ...${bind('topic')} maxlength="300" /><//>
    <${Field} label=${t('common.description')} hint=${t('chat.description_hint')}><textarea ...${bind('description')} rows="4" maxlength="2000"></textarea><//>
    <div class="buttons"><button type="button" onClick=${() => onClose(false)}>${t('common.cancel')}</button><button class="primary">${t('common.save')}</button></div>
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
      <${Avatar} user=${u} size="sm" /><span>${u.display_name} <span class="muted small">@${u.username}</span></span></label>`)}
      ${people.length === 0 && html`<div class="muted">${t('common.nothing')}</div>`}</div>
    <div class="buttons"><button type="button" onClick=${onClose}>${t('common.cancel')}</button>
      <button class="primary" disabled=${sel.length === 0}>${t('chat.start')}</button></div>
  </form><//>`;
}

function Message({ m, ch, me, reload, onReply, reply }) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(m.body);
  const mine = m.user_id === me.id;
  const save = async () => { if (await act(() => api('PATCH', '/messages/' + m.id, { body: text }))) { setEditing(false); reload(); } };
  const del = async () => { if (confirmDelete(t('chat.this_message')) && await act(() => api('DELETE', '/messages/' + m.id))) reload(); };
  const pin = async () => { if (await act(() => api(m.pinned_at ? 'DELETE' : 'POST', `/messages/${m.id}/pin`))) reload(); };
  const react = async (emoji) => { if (await act(() => api('POST', `/messages/${m.id}/reactions`, { emoji }))) reload(); };
  if (m.deleted) return html`<div class=${'msg' + (reply ? ' reply' : '')}><span class="avatar">–</span><div class="body muted"><i>${t('chat.deleted')}</i></div></div>`;
  return html`<div id=${'m-' + m.id} class=${'msg' + (m.pinned_at ? ' pinned' : '') + (reply ? ' reply' : '')}>
    <${Avatar} id=${m.user_id} size=${reply ? 'sm' : ''} />
    <div class="body">
      <div class="meta"><b>${userName(m.user_id)}</b>${S.userMap[m.user_id] && html`<span class="handle">@${S.userMap[m.user_id].username}</span>`}<time>${fmtAgo(m.created_at)}</time>
        ${m.edited_at > 0 && html` · ${t('chat.edited')}`}${m.pinned_at > 0 && html` · <${Icon} name="pin" /> ${t('chat.pinned')}`}</div>
      ${editing
        ? html`<textarea value=${text} onInput=${(e) => setText(e.target.value)} onKeyDown=${(e) => formatKey(e, setText)} aria-label=${t('common.edit')}></textarea>
            <div class="row"><button class="primary" onClick=${save}>${t('common.save')}</button><button onClick=${() => setEditing(false)}>${t('common.cancel')}</button></div>`
        : html`<div class="pre">${rich(m.body)}</div>`}
      ${m.files.length > 0 && html`<div class="files">${m.files.map((f) => html`<${FileView} file=${f} key=${f.id} />`)}</div>`}
      ${m.reactions.length > 0 && html`<div class="reactions">${m.reactions.map((r) => html`<button type="button" key=${r.emoji} class="reaction" aria-pressed=${r.mine}
        aria-label=${t('chat.reaction_n', { emoji: r.emoji, n: r.count })} title=${r.mine ? t('chat.reaction_remove') : t('chat.react')} onClick=${() => react(r.emoji)}>${r.emoji} <b>${r.count}</b></button>`)}</div>`}
    </div>
    <div class="tools">
      <${EmojiButton} set=${reactionEmojis} label=${t('chat.react')} onPick=${react} />
      ${onReply && html`<button class="icon-btn" onClick=${onReply} aria-label=${t('chat.comment')} title=${t('chat.comment')}><${Icon} name="reply" /></button>`}
      <button class="icon-btn" onClick=${() => copyText(pageUrl('chat', ch.id, m.id), t('chat.link_copied'))} aria-label=${t('chat.copy_link')} title=${t('chat.copy_link')}><${Icon} name="link" /></button>
      ${ch.level >= 2 && html`<button class="icon-btn" onClick=${pin} aria-label=${m.pinned_at ? t('chat.unpin') : t('chat.pin')} title=${m.pinned_at ? t('chat.unpin') : t('chat.pin')}><${Icon} name="pin" /></button>`}
      ${mine && html`<button class="icon-btn" onClick=${() => { setText(m.body); setEditing(true); }} aria-label=${t('common.edit')} title=${t('common.edit')}><${Icon} name="edit" /></button>`}
      ${(mine || ch.level >= 3) && html`<button class="icon-btn" onClick=${del} aria-label=${t('common.delete')} title=${t('common.delete')}><${Icon} name="trash" /></button>`}
    </div>
  </div>`;
}

// CommentBox is the small composer under a message.
function CommentBox({ onSend, onCancel }) {
  const [text, setText] = useState('');
  const ref = useRef();
  useEffect(() => { ref.current.focus(); }, []);
  const submit = async (e) => { e.preventDefault(); if (text.trim() && await onSend(text)) setText(''); };
  return html`<form class="comment-box" onSubmit=${submit}>
    <input ref=${ref} value=${text} onInput=${(e) => setText(e.target.value)} onKeyDown=${(e) => { if (e.key === 'Escape') { e.stopPropagation(); onCancel(); } else formatKey(e, setText); }}
      placeholder=${t('chat.comment_placeholder')} aria-label=${t('chat.comment_placeholder')} maxlength="10000" />
    <${EmojiButton} label=${t('fmt.emoji')} onPick=${(e) => setText(text + e)} />
    <button class="primary" aria-label=${t('chat.send_comment')} title=${t('chat.send_comment')}><${Icon} name="send" /></button>
    <button type="button" class="icon-btn" onClick=${onCancel} aria-label=${t('common.cancel')} title=${t('common.cancel')}><${Icon} name="x" /></button></form>`;
}

function Conversation({ ch, me, onChanged, focus }) {
  const [editing, setEditing] = useState(false);
  const sought = useRef(0);
  const [msgs, setMsgs] = useState(null);
  const [more, setMore] = useState(true);
  const [pins, setPins] = useState([]);
  const [showPins, setShowPins] = useState(false);
  const [text, setText] = useState('');
  const [files, setFiles] = useState([]);
  const [busy, setBusy] = useState(false);
  const [replyTo, setReplyTo] = useState(null);
  const box = useRef();
  const input = useRef();
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
  // A link to a message (#/chat/<channel>/<message>) scrolls to it. A
  // message further back is looked for in a few older pages.
  useEffect(() => {
    if (!focus || !msgs) return;
    const el = document.getElementById('m-' + focus);
    if (el) {
      stick.current = false;
      el.scrollIntoView({ block: 'center' });
      el.classList.add('target');
      setTimeout(() => el.classList.remove('target'), 4000);
      go('chat', ch.id);
    } else if (more && msgs.length && sought.current < 5) {
      sought.current++;
      loadOlder();
    } else {
      go('chat', ch.id);
    }
  }, [focus, msgs]);
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
  // Typing "@" and a few letters suggests people to mention.
  const [sugg, setSugg] = useState(null);
  const typed = (e) => {
    setText(e.target.value);
    const m = /(^|[^A-Za-z0-9._@/-])@([A-Za-z0-9._-]{0,31})$/.exec(e.target.value.slice(0, e.target.selectionStart));
    setSugg(m ? { q: m[2].toLowerCase(), at: e.target.selectionStart - m[2].length - 1, i: 0 } : null);
  };
  const candidates = sugg ? S.users.filter((u) => u.id !== me.id && (ch.kind !== 'dm' || (ch.members || []).includes(u.id))
    && (u.username.startsWith(sugg.q) || u.display_name.toLowerCase().split(/\s+/).some((w) => w.startsWith(sugg.q)))).slice(0, 6) : [];
  const mention = (u) => {
    const el = input.current, end = el.selectionStart;
    setText(text.slice(0, sugg.at) + '@' + u.username + ' ' + text.slice(end));
    setSugg(null);
    const pos = sugg.at + u.username.length + 2;
    requestAnimationFrame(() => { el.focus(); el.setSelectionRange(pos, pos); });
  };
  const keydown = (e) => {
    if (sugg && candidates.length) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') { e.preventDefault(); setSugg({ ...sugg, i: (sugg.i + (e.key === 'ArrowDown' ? 1 : candidates.length - 1)) % candidates.length }); return; }
      if (e.key === 'Enter' || e.key === 'Tab') { e.preventDefault(); mention(candidates[Math.min(sugg.i, candidates.length - 1)]); return; }
      if (e.key === 'Escape') { e.stopPropagation(); setSugg(null); return; }
    }
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) send(e); else formatKey(e, setText);
  };
  const comment = async (parent, body) => {
    const ok = await act(() => api('POST', `/channels/${ch.id}/messages`, { body, parent_id: parent }));
    if (ok) { setReplyTo(null); load(); }
    return ok;
  };
  const addPhoto = async (file) => { setBusy(true); const r = await act(() => uploadFile(file)); if (r) setFiles((old) => [...old, r]); setBusy(false); };
  // Comments are shown under the message they belong to.
  const byId = {}, kids = {};
  (msgs || []).forEach((m) => { byId[m.id] = m; });
  const roots = (msgs || []).filter((m) => {
    if (!m.parent_id || !byId[m.parent_id]) return true;
    (kids[m.parent_id] = kids[m.parent_id] || []).push(m);
    return false;
  });
  const other = ch.kind === 'dm' && (ch.members || []).length === 2 ? ch.members.find((x) => x !== me.id) : null;
  const blocked = other && S.blocks.includes(other);
  const del = async () => { if (confirmDelete(ch.name) && await act(() => api('DELETE', '/channels/' + ch.id))) { onChanged(); go('chat'); } };

  return html`<div class="conv">
    <div class="conv-head">
      <a class="icon-btn chat-back" href="#/chat" aria-label=${t('common.back')}><${Icon} name="left" /></a>
      <div class="grow"><h1>${ch.kind === 'dm' ? '' : '# '}${chName(ch, me)}</h1>${ch.topic && html`<div class="muted small">${linkify(ch.topic)}</div>`}
        ${ch.description && html`<details class="small about"><summary class="muted">${t('chat.about')}</summary><div class="pre">${linkify(ch.description)}</div></details>`}</div>
      <button onClick=${() => setShowPins(!showPins)} aria-expanded=${showPins}><${Icon} name="pin" /> ${t('chat.pins')} (${pins.length})</button>
      ${other && html`<button onClick=${() => setBlocked(other, !blocked)} aria-pressed=${blocked}><${Icon} name="block" /> ${blocked ? t('block.undo') : t('block.do')}</button>`}
      ${ch.kind === 'channel' && ch.level >= 3 && html`<button onClick=${() => setEditing(true)}><${Icon} name="edit" /> ${t('chat.edit_channel')}</button>
        <${ShareButton} type="channel" id=${ch.id} onDone=${onChanged} />
        <button class="icon-btn danger" onClick=${del} aria-label=${t('common.delete')} title=${t('common.delete')}><${Icon} name="trash" /></button>`}
    </div>
    ${showPins && html`<div class="pins" role="region" aria-label=${t('chat.pins')}>
      ${pins.length === 0 ? html`<span class="muted">${t('chat.no_pins')}</span>` : pins.map((p) => html`<div class="pin" key=${p.id}>
        <${Icon} name="pin" /><div class="grow"><b>${userName(p.user_id)}</b> <span class="muted small">${fmtAgo(p.created_at)}</span>
          <div class="pre">${rich(p.body)}</div>
          ${p.files.length > 0 && html`<div class="files">${p.files.map((f) => html`<${FileView} file=${f} key=${f.id} />`)}</div>`}</div></div>`)}
    </div>`}
    <div class="msgs" ref=${box} onScroll=${onScroll} role="log" aria-live="polite" aria-label=${t('chat.messages')}>
      ${msgs === null ? html`<div class="loading">${t('common.loading')}</div>` : html`
        ${more && msgs.length > 0 && html`<button class="link" onClick=${loadOlder}>${t('chat.older')}</button>`}
        ${msgs.length === 0 && html`<${Empty} icon="chat" text=${t('chat.empty')} />`}
        ${roots.map((m) => html`<div class="thread-root" key=${m.id}>
          <${Message} m=${m} ch=${ch} me=${me} reload=${load} reply=${!!m.parent_id} onReply=${ch.level >= 2 && !m.deleted ? () => setReplyTo(m.parent_id || m.id) : null} />
          ${(kids[m.id] || replyTo === m.id) && html`<div class="thread" role="group" aria-label=${t('chat.comments')}>
            ${(kids[m.id] || []).map((k) => html`<${Message} key=${k.id} m=${k} ch=${ch} me=${me} reload=${load} reply onReply=${ch.level >= 2 ? () => setReplyTo(m.id) : null} />`)}
            ${replyTo === m.id && html`<${CommentBox} onSend=${(body) => comment(m.id, body)} onCancel=${() => setReplyTo(null)} />`}</div>`}
        </div>`)}`}
    </div>
    ${editing && html`<${EditChannel} ch=${ch} onClose=${(ok) => { setEditing(false); if (ok) onChanged(); }} />`}
    ${blocked && html`<div class="notice small" role="status">${t('block.notice', { name: userName(other) })}</div>`}
    ${ch.level >= 2 ? html`<form class="composer" onSubmit=${send}>
      ${files.length > 0 && html`<div class="files">${files.map((f) => html`<${FileView} key=${f.id} file=${f} onRemove=${() => setFiles(files.filter((x) => x.id !== f.id))} />`)}</div>`}
      <div class="toolbar" role="toolbar" aria-label=${t('chat.tools')}>
        <input type="file" multiple hidden ref=${fileInput} onChange=${attach} />
        <button type="button" class="icon-btn" onClick=${() => fileInput.current.click()} aria-label=${t('chat.attach')} title=${t('chat.attach')} disabled=${busy}><${Icon} name="clip" /></button>
        <${CameraButtons} disabled=${busy} onPhoto=${addPhoto} onText=${(x) => setText((old) => (old ? old + '\n' + x : x))} />
        <${LinkButton} onPick=${(x) => insertAtCursor(input.current, x + ' ', setText)} />
        <span class="sep"></span>
        <${FormatBar} target=${input} setValue=${setText} />
      </div>
      ${sugg && candidates.length > 0 && html`<div class="mention-list" role="listbox" aria-label=${t('mention.suggest')}>
        ${candidates.map((u, i) => html`<button type="button" role="option" key=${u.id} aria-selected=${i === Math.min(sugg.i, candidates.length - 1)}
          onMouseDown=${(e) => { e.preventDefault(); mention(u); }}><${Avatar} user=${u} size="sm" /> ${u.display_name} <span class="muted small">@${u.username}</span></button>`)}</div>`}
      <div class="line">
        <textarea rows="1" ref=${input} value=${text} onInput=${typed} onKeyDown=${keydown} onBlur=${() => setSugg(null)} placeholder=${t('chat.placeholder')} aria-label=${t('chat.placeholder')} maxlength="10000"></textarea>
        <button class="primary" disabled=${busy} aria-label=${t('chat.send')}><${Icon} name="send" /></button>
      </div></form>` : html`<div class="composer muted"><${Icon} name="lock" /> ${t('chat.readonly')}</div>`}
  </div>`;
}

export function Chat({ route }) {
  const s = useStore();
  const st = useLoad(() => api('GET', '/channels'));
  const [modal, setModal] = useState(null);
  const [sorting, setSorting] = useState(false);
  useLive(['channel'], st.reload);
  if (!st.data) return html`<${Loading} state=${st} />`;
  const channels = st.data.filter((c) => c.kind === 'channel');
  const dms = st.data.filter((c) => c.kind === 'dm').sort((a, b) => b.last_at - a.last_at);
  const current = st.data.find((c) => c.id === route[1]);
  const item = (c) => html`<a href=${'#/chat/' + c.id} key=${c.id} aria-current=${current && current.id === c.id ? 'true' : null} class=${c.unread ? 'unread' : ''}>
    ${c.kind === 'dm' ? html`<${Icon} name="people" />` : html`<span aria-hidden="true">#</span>`}
    <span class="grow">${chName(c, s.me)}</span>${c.level < 2 && html`<${Icon} name="lock" label=${t('chat.readonly')} />`}
    ${c.unread > 0 && html`<span class="count" aria-label=${t('chat.unread', { n: c.unread })}>${c.unread}</span>`}</a>`;
  // Each person arranges the channels in their own order.
  const move = async (i, d) => {
    const ids = channels.map((c) => c.id);
    [ids[i], ids[i + d]] = [ids[i + d], ids[i]];
    if (await act(() => api('PUT', '/me/channel-order', { ids }))) st.reload();
  };
  const sortRow = (c, i) => html`<div class="sort-row" key=${c.id}><span aria-hidden="true">#</span><span class="grow">${c.name}</span>
    <button class="icon-btn" disabled=${i === 0} onClick=${() => move(i, -1)} aria-label=${t('common.move_up') + ': ' + c.name}><${Icon} name="up" /></button>
    <button class="icon-btn" disabled=${i === channels.length - 1} onClick=${() => move(i, 1)} aria-label=${t('common.move_down') + ': ' + c.name}><${Icon} name="down" /></button></div>`;
  return html`<div class=${'chat' + (current ? ' has-conv' : '')}>
    <nav class="chan-list" aria-label=${t('nav.chat')}>
      <h2><span class="grow">${t('chat.channels')}</span>
        ${channels.length > 1 && html`<button class="icon-btn" aria-pressed=${sorting} onClick=${() => setSorting(!sorting)}
          aria-label=${t('chat.sort')} title=${t('chat.sort')}><${Icon} name=${sorting ? 'check' : 'sort'} /></button>`}${can('channels.create') && html`<button class="icon-btn" onClick=${() => setModal('channel')}
        aria-label=${t('chat.new_channel')} title=${t('chat.new_channel')}><${Icon} name="plus" /></button>`}</h2>
      ${sorting ? channels.map(sortRow) : channels.map(item)}
      ${sorting && html`<p class="muted small sort-hint">${t('chat.sort_hint')}</p>`}
      <h2><span class="grow">${t('chat.dms')}</span><button class="icon-btn" onClick=${() => setModal('dm')} aria-label=${t('chat.new_dm')} title=${t('chat.new_dm')}><${Icon} name="plus" /></button></h2>
      ${dms.map(item)}
      ${dms.length === 0 && html`<p class="muted small" style=${{ padding: '0 1rem' }}>${t('chat.no_dms')}</p>`}
    </nav>
    ${current ? html`<${Conversation} ch=${current} me=${s.me} onChanged=${st.reload} focus=${route[2]} key=${current.id} />`
      : html`<div class="conv"><${Empty} icon="chat" text=${t('chat.pick')} /></div>`}
    ${modal === 'channel' && html`<${NewChannel} onClose=${() => { setModal(null); st.reload(); }} />`}
    ${modal === 'dm' && html`<${NewDM} onClose=${() => { setModal(null); st.reload(); }} />`}
  </div>`;
}
