// Produces the screenshots of the user manual (docs/manual/img/*.png).
//
// It fills a fresh installation with example data for a fictional club
// through the API and then photographs each part of the application.
// Run it with:  make manual-screenshots
// (tests/e2e/run.sh builds and starts a scratch server and sets the
// environment: E2E_URL, E2E_ADMIN_PASSWORD, E2E_OUT.)
const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');

const BASE = process.env.E2E_URL || 'http://localhost:8090';
const TEMP = process.env.E2E_ADMIN_PASSWORD;
const OUT = process.env.MANUAL_OUT || path.join(__dirname, '..', 'docs', 'manual', 'img');
const PW = 'correct horse battery staple';
fs.mkdirSync(OUT, { recursive: true });

// ---- API helper (a signed-in session) ----
async function session(username, password) {
  const r = await fetch(BASE + '/api/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username, password }) });
  if (!r.ok) throw new Error(`login ${username}: ${r.status}`);
  const cookie = r.headers.get('set-cookie').split(';')[0];
  const { csrf } = await r.json();
  const call = async (method, p, body, raw) => {
    const headers = { Cookie: cookie, 'X-CSRF-Token': csrf };
    if (body !== undefined && !raw) headers['Content-Type'] = 'application/json';
    const res = await fetch(BASE + '/api' + p, { method, headers, body: raw ? body : body === undefined ? undefined : JSON.stringify(body) });
    const text = await res.text();
    if (!res.ok) throw new Error(`${method} ${p}: ${res.status} ${text}`);
    try { return JSON.parse(text); } catch { return text; }
  };
  call.upload = async (name, content) => {
    const fd = new FormData();
    fd.append('file', new Blob([content]), name);
    return call('POST', '/files', fd, true);
  };
  call.cookie = cookie;
  return call;
}

const day = 86400;
const now = Math.floor(Date.now() / 1000);
const at = (days, hour, min = 0) => { const d = new Date(); d.setDate(d.getDate() + days); d.setHours(hour, min, 0, 0); return Math.floor(d.getTime() / 1000); };
const everyone = (level = 1) => [{ type: 'all', id: '', level }];

(async () => {
  // ---------- example data ----------
  let admin = await session('admin', TEMP);
  await admin('POST', '/me/password', { current: TEMP, new: PW });
  admin = await session('admin', PW);
  await admin('PATCH', '/me', { display_name: 'Anna Lindgren', email: 'anna@bjorken.example', phone: '070-123 45 67', locale: 'en', settings: { tour_done: true } });

  const people = [['bertil', 'Bertil Berg', ['leader']], ['cecilia', 'Cecilia Ceder', ['member']], ['david', 'David Dahl', ['member']], ['elsa', 'Elsa Ek', ['leader']], ['farid', 'Farid Hassan', ['member']]];
  const ids = {}, as = {};
  for (const [u, name, roles] of people) {
    ids[u] = (await admin('POST', '/admin/users', { username: u, display_name: name, email: `${u}@bjorken.example`, password: 'temporary-password-1', roles })).id;
    const s = await session(u, 'temporary-password-1');
    await s('POST', '/me/password', { current: 'temporary-password-1', new: PW });
    as[u] = await session(u, PW);
    await as[u]('PATCH', '/me', { locale: 'en', settings: u === 'david' ? {} : { tour_done: true } });
  }
  // Profile pictures and the optional profile fields
  await admin('PATCH', '/me', { avatar: 'preset:11', profile: { address: 'Björkvägen 3, Björkö', interests: 'Orienteering, baking', specialities: 'First aid, minibus licence' },
    profile_shared: { interests: true, specialities: true } });
  for (const [u, n, profile] of [['bertil', 2, { specialities: 'Treasurer, bookkeeping' }], ['cecilia', 6, { interests: 'Football, photography' }], ['elsa', 20, {}], ['farid', 14, { interests: 'Coaching, cooking for many' }]]) {
    await as[u]('PATCH', '/me', { avatar: 'preset:' + n, profile, profile_shared: { interests: true, specialities: true } });
  }
  const board = await admin('POST', '/groups', { name: 'The board', description: 'Elected board members', scope: 'global', members: [ids.bertil, ids.elsa] });
  await admin('POST', '/groups', { name: 'Youth section', description: 'Leaders and helpers for the youth teams', scope: 'global', members: [ids.cecilia, ids.david, ids.farid] });
  await admin('POST', '/groups', { name: 'Summer camp helpers', description: '', scope: 'local', members: [ids.cecilia, ids.farid] });

  // Chat
  const general = (await admin('GET', '/channels'))[0].id;
  const news = (await admin('POST', '/channels', { name: 'announcements', topic: 'News from the board – everyone can read, leaders post', shares: [{ type: 'all', id: '', level: 1 }, { type: 'role', id: 'leader', level: 2 }] })).id;
  await admin('POST', '/channels', { name: 'summer-camp', topic: 'Planning the camp 14–18 July', shares: everyone(2) });
  const pinned = (await admin('POST', `/channels/${general}/messages`, { body: '**Welcome to Björkens IF!** House rules and the membership form are in Documents. Ask here if you wonder about anything.' })).id;
  await admin('POST', `/messages/${pinned}/pin`);
  const question = (await as.cecilia('POST', `/channels/${general}/messages`, { body: 'Hi all! Does anyone have the key to the equipment shed?' })).id;
  await as.bertil('POST', `/channels/${general}/messages`, { body: 'I have it. I will be at the clubhouse on *Saturday from 10*.', parent_id: question });
  await as.cecilia('POST', `/channels/${general}/messages`, { body: 'Perfect, see you then!', parent_id: question });
  ids.general = general;
  const budget = await as.elsa.upload('Budget 2026.xlsx', 'example spreadsheet');
  await as.elsa('POST', `/channels/${general}/messages`, { body: 'Here is the budget proposal for the annual meeting. We meet online: https://meet.jit.si/bjorkens-if', file_ids: [budget.id] });
  const proposal = (await admin('GET', `/channels/${general}/messages`)).pop().id;
  for (const u of ['bertil', 'cecilia', 'farid']) await as[u]('POST', `/messages/${proposal}/reactions`, { emoji: '👍' });
  await admin('POST', `/messages/${proposal}/reactions`, { emoji: '👍' });
  await as.cecilia('POST', `/messages/${proposal}/reactions`, { emoji: '🎉' });
  await as.farid('POST', `/channels/${general}/messages`, { body: 'Thanks Elsa! Looks good to me.' });
  await as.bertil('POST', `/channels/${news}/messages`, { body: 'The spring clean-up day is on Saturday. Coffee and buns for everyone who helps!' });
  const dm = (await as.bertil('POST', '/dm', { user_ids: [ids.cecilia] })).id;
  await as.bertil('POST', `/channels/${dm}/messages`, { body: 'Could you take the U12 training on Thursday?' });

  // Documents
  for (const [name, folder, desc, file] of [
    ['Statutes', 'Association', 'Adopted at the annual meeting 2024', 'Statutes.pdf'],
    ['Membership form', 'Association', 'Print, fill in and hand to a leader', 'Membership form.docx'],
    ['Annual report 2025', 'Board/2025', 'Report and accounts', 'Annual report 2025.pdf'],
    ['Training schedule spring', 'Youth section', '', 'Schedule.xlsx'],
  ]) {
    const f = await as.elsa.upload(file, 'example ' + name);
    const d = await as.elsa('POST', '/documents', { name, folder, description: desc, file_id: f.id, shares: everyone(1) });
    if (name === 'Statutes') {
      const f2 = await as.elsa.upload('Statutes rev.pdf', 'example statutes, revised');
      await as.elsa('POST', `/documents/${d.id}/versions`, { file_id: f2.id, comment: 'Revised §3 after the annual meeting' });
      ids.statutes = d.id;
    }
  }
  const priv = await admin.upload('Draft speech.docx', 'draft');
  await admin('POST', '/documents', { name: 'Draft: speech for the anniversary', folder: '', description: 'Only for me so far', file_id: priv.id });

  // Lists
  const list = (await admin('POST', '/lists', { title: 'Before the summer camp', kind: 'priority', shares: [{ type: 'group', id: board.id, level: 2 }] })).id;
  ids.list = list;
  for (const [text, priority, due] of [['Book the bus', 3, at(6, 12)], ['Order food for 40 people', 2, at(12, 12)], ['Check the tents', 2, 0], ['Send the packing list to parents', 1, at(20, 12)], ['Buy first-aid supplies', 0, 0]]) {
    const owner = { 'Book the bus': ids.bertil, 'Order food for 40 people': ids.farid, 'Check the tents': ids.cecilia }[text] || '';
    const item = await admin('POST', `/lists/${list}/items`, { text, priority, due_at: due ? Math.floor(due / day) * day : 0, assignee_id: owner });
    if (text === 'Check the tents') await admin('PATCH', `/lists/${list}/items/${item.id}`, { done: true });
  }
  await admin('POST', '/lists', { title: 'My shopping list', kind: 'todo' });

  // Polls
  const poll = (await as.elsa('POST', '/polls', { title: 'Which day suits best for the autumn party?', description: 'We need to book the hall this week.', kind: 'single',
    options: ['Friday 9 October', 'Saturday 10 October', 'Saturday 17 October'], allow_change: true, results_visible: 'after_vote', shares: everyone(1) })).id;
  ids.poll = poll;
  const opts = (await admin('GET', `/polls/${poll}`)).options;
  const votes = { admin: 1, bertil: 1, cecilia: 2, david: 1, farid: 0 };
  await admin('POST', `/polls/${poll}/vote`, { choices: { [opts[votes.admin].id]: 1 } });
  for (const u of ['bertil', 'cecilia', 'farid']) await as[u]('POST', `/polls/${poll}/vote`, { choices: { [opts[votes[u]].id]: 1 } });
  const fee = (await as.elsa('POST', '/polls', { title: 'Raise the membership fee to 350 kr?', kind: 'yesno', anonymous: true, shares: everyone(1) })).id;
  const feeOpts = (await admin('GET', `/polls/${fee}`)).options;
  for (const [u, i] of [['bertil', 0], ['cecilia', 0], ['farid', 1]]) await as[u]('POST', `/polls/${fee}/vote`, { choices: { [feeOpts[i].id]: 1 } });
  await admin('POST', `/polls/${fee}/vote`, { choices: { [feeOpts[0].id]: 1 } });
  await as.elsa('POST', `/polls/${fee}/vote`, { choices: { [feeOpts[0].id]: 1 } });
  await as.elsa('POST', `/polls/${fee}/close`);

  // Calendar
  await admin('POST', '/calendar', { title: 'Spring clean-up day', location: 'The clubhouse', starts_at: at(3, 10), ends_at: at(3, 14), shares: everyone(1) });
  await admin('POST', '/calendar', { title: 'U12 training', location: 'Björkvallen', starts_at: at(1, 17, 30), ends_at: at(1, 19), shares: everyone(1) });
  await admin('POST', '/calendar', { title: 'U12 training', location: 'Björkvallen', starts_at: at(8, 17, 30), ends_at: at(8, 19), shares: everyone(1) });
  await admin('POST', '/calendar', { title: 'Dentist', starts_at: at(2, 8), ends_at: at(2, 9) });
  const camp = Math.floor(at(16, 12) / day) * day;
  await admin('POST', '/calendar', { title: 'Summer camp', location: 'Lake Björksjön', all_day: true, starts_at: camp, ends_at: camp + 2 * day, shares: everyone(1) });

  // Meeting with agenda, minutes and a decision
  const meeting = (await as.elsa('POST', '/meetings', { title: 'Board meeting', location: 'The clubhouse', url: 'https://teams.microsoft.com/l/meetup-join/example',
    starts_at: at(5, 18), ends_at: at(5, 20), shares: [{ type: 'group', id: board.id, level: 2 }, { type: 'user', id: (await admin('GET', '/me')).user.id, level: 2 }],
    agenda: ['Opening of the meeting', 'Election of secretary and adjuster', 'Approval of the agenda', 'Finances', 'Summer camp', 'Other business', 'Closing of the meeting'] })).id;
  ids.meeting = meeting;
  let m = await as.elsa('GET', `/meetings/${meeting}`);
  const agenda = m.agenda.map((x) => ({ ...x }));
  agenda[3] = { ...agenda[3], presenter: 'Bertil Berg', duration_min: 15, notes: 'The treasurer presented the accounts. The balance is 48 200 kr, in line with the budget.' };
  agenda[4] = { ...agenda[4], presenter: 'Anna Lindgren', duration_min: 20, description: 'Budget and number of leaders', notes: '34 children have signed up. Six leaders are needed; five are confirmed.' };
  await as.elsa('PUT', `/meetings/${meeting}/agenda`, agenda);
  await as.elsa('PATCH', `/meetings/${meeting}`, { title: m.title, description: '', location: m.location, url: m.url, starts_at: m.starts_at, ends_at: m.ends_at, status: 'in_progress',
    attendees: 'Elsa Ek\nBertil Berg\nAnna Lindgren', chair: 'Elsa Ek', secretary: 'Anna Lindgren', notes: '' });
  m = await as.elsa('GET', `/meetings/${meeting}`);
  const dec = await as.elsa('POST', '/decisions', { title: 'Budget for the summer camp', body: 'The board decided to set aside 20 000 kr for the summer camp 2026, including bus and food.',
    meeting_id: meeting, agenda_item_id: m.agenda[4].id, shares: everyone(1) });
  ids.decision = dec.id;
  await as.elsa('POST', '/decisions', { title: 'New training times for the youth teams', body: 'Tuesdays and Thursdays 17:30–19:00 from 1 September.', shares: everyone(1) });
  await as.elsa('PATCH', `/decisions/${dec.id}`, { title: 'Budget for the summer camp', body: 'The board decided to set aside 24 000 kr for the summer camp 2026, including bus and food.', status: 'decided', comment: 'Raised after the bus quote came in' });

  // Board
  const kb = (await admin('POST', '/boards', { title: 'Clubhouse renovation', description: 'What needs doing before the season starts', columns: ['To do', 'In progress', 'Done'], color: 'green', shares: everyone(2) })).id;
  ids.board = kb;
  const cols = (await admin('GET', `/boards/${kb}`)).columns;
  for (const [c, title, who, due] of [[0, 'Paint the changing rooms', 'david', at(14, 12)], [0, 'Replace the broken window', '', 0], [0, 'Order new goal nets', 'farid', at(9, 12)],
    [1, 'Fix the roof gutter', 'bertil', at(4, 12)], [1, 'Apply for the municipal grant', 'elsa', at(10, 12)], [2, 'Service the lawn mower', 'bertil', 0], [2, 'Clean out the storage room', 'cecilia', 0]]) {
    const color = { 'Replace the broken window': 'red', 'Fix the roof gutter': 'orange', 'Apply for the municipal grant': 'blue', 'Order new goal nets': 'yellow' }[title] || '';
    await admin('POST', `/boards/${kb}/cards`, { column_id: cols[c].id, title, color, assignee_id: who ? ids[who] : '', due_at: due ? Math.floor(due / day) * day : 0 });
  }

  // ---------- screenshots ----------
  const browser = await chromium.launch();
  const context = async (opts = {}, tourOff = true) => {
    const ctx = await browser.newContext({ viewport: { width: 1200, height: 760 }, deviceScaleFactor: 2, locale: 'en-GB', ...opts });
    if (tourOff) await ctx.addInitScript(() => localStorage.setItem('samling.tour', 'off'));
    await ctx.addInitScript(() => { try { localStorage.setItem('samling.locale', 'en'); } catch { /* ignore */ } });
    return ctx;
  };
  const signIn = async (page, user) => {
    await page.goto(BASE);
    await page.getByLabel('Username').fill(user);
    await page.getByLabel('Password').fill(PW);
    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    await page.locator('.shell').waitFor();
  };
  const ctx = await context();
  // A "photo" of a note on the notice board, for the picture and text-scanning examples.
  const note = await ctx.newPage();
  await note.setViewportSize({ width: 640, height: 420 });
  await note.setContent(`<body style="margin:0;background:#c9a36b"><div style="margin:28px;padding:26px 34px;background:#fffdf2;box-shadow:0 3px 10px #0006;font:30px/1.55 Arial,sans-serif;color:#111;height:310px">
    <b>Packing list</b><br>Sleeping bag<br>Rain clothes<br>Torch<br>Swimwear and towel</div></body>`);
  const notePicture = await note.screenshot({ type: 'png' });
  await note.close();
  const pic = await admin.upload('Notice board.png', notePicture);
  await admin('POST', `/attachments/list/${ids.list}`, { file_id: pic.id });
  const pic2 = await as.cecilia.upload('Notice board.png', notePicture);
  await as.cecilia('POST', `/channels/${ids.general}/messages`, { body: 'The packing list is up on the notice board 📌', file_ids: [pic2.id] });
  const page = await ctx.newPage();
  page.setDefaultTimeout(8000);
  const shot = async (name, opts = {}) => {
    await page.waitForTimeout(450); // let toasts and transitions settle
    await page.locator('.toast').first().waitFor({ state: 'detached', timeout: 7000 }).catch(() => {});
    await page.screenshot({ path: path.join(OUT, name + '.png'), ...opts });
    console.log('saved', name);
  };
  const open = async (hash) => { await page.goto(BASE + '/#/' + hash); await page.waitForTimeout(500); };

  await page.goto(BASE);
  await page.getByLabel('Username').fill('anna');
  await page.waitForTimeout(400);
  // The sign-in page has a gradient background: JPEG keeps the file small.
  await page.screenshot({ path: path.join(OUT, '01-sign-in.jpg'), type: 'jpeg', quality: 88 });
  await signIn(page, 'admin');
  await open('home'); await shot('03-home');
  await open('chat/' + general);
  await page.getByRole('button', { name: /Pinned/ }).click();
  await shot('04-chat');
  await open('documents'); await shot('05-documents');
  await open('documents/' + ids.statutes); await shot('06-document');
  await page.getByRole('button', { name: 'Share' }).click();
  await page.getByRole('dialog').waitFor();
  await shot('07-sharing');
  await page.keyboard.press('Escape');
  await open('lists/' + ids.list); await shot('08-list');
  await open('polls/' + ids.poll); await shot('09-poll');
  await open('calendar'); await shot('10-calendar');
  await open('meetings/' + ids.meeting);
  await page.getByRole('tab', { name: 'Agenda' }).click(); await shot('11-meeting-agenda');
  await page.getByRole('tab', { name: 'Minutes' }).click();
  await page.locator('.agenda-item').nth(3).scrollIntoViewIfNeeded(); await shot('12-meeting-minutes');
  await open('decisions'); await shot('13-decisions');
  await open('decisions/' + ids.decision); await shot('14-decision');
  await open('boards/' + ids.board); await shot('15-board');
  await open('search');
  await page.getByRole('searchbox').fill('camp*');
  await page.getByRole('button', { name: 'Advanced search' }).click();
  await page.locator('form[role=search] select').first().selectOption('wildcard');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await page.locator('.hit').first().waitFor();
  await shot('16-search');
  await open('people'); await shot('17-people');
  await page.getByRole('tab', { name: /Groups/ }).click(); await shot('18-groups');
  await open('history'); await shot('19-history');
  await open('lists/' + ids.list);
  await page.getByRole('button', { name: 'Scan text with the camera' }).click();
  await page.getByRole('dialog').locator('input[type=file]').setInputFiles({ name: 'note.png', mimeType: 'image/png', buffer: notePicture });
  await page.getByLabel('Text from the picture').waitFor({ timeout: 120000 });
  await shot('34-scan-text');
  await page.keyboard.press('Escape');
  await open('settings'); await shot('20-settings-appearance');
  await page.getByText('Profile picture', { exact: true }).scrollIntoViewIfNeeded(); await shot('33-profile');
  await page.getByRole('heading', { name: 'Security' }).scrollIntoViewIfNeeded(); await shot('21-settings-security');
  await open('help'); await shot('22-help');
  await open('help/sharing'); await shot('23-help-topic');
  await open('admin'); await shot('24-admin-accounts');
  await page.locator('tr', { hasText: '@cecilia' }).getByRole('button').click();
  await page.getByRole('dialog').waitFor(); await shot('25-admin-account');
  await page.keyboard.press('Escape');
  await page.getByRole('tab', { name: 'Roles' }).click(); await shot('26-admin-roles');
  await page.getByRole('tab', { name: 'Log' }).click(); await page.locator('tbody tr').first().waitFor(); await shot('27-admin-log');
  await page.getByRole('tab', { name: 'Backup & export' }).click(); await shot('28-admin-backup');
  // Themes side by side is easiest to show with the dark theme on a busy page.
  await open('settings');
  await page.getByRole('button', { name: 'Dark' }).click();
  await open('boards/' + ids.board); await shot('29-dark-theme');
  await ctx.close();

  // The welcome tour, as a new member sees it.
  const tctx = await context({}, false);
  const tpage = await tctx.newPage();
  await signIn(tpage, 'david');
  const dialog = tpage.getByRole('dialog');
  await dialog.waitFor();
  await tpage.waitForTimeout(600);
  await tpage.screenshot({ path: path.join(OUT, '02-welcome-tour.png') });
  await dialog.getByRole('button', { name: 'Next' }).click();
  await dialog.getByRole('button', { name: 'Next' }).click();
  await tpage.waitForTimeout(600);
  await tpage.screenshot({ path: path.join(OUT, '02b-welcome-tour-step.png') });
  console.log('saved 02-welcome-tour');
  await tctx.close();

  // On a phone.
  const pctx = await context({ viewport: { width: 390, height: 800 }, isMobile: true, hasTouch: true });
  const pp = await pctx.newPage();
  await signIn(pp, 'cecilia');
  await pp.goto(BASE + '/#/chat/' + general); await pp.waitForTimeout(600);
  await pp.screenshot({ path: path.join(OUT, '30-phone-chat.png') });
  await pp.getByRole('button', { name: 'Menu' }).click(); await pp.waitForTimeout(400);
  await pp.screenshot({ path: path.join(OUT, '31-phone-menu.png') });
  await pp.goto(BASE + '/#/calendar'); await pp.waitForTimeout(600);
  await pp.screenshot({ path: path.join(OUT, '32-phone-calendar.png') });
  console.log('saved phone views');
  await pctx.close();

  await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
