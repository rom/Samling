// Browser smoke test for Samling: drives the real client through every main
// feature as an administrator on a desktop viewport and as a member on a
// phone viewport. Run it with tests/e2e/run.sh, which builds the server and
// starts it on a scratch data directory.
//
// Environment: E2E_URL (server), E2E_ADMIN_PASSWORD (temporary password of
// the fresh "admin" account), E2E_OUT (directory for screenshots/downloads).
const { chromium } = require('playwright');
const fs = require('fs');
const BASE = process.env.E2E_URL || 'http://localhost:8090'; // a host name: passkeys do not work on IP addresses
const temp = process.env.E2E_ADMIN_PASSWORD;
const OUT = process.env.E2E_OUT || __dirname + '/out';
fs.mkdirSync(OUT, { recursive: true });
const PW = 'correct horse battery staple';
const errors = [];
// The welcome tour opens at a person's first sign-in; tests skip it.
const dismissTour = async (p) => {
  const skip = p.getByRole('dialog').getByRole('button', { name: /^(Skip|Hoppa över)/ });
  try { await skip.waitFor({ timeout: 1500 }); await skip.click(); } catch { /* no tour shown */ }
};
const step = async (name, fn) => { try { await fn(); console.log('ok   ' + name); } catch (e) { console.log('FAIL ' + name + ': ' + e.message.split('\n').slice(0,6).join(' | ')); errors.push(name); } };

(async () => {
  // A fake camera, and no permission question: the camera steps run unattended.
  const browser = await chromium.launch({ args: ['--use-fake-device-for-media-stream', '--use-fake-ui-for-media-stream'] });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: 'sv-SE', timezoneId: 'Europe/Stockholm', acceptDownloads: true });
  // The welcome tour is exercised with the second user; elsewhere it is switched off.
  await ctx.addInitScript(() => localStorage.setItem('samling.tour', 'off'));
  const page = await ctx.newPage();
  page.on('console', (m) => { if (m.type() === 'error' && !m.text().includes('401')) { console.log('  console.error:', m.text().slice(0, 300)); errors.push('console'); } });
  page.on('pageerror', (e) => { console.log('  pageerror:', e.message.slice(0, 300)); errors.push('pageerror'); });
  page.on('dialog', (d) => d.accept(d.type() === 'prompt' ? 'Granskas' : undefined));
  const shot = (n) => page.screenshot({ path: `${OUT}/shot-${n}.png` });
  const toastGone = () => page.waitForTimeout(250);
  page.setDefaultTimeout(6000);

  await step('login + forced password change', async () => {
    await page.goto(BASE);
    await page.getByLabel('Användarnamn').fill('admin');
    await page.getByLabel('Lösenord').fill(temp);
    await page.getByRole('button', { name: 'Logga in', exact: true }).click();
    await page.getByRole('heading', { name: 'Välj ett nytt lösenord' }).waitFor();
    await shot('01-pending');
    await page.getByLabel('Nuvarande lösenord').fill(temp);
    await page.getByLabel('Nytt lösenord', { exact: true }).fill(PW);
    await page.getByLabel('Upprepa nytt lösenord').fill(PW);
    await page.getByRole('button', { name: 'Byt lösenord' }).click();
    await page.getByRole('navigation', { name: 'Huvudmeny' }).waitFor();
    await shot('02-home');
  });

  await step('admin: create accounts', async () => {
    await page.goto(BASE + '/#/admin');
    for (const [u, n] of [['bertil', 'Bertil Berg'], ['cecilia', 'Cecilia Ceder']]) {
      await page.getByRole('button', { name: 'Nytt konto' }).click();
      await page.getByLabel('Användarnamn').fill(u);
      await page.getByLabel('Namn', { exact: true }).fill(n);
      await page.getByLabel('E-post').fill(u + '@example.org');
      await page.getByRole('textbox', { name: 'Tillfälligt lösenord' }).fill('temporary-password-1');
      await page.getByRole('button', { name: 'Skapa', exact: true }).click();
      await page.getByText('@' + u).first().waitFor();
    }
    await shot('03-admin');
    await page.getByRole('tab', { name: 'Roller' }).click();
    await page.getByText('Ledare').first().waitFor();
    await page.getByRole('tab', { name: 'Logg' }).click();
    await page.getByText('Skapade konto').first().waitFor();
    await shot('04-log');
  });

  await step('chat: post, pin, attach', async () => {
    await page.goto(BASE + '/#/chat');
    await page.getByRole('link', { name: /general/ }).click();
    await page.getByLabel('Skriv ett meddelande').fill('Välkomna! Mötet hålls på https://teams.microsoft.com/l/meetup-join/abc');
    await page.getByRole('button', { name: 'Skicka' }).click();
    await page.getByText('Anslut via Microsoft Teams').waitFor();
    await page.locator('input[type=file]').setInputFiles({ name: 'Budget 2026.xlsx', mimeType: 'application/vnd.ms-excel', buffer: Buffer.from('fake excel content') });
    await page.getByText('Budget 2026.xlsx').waitFor();
    await page.getByLabel('Skriv ett meddelande').fill('Här är budgeten');
    await page.getByRole('button', { name: 'Skicka' }).click();
    await page.locator('.msg .file-chip').waitFor();
    await page.locator('.msg').first().hover();
    await page.locator('.msg').first().getByRole('button', { name: 'Fäst meddelandet' }).click();
    await page.getByRole('button', { name: /Fästa \(1\)/ }).click();
    await page.locator('.pins .pin').waitFor();
    await shot('05-chat');
    const dl = page.waitForEvent('download');
    await page.locator('.msg .file-chip').click();
    const d = await dl;
    if (d.suggestedFilename() !== 'Budget 2026.xlsx') throw new Error('download name ' + d.suggestedFilename());
  });

  await step('chat: new channel + DM', async () => {
    await page.getByRole('button', { name: 'Ny kanal' }).click();
    await page.getByLabel('Namn').fill('styrelsen');
    await page.getByRole('button', { name: 'Skapa', exact: true }).click();
    await page.getByRole('heading', { name: '# styrelsen' }).waitFor();
    await page.getByRole('button', { name: 'Nytt direktmeddelande' }).click();
    await page.getByRole('dialog').getByText('Bertil Berg').click();
    await page.getByRole('button', { name: 'Starta konversation' }).click();
    await page.getByRole('heading', { name: 'Bertil Berg' }).waitFor();
    await page.getByLabel('Skriv ett meddelande').fill('Hej Bertil!');
    await page.keyboard.press('Enter');
    await page.locator('.msg').waitFor();
  });

  await step('documents: upload, version, copy', async () => {
    await page.goto(BASE + '/#/documents');
    await page.getByRole('button', { name: 'Ladda upp dokument' }).click();
    await page.getByLabel('Fil', { exact: true }).setInputFiles({ name: 'Stadgar.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF v1') });
    await page.getByRole('dialog').getByLabel('Mapp').fill('Styrelsen/2026');
    await page.getByRole('dialog').getByLabel('Lägg till person, grupp eller roll').selectOption({ label: 'Alla' });
    await page.getByRole('dialog').getByRole('button', { name: 'Ladda upp dokument' }).click();
    await page.getByRole('link', { name: /Stadgar\.pdf/ }).click();
    await page.getByRole('heading', { name: 'Versioner' }).waitFor();
    await page.getByLabel('Kommentar').fill('Reviderad');
    await page.locator('input[type=file]').setInputFiles({ name: 'Stadgar-v2.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF v2') });
    await page.getByText('v2').first().waitFor();
    await shot('06-document');
    await page.getByRole('button', { name: 'Gör en kopia' }).click();
    await page.getByRole('heading', { name: 'Kopia av Stadgar.pdf' }).waitFor();
  });

  await step('lists', async () => {
    await page.goto(BASE + '/#/lists');
    await page.getByRole('button', { name: 'Ny lista' }).click();
    await page.getByLabel('Rubrik').fill('Inför sommarlägret');
    await page.getByLabel('Typ').selectOption('priority');
    await page.getByRole('button', { name: 'Skapa', exact: true }).click();
    await page.getByRole('heading', { name: 'Inför sommarlägret' }).waitFor();
    for (const x of ['Boka buss', 'Köpa mat']) {
      await page.getByLabel('Lägg till en rad').fill(x);
      await page.getByRole('button', { name: 'Lägg till', exact: true }).click();
      await page.getByText(x).waitFor();
    }
    await page.getByLabel('Klar: Boka buss').check();
    await page.locator('li.done').waitFor();
    await shot('07-list');
  });

  let pollUrl;
  await step('polls: create + vote + close', async () => {
    await page.goto(BASE + '/#/polls');
    await page.getByRole('button', { name: 'Ny omröstning' }).click();
    await page.getByLabel('Fråga').fill('Vilken färg på klubbtröjan?');
    await page.getByLabel('Alternativ 1').fill('Röd');
    await page.getByLabel('Alternativ 2').fill('Blå');
    await page.getByRole('button', { name: 'Skapa', exact: true }).click();
    await page.getByRole('heading', { name: 'Vilken färg på klubbtröjan?' }).waitFor();
    await page.getByLabel('Blå').check();
    await page.getByRole('button', { name: 'Rösta' }).click();
    await page.getByText('1 (100 %)').waitFor();
    await shot('08-poll');
    await page.getByRole('button', { name: 'Avsluta omröstningen' }).click();
    await page.getByText('Avslutad').first().waitFor();
    pollUrl = page.url();
    // ranked poll
    await page.goto(BASE + '/#/polls');
    await page.getByRole('button', { name: 'Ny omröstning' }).click();
    await page.getByLabel('Fråga').fill('Resmål?');
    await page.getByLabel('Röstningsmetod').selectOption('ranked');
    await page.getByLabel('Alternativ 1').fill('Fjäll');
    await page.getByLabel('Alternativ 2').fill('Hav');
    await page.getByRole('button', { name: 'Skapa', exact: true }).click();
    await page.getByRole('button', { name: 'Flytta ner: Fjäll' }).click();
    await page.getByRole('button', { name: 'Rösta' }).click();
    await page.getByText('1 poäng').waitFor();
  });

  await step('calendar: event + export', async () => {
    await page.goto(BASE + '/#/calendar');
    await page.getByRole('button', { name: 'Ny händelse' }).click();
    await page.getByLabel('Rubrik').fill('Sommarfest');
    await page.getByLabel('Plats').fill('Klubbstugan');
    await page.getByLabel('Länk').fill('https://zoom.us/j/123');
    await page.getByRole('button', { name: 'Spara' }).click();
    await page.locator('.cal-ev', { hasText: 'Sommarfest' }).first().click();
    await page.getByText('Anslut via Zoom').waitFor();
    await shot('09-calendar-event');
    const dl = page.waitForEvent('download');
    await page.getByRole('button', { name: '.ics' }).click();
    const d = await dl;
    const ics = fs.readFileSync(await d.path(), 'utf8');
    if (!ics.includes('SUMMARY:Sommarfest')) throw new Error('ics content');
    fs.writeFileSync(OUT + '/event.ics', ics.replace('Sommarfest', 'Importerad fest').replace(/UID:.*/, 'UID:other-1@x'));
    await page.getByRole('button', { name: 'Stäng' }).click();
    await page.locator('input[type=file]').setInputFiles(OUT + '/event.ics');
    await page.getByText('1 händelser importerade').waitFor();
    await page.locator('.cal-ev', { hasText: 'Importerad fest' }).first().waitFor();
    await shot('10-calendar');
  });

  await step('meetings: plan, agenda, protocol, decision, finalize', async () => {
    await page.goto(BASE + '/#/meetings');
    await page.getByRole('button', { name: 'Planera möte' }).click();
    await page.getByLabel('Rubrik').fill('Styrelsemöte');
    await page.getByRole('dialog').getByLabel('Lägg till person, grupp eller roll').selectOption({ label: 'Alla' });
    await page.getByRole('button', { name: 'Skapa', exact: true }).click();
    await page.getByRole('tab', { name: 'Dagordning' }).click();
    await page.locator('.agenda-item').nth(7).waitFor();
    await page.getByRole('button', { name: 'Lägg till punkt' }).click();
    await page.locator('.agenda-item').last().getByLabel('Rubrik').fill('Sommarläger');
    await page.getByRole('button', { name: 'Spara' }).click();
    await page.getByText('Sparat').waitFor();
    await shot('11-agenda');
    await page.getByRole('tab', { name: 'Protokoll' }).click();
    await page.getByLabel('Ordförande').fill('Anna Admin');
    await page.locator('.agenda-item').nth(4).getByLabel('Anteckningar').fill('Kassan är god: 12 345 kr.');
    await page.locator('.agenda-item').nth(4).getByRole('button', { name: 'För in beslut' }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'För in beslut' }).click();
    await page.getByText('Beslut infört').waitFor();
    await shot('12-protocol');
    await page.getByRole('tab', { name: 'Justera' }).click();
    const dl = page.waitForEvent('download');
    await page.getByRole('button', { name: /Exportera protokoll/ }).click();
    const md = fs.readFileSync(await (await dl).path(), 'utf8');
    if (!md.includes('**Beslut ') || !md.includes('12 345 kr')) throw new Error('protocol content: ' + md.slice(0, 200));
    await page.getByRole('button', { name: 'Justera protokollet' }).click();
    await page.getByText(/Justerat av Anna Admin/).waitFor();
  });

  await step('decisions: log, detail, revise, from poll, history', async () => {
    await page.goto(BASE + '/#/decisions');
    await page.getByRole('link', { name: /-001/ }).click();
    await page.getByRole('button', { name: 'Revidera' }).click();
    await page.getByLabel('Status').selectOption('revoked');
    await page.getByLabel('Skäl till ändringen').fill('Nya uppgifter');
    await page.getByRole('button', { name: 'Spara' }).click();
    await page.getByText('Nya uppgifter').waitFor();
    await shot('13-decision');
    await page.goto(pollUrl);
    await page.getByRole('link', { name: 'För in som beslut' }).click();
    await page.getByRole('dialog').getByLabel('Beslut', { exact: true }).waitFor();
    await page.waitForFunction(() => document.querySelector('[role=dialog] input').value.includes('klubbtröjan'));
    await page.getByRole('dialog').getByRole('button', { name: 'För in beslut' }).click();
    await page.getByRole('heading', { name: /-002 Vilken färg/ }).waitFor();
    await page.goto(BASE + '/#/decisions');
    await page.getByRole('tab', { name: 'Historik' }).click();
    await page.locator('.timeline > div').nth(2).waitFor();
    await shot('14-decision-history');
  });

  await step('boards: create, card, move', async () => {
    await page.goto(BASE + '/#/boards');
    await page.getByRole('button', { name: 'Ny tavla' }).click();
    await page.getByLabel('Rubrik').fill('Styrelsearbete');
    await page.getByRole('button', { name: 'Skapa', exact: true }).click();
    await page.locator('.kcol').nth(2).waitFor();
    await page.locator('.kcol').first().getByRole('button', { name: 'Lägg till kort' }).click();
    await page.getByLabel('Rubrik').fill('Ansök om bidrag');
    await page.getByLabel('Ansvarig').selectOption({ label: 'Bertil Berg' });
    await page.getByRole('button', { name: 'Spara' }).click();
    await page.locator('.kcol').first().locator('.kcard').waitFor();
    await page.getByRole('button', { name: 'Flytta höger: Ansök om bidrag' }).click();
    await page.locator('.kcol').nth(1).locator('.kcard').waitFor();
    // drag and drop to third column
    await page.locator('.kcard').dragTo(page.locator('.kcol').nth(2));
    await page.locator('.kcol').nth(2).locator('.kcard').waitFor();
    await page.getByRole('button', { name: 'Lägg till kolumn' }).click();
    await page.locator('.kcol').nth(3).waitFor();
    await shot('15-board');
  });

  await step('people + groups', async () => {
    await page.goto(BASE + '/#/people');
    await page.getByText('Cecilia Ceder', { exact: true }).waitFor();
    await page.getByRole('tab', { name: /Grupper/ }).click();
    await page.getByRole('button', { name: 'Ny gemensam grupp' }).click();
    await page.getByLabel('Namn').fill('Ungdomssektionen');
    await page.getByRole('dialog').getByText('Cecilia Ceder').click();
    await page.getByRole('button', { name: 'Spara' }).click();
    await page.getByRole('heading', { name: 'Ungdomssektionen' }).waitFor();
    await page.getByRole('button', { name: 'Ny privat grupp' }).click();
    await page.getByLabel('Namn').fill('Mina vänner');
    await page.getByRole('button', { name: 'Spara' }).click();
    await page.getByRole('heading', { name: 'Mina vänner' }).waitFor();
    await shot('16-groups');
  });

  await step('chat: formatting, emoji, reactions and comments', async () => {
    await page.goto(BASE + '/#/chat');
    await page.getByRole('link', { name: /general/ }).click();
    const box = page.getByLabel('Skriv ett meddelande');
    await box.fill('viktigt');
    await box.press('Control+a');
    await box.press('Control+b');
    if ((await box.inputValue()) !== '**viktigt**') throw new Error('Ctrl+B gave ' + (await box.inputValue()));
    await box.fill('**Städdag** på *lördag* – __ta med handskar__! ');
    await page.getByRole('toolbar').getByRole('button', { name: 'Infoga emoji' }).click();
    await page.locator('.emoji-pop').getByRole('button', { name: '🎉' }).click();
    if (!(await box.inputValue()).includes('🎉')) throw new Error('emoji not inserted');
    await page.getByRole('button', { name: 'Skicka', exact: true }).click();
    const msg = page.locator('.thread-root').last().locator('.msg').first();
    await msg.locator('strong', { hasText: 'Städdag' }).waitFor();
    await msg.locator('em', { hasText: 'lördag' }).waitFor();
    await msg.locator('u', { hasText: 'ta med handskar' }).waitFor();
    // A message that looks like markup stays text.
    await box.fill('<img src=x onerror=alert(1)> **<b>fet</b>**');
    await box.press('Enter');
    await page.locator('.msg strong', { hasText: '<b>fet</b>' }).waitFor();
    if (await page.locator('.msg .pre img').count()) throw new Error('markup was interpreted');
    // React, and take the reaction back.
    await msg.hover();
    await msg.getByRole('button', { name: 'Reagera' }).click();
    await msg.locator('.emoji-pop').getByRole('button', { name: '👍' }).click();
    await msg.locator('.reaction[aria-pressed=true]', { hasText: '1' }).waitFor();
    await msg.locator('.reaction').click();
    await msg.locator('.reaction').waitFor({ state: 'detached' });
    await msg.hover();
    await msg.getByRole('button', { name: 'Reagera' }).click();
    await msg.locator('.emoji-pop').getByRole('button', { name: '❤️' }).click();
    await msg.locator('.reaction').waitFor();
    // Comment on the message.
    await msg.hover();
    await msg.getByRole('button', { name: 'Kommentera' }).click();
    await page.getByLabel('Skriv en kommentar').fill('Jag kommer!');
    await page.getByLabel('Skriv en kommentar').press('Enter');
    await page.locator('.thread .msg', { hasText: 'Jag kommer!' }).waitFor();
    await shot('50-chat-rich');
  });

  // A picture of printed text, for the text recognition.
  const textPicture = async () => {
    const p = await ctx.newPage();
    await p.setViewportSize({ width: 900, height: 420 });
    await p.setContent('<body style="margin:0;background:#fff"><div style="font:54px/1.5 Arial,sans-serif;color:#000;padding:40px">Packa tältet<br>Ring Cecilia<br>Betala avgiften</div></body>');
    const buf = await p.screenshot({ type: 'png' });
    await p.close();
    return buf;
  };

  await step('camera: a photo in the chat', async () => {
    await page.getByRole('toolbar').getByRole('button', { name: 'Ta en bild' }).click();
    const dlg = page.getByRole('dialog', { name: 'Ta en bild' });
    await dlg.locator('video').waitFor();
    await page.waitForFunction(() => { const v = document.querySelector('video.camera'); return v && v.videoWidth > 0; });
    await dlg.getByRole('button', { name: 'Ta bild' }).click();
    await page.locator('.composer .thumb img').waitFor();
    await page.getByLabel('Skriv ett meddelande').fill('Så här ser det ut');
    await page.getByRole('button', { name: 'Skicka', exact: true }).click();
    const img = page.locator('.msg .thumb img').last();
    await img.waitFor();
    await page.waitForFunction(() => { const all = document.querySelectorAll('.msg .thumb img'); const i = all[all.length - 1]; return i && i.complete && i.naturalWidth > 0; });
    await img.click();
    await page.getByRole('dialog').locator('img.lightbox').waitFor();
    await shot('51-chat-photo');
    await page.keyboard.press('Escape');
  });

  await step('lists: task owner, attached picture, scanned text becomes items', async () => {
    await page.goto(BASE + '/#/lists');
    await page.getByRole('link', { name: /Inför sommarlägret/ }).click();
    await page.getByLabel('Ansvarig: Köpa mat').selectOption({ label: 'Bertil Berg' });
    await page.waitForFunction(() => [...document.querySelectorAll('.items li')].some((li) => li.textContent.includes('Köpa mat') && li.querySelector('.avatar')));
    const att = page.getByRole('group', { name: 'Bilder och filer' });
    await att.locator('input[type=file]').first().setInputFiles({ name: 'karta.png', mimeType: 'image/png', buffer: await textPicture() });
    await att.locator('.thumb img').waitFor();
    await att.getByRole('button', { name: 'Skanna text med kameran' }).click();
    const dlg = page.getByRole('dialog', { name: 'Skanna text med kameran' });
    await dlg.locator('input[type=file]').setInputFiles({ name: 'lapp.png', mimeType: 'image/png', buffer: await textPicture() });
    const text = dlg.getByLabel('Text från bilden');
    await text.waitFor({ timeout: 120000 });
    const got = await text.inputValue();
    if (!/Packa t.ltet/.test(got) || !/Ring Cecilia/.test(got) || !/Betala avgiften/.test(got)) throw new Error('recognised: ' + JSON.stringify(got));
    await shot('52-scan');
    await dlg.getByRole('button', { name: 'Använd texten' }).click();
    await page.locator('.items li', { hasText: 'Ring Cecilia' }).waitFor();
    await page.locator('.items li', { hasText: 'Betala avgiften' }).waitFor();
    await shot('53-list-owner');
  });

  await step('boards: colour marks; a card with a picture', async () => {
    await page.goto(BASE + '/#/boards');
    await page.getByRole('link', { name: /Styrelsearbete/ }).click();
    await page.locator('.board-bar').getByRole('radio', { name: 'Grön' }).click();
    await page.locator('.board-bar.mark-green').waitFor();
    await page.locator('.kcard').first().click();
    const dlg = page.getByRole('dialog', { name: 'Kort' });
    await dlg.getByRole('radio', { name: 'Röd' }).click();
    await dlg.getByRole('group', { name: 'Bilder och filer' }).locator('input[type=file]').first().setInputFiles({ name: 'skiss.png', mimeType: 'image/png', buffer: await textPicture() });
    await dlg.locator('.thumb img').waitFor();
    await dlg.getByRole('button', { name: 'Spara' }).click();
    await page.locator('.kcard.mark-red').waitFor();
    await shot('54-board-colours');
    await page.goto(BASE + '/#/boards');
    await page.locator('a.card.mark-green').waitFor();
  });

  await step('profile: optional fields, picture; blocking direct messages', async () => {
    await page.goto(BASE + '/#/settings');
    await page.getByLabel('Intressen', { exact: true }).fill('Orientering och bakning');
    await page.getByLabel('Adress', { exact: true }).fill('Björkvägen 3');
    await page.getByLabel('Visa för andra medlemmar: Intressen').check();
    await page.getByRole('radio', { name: 'Bild 3', exact: true }).click();
    await page.locator('form.card').getByRole('button', { name: 'Spara' }).click();
    await page.locator('.who .avatar.preset').waitFor();
    await shot('55-profile');
    await page.goto(BASE + '/#/people');
    await page.getByText('Orientering och bakning').waitFor();
    await page.getByRole('button', { name: 'Bertil Berg: Blockera direktmeddelanden' }).click();
    await page.getByText('Blockerad', { exact: true }).waitFor();
    await shot('56-people');
    await page.goto(BASE + '/#/settings');
    await page.locator('.card', { hasText: 'Blockerade personer' }).getByRole('button', { name: 'Sluta blockera' }).click();
    await page.getByText('Du har inte blockerat någon.').waitFor();
  });

  await step('search', async () => {
    await page.goto(BASE + '/#/search');
    await page.getByLabel(/Sök meddelanden/).fill('budget*');
    await page.getByRole('button', { name: 'Avancerad sökning' }).click();
    await page.getByLabel('Matchning').selectOption('wildcard');
    await page.getByRole('button', { name: 'Sök', exact: true }).click();
    await page.getByText(/träffar/).waitFor();
    await page.locator('.hit').first().waitFor();
    await shot('17-search');
    await page.getByLabel('Matchning').selectOption('regex');
    await page.getByLabel(/Sök meddelanden/).fill('(');
    await page.getByRole('button', { name: 'Sök', exact: true }).click();
    await page.getByText('Det reguljära uttrycket är inte giltigt.').waitFor();
    errors.splice(errors.indexOf('console'), 1); // the expected 400
  });

  await step('history', async () => {
    await page.goto(BASE + '/#/history');
    await page.getByText('Förde in beslut').first().waitFor();
    await shot('18-history');
  });

  await step('settings: theme, language, size, MFA setup shown', async () => {
    await page.goto(BASE + '/#/settings');
    await page.getByRole('button', { name: 'Höst' }).click();
    await page.waitForFunction(() => document.documentElement.dataset.theme === 'autumn');
    await page.getByLabel('Stor', { exact: true }).check();
    await page.waitForFunction(() => document.documentElement.dataset.size === 'l');
    await shot('19-settings-autumn');
    await page.getByLabel('English').check();
    await page.getByRole('heading', { name: 'Appearance' }).waitFor();
    await page.getByRole('button', { name: 'Dark' }).click();
    await page.getByLabel('Normal').check();
    await shot('20-settings-dark-en');
    await page.reload();
    await page.getByRole('heading', { name: 'Appearance' }).waitFor(); // language + theme persisted
    if (await page.evaluate(() => document.documentElement.dataset.theme) !== 'dark') throw new Error('theme not persisted');
    await page.goto(BASE + '/#/home');
    await page.getByText('Latest decisions').waitFor();
    await shot('21-home-dark');
  });

  await step('backup download', async () => {
    await page.goto(BASE + '/#/admin');
    await page.getByRole('tab', { name: 'Backup & export' }).click();
    await page.getByLabel('Passphrase', { exact: true }).first().fill('a very long passphrase');
    await page.getByLabel('Repeat passphrase').fill('a very long passphrase');
    const dl = page.waitForEvent('download');
    await page.getByRole('button', { name: 'Create backup' }).click();
    const d = await dl;
    const size = fs.statSync(await d.path()).size;
    if (size < 500 || !d.suggestedFilename().endsWith('.bak')) throw new Error('backup ' + d.suggestedFilename() + ' ' + size);
    await d.saveAs(OUT + '/ui-backup.bak');
  });

  await step('second user: forced pw change, sees shared, not private; live update', async () => {
    const ctx2 = await browser.newContext({ viewport: { width: 390, height: 780 }, locale: 'sv-SE', isMobile: true, hasTouch: true });
    const p2 = await ctx2.newPage();
    p2.on('pageerror', (e) => { console.log('  pageerror(p2):', e.message.slice(0, 300)); errors.push('pageerror'); });
    p2.setDefaultTimeout(6000);
    await p2.goto(BASE);
    await p2.getByLabel('Användarnamn').fill('bertil');
    await p2.getByLabel('Lösenord').fill('temporary-password-1');
    await p2.getByRole('button', { name: 'Logga in', exact: true }).click();
    await p2.getByLabel('Nuvarande lösenord').fill('temporary-password-1');
    await p2.getByLabel('Nytt lösenord', { exact: true }).fill(PW);
    await p2.getByLabel('Upprepa nytt lösenord').fill(PW);
    await p2.getByRole('button', { name: 'Byt lösenord' }).click();
    await p2.getByRole('button', { name: 'Meny' }).waitFor();
    // First sign-in: the welcome tour opens. Step through it, then check it stays away.
    const tour = p2.getByRole('dialog');
    await tour.waitFor();
    await p2.screenshot({ path: OUT + '/shot-29-mobile-tour.png' });
    await tour.getByRole('button', { name: 'Nästa' }).click();
    await tour.getByRole('button', { name: 'Nästa' }).click();
    await tour.getByRole('button', { name: 'Hoppa över' }).click();
    await tour.waitFor({ state: 'detached' });
    await p2.reload();
    await p2.getByRole('button', { name: 'Meny' }).waitFor();
    await p2.waitForTimeout(1200);
    if (await p2.getByRole('dialog').count()) throw new Error('the tour came back after being skipped');
    await p2.screenshot({ path: OUT + '/shot-30-mobile-home.png' });
    await p2.getByRole('button', { name: 'Meny' }).click();
    await p2.screenshot({ path: OUT + '/shot-31-mobile-menu.png' });
    if (await p2.getByRole('link', { name: 'Administration' }).count()) throw new Error('member sees admin link');
    await p2.getByRole('link', { name: 'Chatt' }).click();
    await p2.screenshot({ path: OUT + '/shot-32-mobile-chat-list.png' });
    await p2.getByRole('link', { name: /Anna Admin/ }).click();
    await p2.getByText('Hej Bertil!').waitFor();
    // live: admin writes, bertil sees it without reload
    await page.goto(BASE + '/#/chat');
    await page.getByRole('link', { name: /Bertil Berg/ }).click();
    await page.getByLabel('Write a message').fill('Live-test 123');
    await page.keyboard.press('Enter');
    await p2.getByText('Live-test 123').waitFor();
    await p2.screenshot({ path: OUT + '/shot-33-mobile-chat.png' });
    await p2.goto(BASE + '/#/lists');
    await p2.getByText('Inga listor än').waitFor(); // admin's list is private
    await p2.goto(BASE + '/#/calendar');
    await p2.locator('.cal-ev', { hasText: 'Styrelsemöte' }).first().waitFor();
    if (await p2.locator('.cal-ev', { hasText: 'Sommarfest' }).count()) throw new Error('private event leaked');
    await p2.screenshot({ path: OUT + '/shot-34-mobile-calendar.png' });
    await p2.goto(BASE + '/#/boards');
    await p2.goto(BASE + '/#/admin');
    await p2.goto(BASE + '/#/meetings');
    await p2.getByRole('link', { name: /Styrelsemöte/ }).click();
    await p2.getByText(/Justerat av Anna Admin/).waitFor();
    await p2.screenshot({ path: OUT + '/shot-35-mobile-meeting.png' });
    await ctx2.close();
  });

  await step('crafted links cannot steer requests to another endpoint', async () => {
    const sent = [];
    const watch = (r) => { if (r.url().includes('/api/')) sent.push(new URL(r.url()).pathname); };
    page.on('request', watch);
    for (const hash of ['#/lists/..%2Fadmin%2Fusers', '#/lists/..%2F..%2Fapi%2Fme', '#/documents/%2e%2e%2ffiles%2fabc', '#/boards/a%2Fb', '#/polls/%ZZ']) {
      await page.goto(BASE + '/' + hash);
      await page.waitForTimeout(300);
    }
    page.off('request', watch);
    const bad = sent.filter((p) => !/^\/api\/(channels|calendar|polls|decisions|events|me|users|groups|roles|lists|documents|boards)$/.test(p));
    if (bad.length) throw new Error('unexpected requests: ' + bad.join(', '));
    // The request helper itself refuses paths that are not plain segments.
    const res = await page.evaluate(async () => {
      const { api } = await import('/js/lib.js');
      const out = [];
      for (const p of ['/lists/../admin/users', '/lists/a%2Fb', '/lists//x', '/files/x/../../me']) {
        try { await api('GET', p); out.push('SENT ' + p); } catch (e) { out.push(e.code); }
      }
      return out;
    });
    if (res.some((x) => x !== 'bad_request_path')) throw new Error('path guard: ' + res.join(', '));
  });

  await step('two-step verification with recovery codes; passkey registration and sign-in', async () => {
    const totp = (secret) => {
      const crypto = require('crypto');
      const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
      let bits = '';
      for (const ch of secret) bits += alphabet.indexOf(ch).toString(2).padStart(5, '0');
      const key = Buffer.from(bits.match(/.{8}/g).map((b) => parseInt(b, 2)));
      const msg = Buffer.alloc(8);
      msg.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)));
      const h = crypto.createHmac('sha1', key).update(msg).digest();
      const o = h[h.length - 1] & 15;
      return String((h.readUInt32BE(o) & 0x7fffffff) % 1000000).padStart(6, '0');
    };
    const ctx3 = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: 'en-GB' });
    await ctx3.addInitScript(() => localStorage.setItem('samling.tour', 'off'));
    const p3 = await ctx3.newPage();
    p3.on('pageerror', (e) => { console.log('  pageerror(p3):', e.message.slice(0, 300)); errors.push('pageerror'); });
    p3.on('dialog', (d) => d.accept(PW));
    p3.setDefaultTimeout(6000);
    // A virtual authenticator with user verification stands in for a phone or security key.
    const cdp = await ctx3.newCDPSession(p3);
    await cdp.send('WebAuthn.enable');
    await cdp.send('WebAuthn.addVirtualAuthenticator', { options: { protocol: 'ctap2', transport: 'internal', hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true } });
    const signIn = async (code) => {
      await p3.goto(BASE);
      await p3.getByLabel('Username').fill('cecilia');
      await p3.getByLabel('Password').fill(code === undefined ? 'temporary-password-1' : PW);
      await p3.getByRole('button', { name: 'Sign in', exact: true }).click();
      if (code !== undefined && code !== null) {
        await p3.getByLabel('Verification code').fill(code);
        await p3.getByRole('button', { name: 'Sign in', exact: true }).click();
      }
    };
    await p3.goto(BASE);
    await p3.getByRole('combobox').selectOption('en');
    await signIn();
    // The account's language is Swedish until it is changed in the settings.
    await p3.getByLabel('Nuvarande lösenord').fill('temporary-password-1');
    await p3.getByLabel('Nytt lösenord', { exact: true }).fill(PW);
    await p3.getByLabel('Upprepa nytt lösenord').fill(PW);
    await p3.getByRole('button', { name: 'Byt lösenord' }).click();
    await p3.locator('nav.nav').waitFor();
    await p3.goto(BASE + '/#/settings');
    await p3.getByLabel('English').check();
    // Authenticator app + recovery codes
    const mfa = p3.locator('form', { has: p3.getByRole('button', { name: 'Set up two-step verification' }) });
    await mfa.getByLabel('Current password').fill(PW);
    await mfa.getByRole('button').click();
    const secret = (await p3.locator('code.secret').innerText()).trim();
    await p3.getByLabel('Verification code').fill(totp(secret));
    await p3.getByRole('button', { name: 'Turn on' }).click();
    await p3.locator('.codes li').nth(9).waitFor();
    const codes = await p3.locator('.codes li').allInnerTexts();
    await p3.screenshot({ path: OUT + '/shot-50-recovery-codes.png' });
    await p3.getByRole('button', { name: 'I have saved them' }).click();
    await p3.getByText('Unused recovery codes: 10.').waitFor();
    // Passkey registration
    const pk = p3.locator('form', { has: p3.getByRole('button', { name: 'Add a passkey' }) });
    await pk.getByLabel('Name, e.g. My phone').fill('Test key');
    await pk.getByLabel('Current password').fill(PW);
    await pk.getByRole('button', { name: 'Add a passkey' }).click();
    await p3.getByText('Test key').waitFor();
    await p3.screenshot({ path: OUT + '/shot-51-passkey.png' });
    // Sign in again with password + a recovery code (single use)
    await p3.getByRole('button', { name: 'Sign out', exact: true }).click();
    await signIn(codes[0]);
    await p3.locator('nav.nav').waitFor();
    await p3.getByRole('button', { name: 'Sign out', exact: true }).click();
    await signIn(codes[0]);
    await p3.getByText('Wrong username, password or code.').waitFor();
    // Sign in with the passkey: no user name, no password
    await p3.goto(BASE);
    await p3.getByRole('button', { name: 'Sign in with a passkey' }).click();
    await p3.locator('nav.nav').waitFor();
    await p3.goto(BASE + '/#/settings');
    await p3.getByText('last used').waitFor();
    await p3.getByText('Unused recovery codes: 9.').waitFor();
    // The administrator signs the account out everywhere.
    await page.goto(BASE + '/#/admin');
    await page.locator('tr', { hasText: '@cecilia' }).getByRole('button').click();
    await page.getByRole('button', { name: /Sign out everywhere \(\d active\)/ }).click();
    await page.getByText('The person was signed out on all devices').waitFor();
    await p3.reload();
    await p3.getByRole('button', { name: 'Sign in', exact: true }).waitFor();
    await ctx3.close();
    errors.splice(0, errors.length, ...errors.filter((x) => x !== 'console')); // expected 401s above
  });

  await step('online help in both languages; tour can be restarted', async () => {
    await page.goto(BASE + '/#/meetings');
    await page.locator('.page-head').getByRole('link', { name: 'Help' }).click();
    await page.waitForFunction(() => location.hash === '#/help/meetings');
    await page.getByRole('heading', { level: 1 }).waitFor();
    await shot('60-help-topic');
    await page.goto(BASE + '/#/help');
    await page.getByRole('searchbox').fill('passkey');
    await page.getByRole('link', { name: /Settings/ }).first().waitFor();
    await page.getByRole('searchbox').fill('');
    await shot('61-help');
    await page.getByRole('button', { name: /welcome tour/i }).click();
    const tour = page.getByRole('dialog');
    await tour.waitFor();
    await shot('62-tour');
    await page.keyboard.press('Escape');
    await tour.waitFor({ state: 'detached' });
  });

  await step('logout', async () => {
    await page.getByRole('button', { name: 'Sign out', exact: true }).click();
    await page.getByRole('button', { name: 'Sign in', exact: true }).waitFor();
    await shot('40-login');
  });

  await browser.close();
  console.log(errors.length ? 'PROBLEMS: ' + errors.join(', ') : 'ALL OK');
  process.exit(errors.length ? 1 : 0);
})();
