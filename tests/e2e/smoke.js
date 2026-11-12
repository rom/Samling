// Browser smoke test for Samling: drives the real client through every main
// feature as an administrator on a desktop viewport and as a member on a
// phone viewport. Run it with tests/e2e/run.sh, which builds the server and
// starts it on a scratch data directory.
//
// Environment: E2E_URL (server), E2E_ADMIN_PASSWORD (temporary password of
// the fresh "admin" account), E2E_OUT (directory for screenshots/downloads).
const { chromium } = require('playwright');
const fs = require('fs');
const BASE = process.env.E2E_URL || 'http://127.0.0.1:8090';
const temp = process.env.E2E_ADMIN_PASSWORD;
const OUT = process.env.E2E_OUT || __dirname + '/out';
fs.mkdirSync(OUT, { recursive: true });
const PW = 'correct horse battery staple';
const errors = [];
const step = async (name, fn) => { try { await fn(); console.log('ok   ' + name); } catch (e) { console.log('FAIL ' + name + ': ' + e.message.split('\n').slice(0,6).join(' | ')); errors.push(name); } };

(async () => {
  const browser = await chromium.launch();
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 }, locale: 'sv-SE', timezoneId: 'Europe/Stockholm', acceptDownloads: true });
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
    await page.getByRole('button', { name: 'Logga in' }).click();
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
    await page.getByRole('button', { name: 'Set up two-step verification' }).click();
    await page.locator('.qr img').waitFor();
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
    await p2.getByRole('button', { name: 'Logga in' }).click();
    await p2.getByLabel('Nuvarande lösenord').fill('temporary-password-1');
    await p2.getByLabel('Nytt lösenord', { exact: true }).fill(PW);
    await p2.getByLabel('Upprepa nytt lösenord').fill(PW);
    await p2.getByRole('button', { name: 'Byt lösenord' }).click();
    await p2.getByRole('button', { name: 'Meny' }).waitFor();
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

  await step('logout', async () => {
    await page.getByRole('button', { name: 'Sign out' }).click();
    await page.getByRole('button', { name: 'Sign in' }).waitFor();
    await shot('40-login');
  });

  await browser.close();
  console.log(errors.length ? 'PROBLEMS: ' + errors.join(', ') : 'ALL OK');
  process.exit(errors.length ? 1 : 0);
})();
