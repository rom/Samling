import { html, useState, useRef, t, api, act, download, uploadFile, useLoad, go, userName, fmtDateTime, fmtSize, Icon, Modal, Field, useForm,
  ShareEditor, ShareButton, SharedBadge, Page, Empty, Loading, confirmDelete } from '../lib.js';

function Upload({ onClose, folders, folder }) {
  const [f, , bind] = useForm({ name: '', folder: folder || '', description: '' });
  const [shares, setShares] = useState([]);
  const [file, setFile] = useState(null);
  const [busy, setBusy] = useState(false);
  const submit = async (e) => {
    e.preventDefault();
    setBusy(true);
    const up = await act(() => uploadFile(file));
    const r = up && await act(() => api('POST', '/documents', { ...f, file_id: up.id, shares }), t('docs.uploaded'));
    setBusy(false);
    if (r) onClose(true);
  };
  return html`<${Modal} title=${t('docs.upload')} onClose=${() => onClose(false)}><form onSubmit=${submit}>
    <${Field} label=${t('docs.file')}><input type="file" required onChange=${(e) => setFile(e.target.files[0])} /><//>
    <${Field} label=${t('common.name')} hint=${t('docs.name_hint')}><input ...${bind('name')} maxlength="200" /><//>
    <${Field} label=${t('docs.folder')} hint=${t('docs.folder_hint')}><input ...${bind('folder')} list="folders" /><//>
    <datalist id="folders">${folders.map((x) => html`<option value=${x} />`)}</datalist>
    <${Field} label=${t('common.description')}><textarea ...${bind('description')}></textarea><//>
    <${Field} group label=${t('share.who')}><${ShareEditor} value=${shares} onChange=${setShares} /><//>
    <div class="buttons"><button type="button" onClick=${() => onClose(false)}>${t('common.cancel')}</button>
      <button class="primary" disabled=${busy || !file}>${busy ? t('common.working') : t('docs.upload')}</button></div>
  </form><//>`;
}

function Detail({ id }) {
  const st = useLoad(() => api('GET', '/documents/' + id), [id]);
  const [edit, setEdit] = useState(null);
  const [comment, setComment] = useState('');
  const fileInput = useRef();
  if (!st.data) return html`<${Page} title=${t('nav.documents')} back="#/documents"><${Loading} state=${st} /><//>`;
  const d = st.data;
  const dl = (v) => act(() => download('GET', `/documents/${id}/download` + (v ? '?version=' + v : ''), undefined, d.name));
  const newVersion = async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    const up = await act(() => uploadFile(file));
    if (up && await act(() => api('POST', `/documents/${id}/versions`, { file_id: up.id, comment }), t('docs.version_added'))) { setComment(''); st.reload(); }
    e.target.value = '';
  };
  const save = async (e) => {
    e.preventDefault();
    if (await act(() => api('PATCH', '/documents/' + id, edit), t('common.saved'))) { setEdit(null); st.reload(); }
  };
  const copy = async () => {
    const r = await act(() => api('POST', `/documents/${id}/copy`, { name: t('docs.copy_of', { name: d.name }), folder: d.folder }), t('docs.copied'));
    if (r) go('documents', r.id);
  };
  const del = async () => { if (confirmDelete(d.name) && await act(() => api('DELETE', '/documents/' + id))) go('documents'); };
  return html`<${Page} title=${d.name} back="#/documents" actions=${html`
      <button class="primary" onClick=${() => dl()}><${Icon} name="download" /> ${t('common.download')}</button>
      <button onClick=${copy}><${Icon} name="copy" /> ${t('docs.copy')}</button>
      ${d.level >= 2 && html`<button onClick=${() => setEdit({ name: d.name, folder: d.folder, description: d.description })}><${Icon} name="edit" /> ${t('common.edit')}</button>`}
      ${d.level >= 3 && html`<${ShareButton} type="document" id=${id} onDone=${st.reload} />
        <button class="danger" onClick=${del}><${Icon} name="trash" /> ${t('common.delete')}</button>`}`}>
    <div class="stack">
      <div class="card">
        <div class="row"><${SharedBadge} shared=${d.shared} /><span class="muted">${d.folder ? '/' + d.folder : t('docs.root')}</span>
          <span class="muted">· ${t('docs.owner')}: ${userName(d.owner_id)}</span></div>
        ${d.description && html`<p class="pre">${d.description}</p>`}
      </div>
      <div class="card"><h2>${t('docs.versions')}</h2>
        <div class="table-wrap"><table><thead><tr><th>${t('docs.version')}</th><th>${t('docs.file')}</th><th>${t('common.when')}</th><th>${t('common.who')}</th><th>${t('docs.comment')}</th><th></th></tr></thead>
        <tbody>${d.versions.map((v) => html`<tr key=${v.version}>
          <td>v${v.version}${v.version === d.version && html` <span class="badge info">${t('docs.current')}</span>`}</td>
          <td>${v.file.name} <span class="muted small nowrap">${fmtSize(v.file.size)}</span></td>
          <td class="nowrap">${fmtDateTime(v.created_at)}</td><td>${userName(v.user_id)}</td><td>${v.comment}</td>
          <td><button class="icon-btn" onClick=${() => dl(v.version)} aria-label=${t('common.download') + ' v' + v.version}><${Icon} name="download" /></button></td></tr>`)}
        </tbody></table></div>
        ${d.level >= 2 && html`<div class="add-line">
          <input type="text" placeholder=${t('docs.comment_placeholder')} aria-label=${t('docs.comment')} value=${comment} onInput=${(e) => setComment(e.target.value)} />
          <input type="file" hidden ref=${fileInput} onChange=${newVersion} />
          <button onClick=${() => fileInput.current.click()}><${Icon} name="upload" /> ${t('docs.new_version')}</button></div>`}
      </div>
    </div>
    ${edit && html`<${Modal} title=${t('common.edit')} onClose=${() => setEdit(null)}><form onSubmit=${save}>
      <${Field} label=${t('common.name')}><input value=${edit.name} onInput=${(e) => setEdit({ ...edit, name: e.target.value })} required /><//>
      <${Field} label=${t('docs.folder')}><input value=${edit.folder} onInput=${(e) => setEdit({ ...edit, folder: e.target.value })} /><//>
      <${Field} label=${t('common.description')}><textarea value=${edit.description} onInput=${(e) => setEdit({ ...edit, description: e.target.value })}></textarea><//>
      <div class="buttons"><button type="button" onClick=${() => setEdit(null)}>${t('common.cancel')}</button><button class="primary">${t('common.save')}</button></div>
    </form><//>`}
  <//>`;
}

export function Documents({ route }) {
  const st = useLoad(() => api('GET', '/documents'));
  const [upload, setUpload] = useState(false);
  const [folder, setFolder] = useState(null); // null = all
  const [q, setQ] = useState('');
  if (route[1]) return html`<${Detail} id=${route[1]} key=${route[1]} />`;
  if (!st.data) return html`<${Page} title=${t('nav.documents')}><${Loading} state=${st} /><//>`;
  const folders = [...new Set(st.data.map((d) => d.folder))].sort();
  const docs = st.data.filter((d) => (folder === null || d.folder === folder || d.folder.startsWith(folder + '/')) && d.name.toLowerCase().includes(q.toLowerCase()));
  return html`<${Page} title=${t('nav.documents')} actions=${html`<button class="primary" onClick=${() => setUpload(true)}><${Icon} name="upload" /> ${t('docs.upload')}</button>`}>
    <div class="row" style=${{ marginBottom: '1rem' }}>
      <select aria-label=${t('docs.folder')} value=${folder === null ? '*' : folder} onChange=${(e) => setFolder(e.target.value === '*' ? null : e.target.value)}>
        <option value="*">${t('docs.all_folders')}</option>
        ${folders.map((x) => html`<option value=${x}>${x ? '/' + x : t('docs.root')}</option>`)}</select>
      <input class="grow" type="search" placeholder=${t('common.filter')} aria-label=${t('common.filter')} value=${q} onInput=${(e) => setQ(e.target.value)} />
    </div>
    ${docs.length === 0 ? html`<${Empty} icon="doc" text=${t('docs.empty')} />` : html`<div class="rows">
      ${docs.map((d) => html`<a href=${'#/documents/' + d.id} key=${d.id}><${Icon} name="doc" />
        <span class="grow"><b>${d.name}</b><br /><span class="muted small">${d.folder ? '/' + d.folder + ' · ' : ''}v${d.version} · ${fmtSize(d.size)} · ${fmtDateTime(d.updated_at)}</span></span>
        <${SharedBadge} shared=${d.shared} /></a>`)}</div>`}
    ${upload && html`<${Upload} folders=${folders.filter(Boolean)} folder=${folder} onClose=${(ok) => { setUpload(false); if (ok) st.reload(); }} />`}
  <//>`;
}
