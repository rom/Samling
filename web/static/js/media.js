// Pictures, the camera, text recognition, text formatting, emoji and colour
// marks: the building blocks shared by chat, lists, boards and the rest.
import { html, useState, useEffect, useRef, S, t, api, act, uploadFile, download, useLoad, linkify, fmtSize, Icon, Modal, toast,
  linkKinds, refToken, refsIn } from './lib.js';

const idOK = (id) => /^[A-Za-z0-9_-]{1,80}$/.test(id || '');
export const imageUrl = (id) => (idOK(id) ? '/api/files/' + id + '/image' : '');
export const isImage = (f) => /^image\/(jpeg|png|gif|webp)$/.test(f.mime || '');

// ---------- avatars ----------

export const avatarPresets = ['🦊', '🐻', '🐼', '🦉', '🐧', '🦋', '🐢', '🐬', '🦔', '🐝', '🌻', '🌲', '🍄', '⚽', '🏀', '🎾', '⛵', '🚲', '🎸', '🎨', '📚', '🧭', '⭐', '☕'];

// Avatar shows a person's photo, chosen picture or initial. It is
// decoration: the name is always written next to it.
export function Avatar({ id, user, size }) {
  const u = user || S.userMap[id];
  const av = (u && u.avatar) || '';
  const cls = 'avatar' + (size ? ' ' + size : '');
  if (av.startsWith('file:') && idOK(av.slice(5))) return html`<img class=${cls} src=${imageUrl(av.slice(5))} alt="" loading="lazy" />`;
  const n = av.startsWith('preset:') ? +av.slice(7) : 0;
  if (n >= 1 && n <= avatarPresets.length) return html`<span class=${cls + ' preset p' + (n % 8)} aria-hidden="true">${avatarPresets[n - 1]}</span>`;
  return html`<span class=${cls} aria-hidden="true">${((u && u.display_name) || '–').slice(0, 1).toUpperCase()}</span>`;
}

// ---------- formatted text ----------

// Messages are plain text with three marks: **bold**, *italic* and
// __underlined__. The marks are turned into elements here; the text itself is
// always inserted as text, never as HTML.
const marks = /\*\*\*([^\s*](?:[^*\n]*[^\s*])?)\*\*\*|\*\*([^\s*](?:[^*\n]*[^\s*])?)\*\*|__([^\s_](?:[^_\n]*[^\s_])?)__|\*([^\s*](?:[^*\n]*[^\s*])?)\*/g;
// A mention is "@username". It is shown with the person's name when the
// username exists; otherwise the text is left as it is.
const mentionRe = /(^|[^A-Za-z0-9._@/-])@([A-Za-z0-9][A-Za-z0-9._-]{1,31})/g;
let byUsername = null, byUsernameFor = null;
function findUser(name) {
  if (byUsernameFor !== S.users) { byUsernameFor = S.users; byUsername = Object.fromEntries(S.users.map((u) => [u.username, u])); }
  for (let n = name.toLowerCase(); n.length >= 2; n = n.slice(0, -1)) {
    if (byUsername[n]) return [byUsername[n], n.length];
    if (!/[._-]$/.test(n)) break; // only trailing punctuation may be dropped ("@anna.")
  }
  return null;
}
function mentions(text) {
  const out = [];
  let last = 0;
  for (const m of text.matchAll(mentionRe)) {
    const hit = findUser(m[2]);
    if (!hit) continue;
    const at = m.index + m[1].length;
    if (at > last) out.push(text.slice(last, at));
    out.push(html`<span class=${'mention' + (S.me && hit[0].id === S.me.id ? ' me' : '')} title=${'@' + hit[0].username}>@${hit[0].display_name}</span>`);
    last = at + 1 + hit[1];
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}
function format(text, depth = 0) {
  if (depth > 3) return [text];
  const out = [];
  let last = 0;
  const plain = (s) => { if (depth === 0 || s) out.push(...mentions(s)); };
  for (const m of text.matchAll(marks)) {
    if (m.index > last) plain(text.slice(last, m.index));
    if (m[1] !== undefined) out.push(html`<strong><em>${format(m[1], depth + 1)}</em></strong>`);
    else if (m[2] !== undefined) out.push(html`<strong>${format(m[2], depth + 1)}</strong>`);
    else if (m[3] !== undefined) out.push(html`<u>${format(m[3], depth + 1)}</u>`);
    else out.push(html`<em>${format(m[4], depth + 1)}</em>`);
    last = m.index + m[0].length;
  }
  if (last < text.length) plain(text.slice(last));
  return out;
}
export const rich = (text) => linkify(text || '').flatMap((part) => (typeof part === 'string' ? format(part) : [part]));

// wrapSelection puts a mark around the selected text of a text box (or
// removes it when it is already there).
export function wrapSelection(el, mark, setValue) {
  const { selectionStart: a, selectionEnd: b, value } = el;
  const n = mark.length;
  let next, from, to;
  if (value.slice(a - n, a) === mark && value.slice(b, b + n) === mark) {
    next = value.slice(0, a - n) + value.slice(a, b) + value.slice(b + n);
    from = a - n; to = b - n;
  } else {
    next = value.slice(0, a) + mark + value.slice(a, b) + mark + value.slice(b);
    from = a + n; to = b + n;
  }
  setValue(next);
  requestAnimationFrame(() => { el.focus(); el.setSelectionRange(from, to); });
}
export function insertAtCursor(el, text, setValue) {
  const { selectionStart: a, selectionEnd: b, value } = el;
  setValue(value.slice(0, a) + text + value.slice(b));
  requestAnimationFrame(() => { el.focus(); el.setSelectionRange(a + text.length, a + text.length); });
}
const formats = [['b', '**', 'fmt.bold'], ['i', '*', 'fmt.italic'], ['u', '__', 'fmt.underline']];
// formatKey handles Ctrl/⌘ + B, I and U in a text box.
export function formatKey(e, setValue) {
  if (!(e.ctrlKey || e.metaKey) || e.altKey) return false;
  const f = formats.find((x) => x[0] === e.key.toLowerCase());
  if (!f) return false;
  e.preventDefault();
  wrapSelection(e.target, f[1], setValue);
  return true;
}

export const emojis = ['😀', '😂', '🙂', '😉', '😍', '😎', '🤔', '😮', '😢', '😭', '😡', '😴', '👍', '👎', '👏', '🙏', '💪', '👋', '🤝', '👀',
  '❤️', '🔥', '🎉', '⭐', '✅', '❌', '❓', '❗', '☕', '🍰', '🎂', '🌞', '🌧️', '❄️', '⚽', '🏆', '📅', '📌', '📎', '💡'];
export const reactionEmojis = ['👍', '👎', '❤️', '😀', '😂', '😮', '😢', '😡', '🎉', '🙏', '👏', '🔥', '✅', '❓', '👀', '💪'];

// EmojiButton opens a small palette and reports the chosen emoji.
export function EmojiButton({ onPick, set = emojis, label, icon = 'smile' }) {
  const [open, setOpen] = useState(false);
  const ref = useRef();
  useEffect(() => {
    if (!open) return undefined;
    const away = (e) => { if (!ref.current.contains(e.target)) setOpen(false); };
    const key = (e) => { if (e.key === 'Escape') { e.stopPropagation(); setOpen(false); } };
    document.addEventListener('mousedown', away);
    document.addEventListener('keydown', key, true);
    return () => { document.removeEventListener('mousedown', away); document.removeEventListener('keydown', key, true); };
  }, [open]);
  return html`<span class="popup-wrap" ref=${ref}>
    <button type="button" class="icon-btn" aria-label=${label} title=${label} aria-expanded=${open} onClick=${() => setOpen(!open)}><${Icon} name=${icon} /></button>
    ${open && html`<div class="emoji-pop" role="group" aria-label=${label}>
      ${set.map((e) => html`<button type="button" key=${e} onClick=${() => { setOpen(false); onPick(e); }}>${e}</button>`)}</div>`}</span>`;
}

// FormatBar: bold, italic, underline and emoji for the text box in "target".
export function FormatBar({ target, setValue }) {
  return html`${formats.map(([k, mark, label]) => html`<button type="button" key=${k} class=${'icon-btn fmt fmt-' + k} aria-label=${t(label)} title=${t(label)}
      onClick=${() => wrapSelection(target.current, mark, setValue)}>${t(label).slice(0, 1).toUpperCase()}</button>`)}
    <${EmojiButton} label=${t('fmt.emoji')} onPick=${(e) => insertAtCursor(target.current, e, setValue)} />`;
}

// ---------- camera and text recognition ----------

// toJpeg scales a picture down to a sensible size: photos straight from a
// phone camera are needlessly large.
async function toJpeg(source, max = 2000) {
  const w = source.videoWidth || source.width, h = source.videoHeight || source.height;
  if (!w || !h) throw new Error('no picture');
  const k = Math.min(1, max / Math.max(w, h));
  const canvas = document.createElement('canvas');
  canvas.width = Math.round(w * k);
  canvas.height = Math.round(h * k);
  canvas.getContext('2d').drawImage(source, 0, 0, canvas.width, canvas.height);
  return new Promise((resolve, reject) => canvas.toBlob((b) => (b ? resolve(b) : reject(new Error('no picture'))), 'image/jpeg', 0.86));
}
const stamp = () => new Date().toISOString().slice(0, 19).replace(/[-:]/g, '').replace('T', '-');
// preparePhoto turns a chosen picture file into an upload of reasonable size.
export async function preparePhoto(file, max = 2000) {
  const bmp = await createImageBitmap(file);
  try { return new File([await toJpeg(bmp, max)], (file.name || 'photo').replace(/\.[^.]*$/, '') + '.jpg', { type: 'image/jpeg' }); } finally { bmp.close(); }
}

// CaptureDialog takes a picture with the camera (or lets the person choose
// one). In "ocr" mode the text in the picture is read and can be corrected
// before it is used.
function CaptureDialog({ mode, onClose, onPhoto, onText, square }) {
  const [stream, setStream] = useState(null);
  const [stage, setStage] = useState('camera'); // camera | reading | review
  const [progress, setProgress] = useState(0);
  const [text, setText] = useState('');
  const [keep, setKeep] = useState(false);
  const [photo, setPhoto] = useState(null);
  const video = useRef();
  const picker = useRef();
  useEffect(() => {
    let live = null, gone = false;
    if (navigator.mediaDevices && navigator.mediaDevices.getUserMedia) {
      navigator.mediaDevices.getUserMedia({ video: { facingMode: square ? 'user' : 'environment', width: { ideal: 1920 } }, audio: false }).then((s) => {
        if (gone) { s.getTracks().forEach((x) => x.stop()); return; }
        live = s;
        setStream(s);
      }).catch(() => { /* no camera, or not allowed: choosing a picture still works */ });
    }
    return () => { gone = true; if (live) live.getTracks().forEach((x) => x.stop()); };
  }, []);
  useEffect(() => { if (stream && video.current) { video.current.srcObject = stream; video.current.play().catch(() => {}); } }, [stream, stage]);

  const use = async (blob) => {
    const file = new File([blob], (mode === 'ocr' ? 'scan-' : 'photo-') + stamp() + '.jpg', { type: 'image/jpeg' });
    if (mode !== 'ocr') { onPhoto(file); onClose(); return; }
    setPhoto(file);
    setStage('reading');
    try {
      const { recognize } = await import('./ocr.js');
      setText(await recognize(blob, setProgress));
      setStage('review');
    } catch (e) {
      console.error(e);
      toast(t('err.ocr_failed'), 'error');
      setStage('camera');
    }
  };
  const snap = () => toJpeg(video.current, square ? 1024 : 2000).then(use).catch(() => toast(t('err.not_image'), 'error'));
  const pick = async (e) => {
    const f = e.target.files[0];
    e.target.value = '';
    if (!f) return;
    try { use(await preparePhoto(f, square ? 1024 : 2000)); } catch { toast(t('err.not_image'), 'error'); }
  };
  const done = () => { onText(text.trim(), keep ? photo : null); onClose(); };

  return html`<${Modal} title=${mode === 'ocr' ? t('cam.scan') : t('cam.photo')} onClose=${onClose} wide>
    ${stage === 'camera' && html`
      ${mode === 'ocr' && html`<p class="muted">${t('cam.scan_hint')}</p>`}
      ${stream ? html`<video class="camera" ref=${video} autoplay playsinline muted aria-label=${t('cam.preview')}></video>`
        : html`<p class="muted"><${Icon} name="camera" /> ${t('cam.none')}</p>`}
      <input type="file" accept="image/*" capture="environment" hidden ref=${picker} onChange=${pick} />
      <div class="buttons"><button type="button" onClick=${onClose}>${t('common.cancel')}</button>
        <button type="button" class=${stream ? '' : 'primary'} onClick=${() => picker.current.click()}><${Icon} name="image" /> ${t('cam.choose')}</button>
        ${stream && html`<button type="button" class="primary" onClick=${snap}><${Icon} name="camera" /> ${t('cam.take')}</button>`}</div>`}
    ${stage === 'reading' && html`<div class="loading" role="status">${t('cam.reading')} ${Math.round(progress * 100)} %
      <progress max="1" value=${progress}></progress></div>`}
    ${stage === 'review' && html`
      <p class="muted">${text ? t('cam.review') : t('cam.no_text')}</p>
      <textarea rows="10" value=${text} onInput=${(e) => setText(e.target.value)} aria-label=${t('cam.text')}></textarea>
      ${onPhoto && html`<label class="check"><input type="checkbox" checked=${keep} onChange=${(e) => setKeep(e.target.checked)} /> ${t('cam.keep_photo')}</label>`}
      <div class="buttons"><button type="button" onClick=${() => setStage('camera')}>${t('cam.again')}</button>
        <button type="button" class="primary" disabled=${!text.trim()} onClick=${done}>${t('cam.use_text')}</button></div>`}
  <//>`;
}

// CameraButtons offers "take a picture" (when onPhoto is given) and "scan
// text" (when onText is given). onText receives the text and, if the person
// chose to keep it, the picture.
export function CameraButtons({ onPhoto, onText, labels, disabled, square }) {
  const [mode, setMode] = useState(null);
  const btn = (m, icon, key) => html`<button type="button" class=${labels ? '' : 'icon-btn'} disabled=${disabled} aria-label=${labels ? null : t(key)} title=${t(key)}
    onClick=${() => setMode(m)}><${Icon} name=${icon} />${labels ? ' ' + t(key) : ''}</button>`;
  return html`${onPhoto && btn('photo', 'camera', 'cam.photo')}${onText && btn('ocr', 'scan', 'cam.scan')}
    ${mode && html`<${CaptureDialog} mode=${mode} square=${square} onPhoto=${onPhoto} onText=${(text, photo) => { if (photo && onPhoto) onPhoto(photo); onText(text); }} onClose=${() => setMode(null)} />`}`;
}

// ScanInto is the "scan text" button for a form field: the recognised text
// is added to what the field already holds.
export function ScanInto({ value, onChange }) {
  return html`<${TextTools} value=${value} onChange=${onChange} />`;
}

// ---------- files and pictures ----------

export function Lightbox({ file, onClose }) {
  return html`<${Modal} title=${file.name} onClose=${onClose} wide>
    <img class="lightbox" src=${imageUrl(file.id)} alt=${file.name} />
    <div class="buttons"><button type="button" onClick=${() => act(() => download('GET', '/files/' + file.id, undefined, file.name))}><${Icon} name="download" /> ${t('common.download')}</button>
      <button type="button" class="primary" onClick=${onClose}>${t('common.close')}</button></div><//>`;
}

// FileView shows an attached file: pictures as a thumbnail that opens
// larger, everything else as a download button.
export function FileView({ file, onRemove }) {
  const [open, setOpen] = useState(false);
  const remove = onRemove && html`<button type="button" class="icon-btn remove" aria-label=${t('common.remove') + ': ' + file.name} title=${t('common.remove')} onClick=${onRemove}><${Icon} name="x" /></button>`;
  if (isImage(file) && idOK(file.id)) {
    return html`<span class="thumb"><button type="button" title=${file.name} onClick=${() => setOpen(true)}><img src=${imageUrl(file.id)} alt=${file.name} loading="lazy" /></button>${remove}
      ${open && html`<${Lightbox} file=${file} onClose=${() => setOpen(false)} />`}</span>`;
  }
  return html`<span class="file-wrap"><button type="button" class="file-chip" title=${t('common.download')} onClick=${() => act(() => download('GET', '/files/' + file.id, undefined, file.name))}>
    <${Icon} name="clip" /><span>${file.name}</span><span class="muted nowrap">${fmtSize(file.size)}</span></button>${remove}</span>`;
}

// Attachments lists the pictures and files attached to a list, event, card,
// meeting, poll or decision, and lets people with the right to change it add
// more: from a file, or straight from the camera.
export function Attachments({ type, id, canEdit, onText }) {
  const st = useLoad(() => api('GET', `/attachments/${type}/${id}`), [type, id]);
  const [busy, setBusy] = useState(false);
  const input = useRef();
  const add = async (files) => {
    setBusy(true);
    for (const file of files) {
      const up = await act(() => uploadFile(file));
      if (up) await act(() => api('POST', `/attachments/${type}/${id}`, { file_id: up.id }));
    }
    setBusy(false);
    st.reload();
  };
  const remove = async (f) => { if (await act(() => api('DELETE', `/attachments/${type}/${id}/${f.id}`))) st.reload(); };
  const list = st.data || [];
  if (!canEdit && list.length === 0) return null;
  return html`<div class="attachments" role="group" aria-label=${t('att.title')}>
    <h3>${t('att.title')}${list.length > 0 ? ' (' + list.length + ')' : ''}</h3>
    ${list.length > 0 && html`<div class="files">${list.map((f) => html`<${FileView} key=${f.id} file=${f} onRemove=${canEdit ? () => remove(f) : null} />`)}</div>`}
    ${canEdit && html`<div class="row no-print">
      <input type="file" multiple hidden ref=${input} onChange=${(e) => { const fs = [...e.target.files]; e.target.value = ''; add(fs); }} />
      <button type="button" disabled=${busy} onClick=${() => input.current.click()}><${Icon} name="clip" /> ${t('att.add')}</button>
      <${CameraButtons} labels disabled=${busy} onPhoto=${(f) => add([f])} onText=${onText} /></div>`}
  </div>`;
}

// ---------- internal links ----------

// LinkButton lets the person pick something in Samling – a poll, a board, a
// document … – and reports the link text to insert.
const linkSources = [['poll', '/polls'], ['board', '/boards'], ['document', '/documents'], ['list', '/lists'], ['meeting', '/meetings'], ['decision', '/decisions'],
  ['event', '/calendar?from=' + Math.floor(Date.now() / 1000 - 30 * 86400)], ['channel', '/channels']];
function LinkPicker({ onPick, onClose }) {
  const [items, setItems] = useState(null);
  const [q, setQ] = useState('');
  const [kind, setKind] = useState('');
  useEffect(() => {
    Promise.all(linkSources.map(([type, path]) => api('GET', path).then((rows) => rows.filter((r) => type !== 'channel' || r.kind === 'channel')
      .map((r) => ({ type, id: r.id, title: type === 'decision' ? r.number + ' ' + r.title : r.title || r.name }))).catch(() => [])))
      .then((all) => setItems(all.flat()));
  }, []);
  const shown = (items || []).filter((x) => (!kind || x.type === kind) && x.title.toLowerCase().includes(q.toLowerCase())).slice(0, 60);
  return html`<${Modal} title=${t('link.insert')} onClose=${onClose}>
    <p class="muted small">${t('link.hint')}</p>
    <div class="row">
      <input class="grow" type="search" value=${q} onInput=${(e) => setQ(e.target.value)} placeholder=${t('common.filter')} aria-label=${t('common.filter')} />
      <select value=${kind} onChange=${(e) => setKind(e.target.value)} aria-label=${t('link.kind')}><option value="">${t('link.all')}</option>
        ${linkSources.map(([type]) => html`<option value=${type}>${t('link.kind_' + type)}</option>`)}</select></div>
    ${items === null ? html`<div class="loading">${t('common.loading')}</div>` : html`<div class="rows link-list">
      ${shown.map((x) => html`<button type="button" key=${x.type + x.id} onClick=${() => { onPick(refToken(x.type, x.id)); onClose(); }}>
        <${Icon} name=${linkKinds[x.type][1]} /><span class="grow">${x.title}</span><span class="muted small">${t('link.kind_' + x.type)}</span></button>`)}
      ${shown.length === 0 && html`<div class="muted">${t('common.nothing')}</div>`}</div>`}
  <//>`;
}
export function LinkButton({ onPick, labels, disabled }) {
  const [open, setOpen] = useState(false);
  return html`<button type="button" class=${labels ? '' : 'icon-btn'} disabled=${disabled} aria-label=${labels ? null : t('link.insert')} title=${t('link.insert')}
      onClick=${() => setOpen(true)}><${Icon} name="link" />${labels ? ' ' + t('link.insert') : ''}</button>
    ${open && html`<${LinkPicker} onPick=${onPick} onClose=${() => setOpen(false)} />`}`;
}

// Refs shows the internal links found in a text that is edited as plain
// text (meeting notes, descriptions), so that they can be followed.
export function Refs({ text }) {
  const refs = refsIn(text);
  if (!refs.length) return null;
  return html`<div class="refs" role="group" aria-label=${t('link.in_text')}><span class="muted small">${t('link.in_text')}:</span> ${refs}</div>`;
}

// TextTools sits under a text field: scan text into it, insert a link, and
// the links the text already holds.
export function TextTools({ value, onChange, scan = true, disabled }) {
  const add = (x, sep) => onChange(value ? value.replace(/\s+$/, '') + sep + x : x);
  return html`<div class="field-tools">
    ${!disabled && scan && html`<${CameraButtons} labels onText=${(x) => add(x, '\n')} />`}
    ${!disabled && html`<${LinkButton} labels onPick=${(x) => add(x, ' ')} />`}
    <${Refs} text=${value} /></div>`;
}

// ---------- colour marks ----------

export const colors = ['', 'red', 'orange', 'yellow', 'green', 'teal', 'blue', 'purple', 'pink', 'gray'];
export function ColorPicker({ value, onChange, disabled }) {
  return html`<div class="colors" role="radiogroup" aria-label=${t('color.label')}>
    ${colors.map((c) => html`<button type="button" key=${c} role="radio" aria-checked=${(value || '') === c} disabled=${disabled} class=${'dot ' + (c ? 'mark-' + c : 'mark-none')}
      aria-label=${t('color.' + (c || 'none'))} title=${t('color.' + (c || 'none'))} onClick=${() => onChange(c)}></button>`)}</div>`;
}
export const markClass = (c) => (colors.includes(c) && c ? ' marked mark-' + c : '');
