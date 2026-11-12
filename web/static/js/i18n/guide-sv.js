// Svenska texter för välkomstguiden och hjälpen. Nycklarna ska vara identiska med guide-en.js.
//
// Hjälptexterna använder ett enkelt radformat som views/help.js tolkar:
//   ”# ” underrubrik, ”1. ” numrerat steg, ”- ” punkt, ”> ” notis, tom rad = nytt stycke,
//   [[någon.nyckel]] sätter in texten på en knapp eller meny ur ordlistan (så att hjälpen
//   alltid citerar den verkliga texten), **text** är betoning.
export default {
  'nav.help': 'Hjälp',

  // ---------- välkomstguiden ----------
  'tour.step': 'Steg {n} av {total}', 'tour.skip': 'Hoppa över', 'tour.back': 'Tillbaka', 'tour.next': 'Nästa', 'tour.done': 'Klar',
  'tour.welcome.title': 'Välkommen till Samling',
  'tour.welcome.text': 'Samling är föreningens egen plats för att prata, spara dokument, planera och fatta beslut tillsammans. Den här korta guiden visar var allt finns. Den tar ungefär en minut.',
  'tour.nav.title': 'Menyn',
  'tour.nav.text': 'Menyn till vänster tar dig till alla delar. Hem ger en överblick över vad som är nytt.',
  'tour.nav.text_narrow': 'Menyknappen uppe till vänster öppnar menyn med alla delar. Hem ger en överblick över vad som är nytt.',
  'tour.chat.title': 'Chatt',
  'tour.chat.text': 'Prata i kanaler som många kan följa, eller skicka direktmeddelanden som bara mottagarna kan läsa. Du kan fästa viktiga meddelanden, reagera och kommentera, och bifoga filer eller bilder från kameran.',
  'tour.documents.title': 'Dokument',
  'tour.documents.text': 'Samla föreningens filer på ett ställe, sorterade i mappar. När du laddar upp en ny version finns de tidigare kvar.',
  'tour.sharing.title': 'Privat tills du delar',
  'tour.sharing.text': 'Allt du skapar är privat från början. Du väljer vilka som får åtkomst – alla, en grupp, en roll eller en enskild person – och om de bara får läsa eller också ändra.',
  'tour.lists.title': 'Listor, tavlor och omröstningar',
  'tour.lists.text': 'Bocka av uppgifter i en lista, följ arbetet på en tavla med kort och låt medlemmarna rösta i en omröstning.',
  'tour.calendar.title': 'Kalender, möten och beslut',
  'tour.calendar.text': 'Kalendern visar vad som är på gång. Möten har dagordning och protokoll, och det som beslutas sparas i beslutsloggen.',
  'tour.search.title': 'Sök',
  'tour.search.text': 'En enda sökning hittar meddelanden, dokument, personer och mer – men bara sådant som du har åtkomst till.',
  'tour.settings.title': 'Inställningar',
  'tour.settings.text': 'Välj språk, tema och textstorlek, och slå på de tillgänglighetsval du behöver. Skydda kontot med tvåstegsverifiering.',
  'tour.help.title': 'Hjälpen finns alltid nära',
  'tour.help.text': 'Hjälpen förklarar varje del steg för steg, och frågetecknet högst upp på en sida öppnar hjälpen för just den sidan. Du kan se den här guiden igen från Hjälp.',

  // ---------- hjälpen: gränssnitt ----------
  'help.intro': 'Här kan du läsa hur Samling fungerar, en del i taget. Sök på ett ord eller välj ett ämne nedan.',
  'help.tour_again': 'Visa välkomstguiden igen', 'help.search': 'Sök i hjälpen', 'help.none': 'Inget hjälpämne matchar din sökning.',
  'help.count': '{n} ämnen hittades', 'help.open': 'Öppna {name}', 'help.related': 'Relaterade ämnen', 'help.all': 'Alla hjälpämnen',
  'help.group.basics': 'Börja här', 'help.group.sections': 'Delarna i Samling', 'help.group.more': 'Mer', 'help.group.admin': 'Administration',

  // ---------- hjälpen: ämnen ----------
  'help.start.title': 'Kom igång',
  'help.start.intro': 'Logga in, hitta runt och gör några bra val från början.',
  'help.start.body': `# Logga in
1. Öppna adressen som du har fått av din administratör.
2. Skriv ditt användarnamn och lösenord och välj [[login.submit]].
3. Första gången, och när lösenordet har återställts, får du välja ett eget nytt lösenord.
4. Om tvåstegsverifiering är påslagen för ditt konto skriver du den sexsiffriga koden från din autentiseringsapp.

> Efter 10 felaktiga lösenord låses kontot i 15 minuter. En administratör kan låsa upp det eller ge dig ett tillfälligt lösenord.

# Hitta runt
- Menyn leder till alla delar. På en telefon öppnar du den med knappen [[nav.menu]] uppe till vänster.
- [[nav.home]] sammanfattar det som väntar på dig: olästa konversationer, kommande händelser, omröstningar att rösta i och de senaste besluten.
- Frågetecknet uppe till höger på en sida öppnar hjälpen för den sidan.
- Du loggar ut med knappen [[nav.logout]] bredvid ditt namn längst ner i menyn.

# Tre saker att göra först
1. Öppna [[nav.settings]] och välj språk, tema och textstorlek.
2. Kontrollera ditt namn och dina kontaktuppgifter under [[settings.profile]], och bestäm om andra medlemmar ska få se dem.
3. Slå på [[settings.mfa]] för att skydda ditt konto.

# Bra att veta
- Det du skapar är privat tills du delar det. Ämnet Delning och åtkomst förklarar hur.
- Nya meddelanden och de flesta andra ändringar dyker upp av sig själva. Du behöver sällan ladda om sidan.
- Samling fungerar i webbläsaren på dator, surfplatta och telefon, och kan läggas på telefonens hemskärm.`,

  'help.sharing.title': 'Delning och åtkomst',
  'help.sharing.intro': 'Allt du skapar är privat tills du delar det – och delning fungerar likadant överallt.',
  'help.sharing.body': `Listor, dokument, omröstningar, kalenderhändelser, tavlor, möten, beslut och kanaler delas alla på samma sätt. Det som finns är märkt [[share.private]] eller [[share.shared]], så att du ser skillnaden direkt.

# Vilka du kan dela med
- [[share.everyone]] – samtliga medlemmar.
- En grupp – någon av föreningens gemensamma grupper eller en av dina egna privata grupper.
- En roll – alla som har rollen, till exempel Ledare.
- En person – en enskild medlem.

# Läsa eller ändra
- [[share.read]]: personen kan se det du delar. I en omröstning betyder det också att personen får rösta.
- [[share.write]]: personen kan dessutom bidra – skriva i en kanal, lägga till rader i en lista, ladda upp en ny version av ett dokument, flytta kort på en tavla.

# Dela när du skapar något
1. Leta upp [[share.who]] i formuläret. (För ett möte heter fältet [[meet.invited]], för en omröstning [[polls.who]] och för ett beslut [[dec.audience]].)
2. Välj någon i listan [[share.add]]. Upprepa för att lägga till fler.
3. Välj [[share.read]] eller [[share.write]] på varje rad.
4. Lämna tomt om du vill ha det för dig själv.

# Ändra delningen i efterhand
1. Öppna det du vill dela och välj [[share.button]].
2. Lägg till rader, ändra behörigheten eller ta bort en rad med krysset.
3. Välj [[common.save]]. Tar du bort alla rader blir det privat igen.

# Bra att veta
- Bara ägaren kan ändra delningen eller ta bort det som delas. För sådant som är delat kan även en moderator (vanligtvis en ledare) göra det.
- Administratörer kan inte se det du har privat och inte heller dina direktmeddelanden.
- Nya kanaler och omröstningar föreslår [[share.everyone]] från början. Allt annat är privat tills du väljer något annat.
- Delar du ofta med samma personer? Gör en egen grupp under [[nav.people]].
- Omröstningar och beslut har ingen behörighetsnivå: de som en omröstning delas med kan se den och rösta, och de som ett beslut delas med kan läsa det.`,

  'help.home.title': 'Hem',
  'help.home.intro': 'Överblicken som du ser när du loggar in: vad som är nytt och vad som väntar på dig.',
  'help.home.body': `# Det här visar sidan
- [[home.unread]] – kanaler och direktmeddelanden med nya meddelanden. Välj en för att öppna den.
- [[home.upcoming]] – händelser och möten de närmaste 30 dagarna.
- [[home.to_vote]] – öppna omröstningar som du ännu inte har röstat i.
- [[home.decisions]] – de beslut som senast har förts in.

# Tips
- [[home.all]] i en ruta öppnar hela den delen.
- Sidan uppdateras av sig själv när något händer.
- Du ser bara sådant som du har åtkomst till, så två medlemmar kan ha olika startsidor.`,

  'help.chat.title': 'Chatt',
  'help.chat.intro': 'Kanaler för ämnen som rör många, och direktmeddelanden för privata samtal.',
  'help.chat.body': `# Läsa och skriva
1. Öppna [[nav.chat]] och välj en kanal under [[chat.channels]] eller en konversation under [[chat.dms]].
2. Skriv i fältet längst ner och tryck Enter eller välj [[chat.send]]. Skift+Enter ger en ny rad.
3. En siffra bredvid ett namn betyder olästa meddelanden. Nya meddelanden dyker upp av sig själva.

# Starta ett direktmeddelande
1. Välj plustecknet bredvid [[chat.dms]] ([[chat.new_dm]]).
2. Bocka för en eller flera mottagare – högst 19 – och välj [[chat.start]].

Bara deltagarna kan läsa ett direktmeddelande. Inte ens en administratör kan det. Du kan också välja [[people.message]] bredvid ett namn under [[nav.people]].

# Bifoga filer
1. Välj gemet ([[chat.attach]]) och välj en eller flera filer, högst 10 per meddelande.
2. Skicka meddelandet. Alla som kan läsa konversationen kan ladda ner en fil genom att välja den.

# Fästa, ändra och ta bort
- Peka på ett meddelande för att se dess knappar. På en telefon syns de hela tiden.
- Nålen ([[chat.pin]]) gör att något viktigt är lätt att hitta igen. [[chat.pins]] högst upp i konversationen visar allt som är fäst. Alla som får skriva i konversationen får fästa.
- Med pennan och papperskorgen ändrar du och tar bort dina egna meddelanden. Ett ändrat meddelande märks ut.

# Fet, kursiv, understruken och emoji
- Markera text och välj [[fmt.bold]], [[fmt.italic]] eller [[fmt.underline]] ovanför meddelandefältet, eller tryck Ctrl+B, Ctrl+I eller Ctrl+U. Texten får tecken omkring sig – två asterisker för fet, en för kursiv, två understreck för understruken – och visas formaterad när den har skickats.
- Smilisen ([[fmt.emoji]]) infogar en emoji där markören står.

# Reagera och kommentera
- [[chat.react]] på ett meddelande sätter en liten emoji under det, till exempel tummen upp, ett hjärta eller ett gråtande ansikte. Välj din reaktion igen för att ta bort den. Du kan reagera även i kanaler där du bara kan läsa.
- [[chat.comment]] öppnar ett fält under meddelandet. Kommentarer visas indragna under det meddelande de hör till, så att en diskussion hålls ihop.

# Bilder från kameran
- Kameran ([[cam.photo]]) tar en bild och bifogar den till ditt meddelande. Bilder visas direkt i samtalet; välj en bild för att se den större.
- [[cam.scan]] läser av texten i en bild och lägger den i meddelandefältet. Se avsnittet Bilder och kameran.

# Blockera direktmeddelanden
Om du inte vill ha direktmeddelanden från någon öppnar du samtalet med personen och väljer [[block.do]], eller använder samma knapp bredvid personen under [[nav.people]]. Personen kan då varken skriva till dig eller starta ett nytt samtal med dig. Personen får inget besked, och kanaler påverkas inte. Häv blockeringen med [[block.undo]], på samma ställe eller under [[nav.settings]].

# Kanaler
- För att skapa en kanal krävs behörigheten [[perm.channels.create]], som ledare vanligtvis har. Använd plustecknet bredvid [[chat.channels]] ([[chat.new_channel]]).
- Kanalens delning avgör vilka som får läsa och vilka som får skriva. En kanal som alla får läsa men bara några får skriva i fungerar som en anslagstavla. I en sådan kanal ser du ett hänglås och texten [[chat.readonly]].
- Länkar till videomöten i Teams, Zoom, Google Meet och liknande tjänster visas som en knapp för att ansluta.`,

  'help.documents.title': 'Dokument',
  'help.documents.intro': 'Arkivet för filer som ska sparas: stadgar, protokoll, blanketter och liknande.',
  'help.documents.body': `# Ladda upp ett dokument
1. Välj [[docs.upload]].
2. Välj filen. Ge den ett namn om du vill ha något annat än filnamnet.
3. Skriv en mapp om du vill, till exempel Styrelsen/2026 (ett snedstreck ger en undermapp), och en beskrivning.
4. Välj under [[share.who]] vilka som ska få åtkomst – eller lämna tomt för att ha dokumentet privat.
5. Välj [[docs.upload]].

# Hitta och öppna
- Begränsa listan till en mapp med mapplistan högst upp, eller skriv en del av namnet i [[common.filter]].
- Välj ett dokument och sedan [[common.download]]. Filen sparas på din enhet och öppnas i ditt eget program. Dokument öppnas aldrig inne i Samling.

# Ladda upp en ny version
1. Öppna dokumentet.
2. Beskriv gärna vad som har ändrats i kommentarsfältet.
3. Välj [[docs.new_version]] och välj filen.

Alla tidigare versioner finns kvar i listan under [[docs.versions]] och går fortfarande att ladda ner. Du behöver [[share.write]] för att ladda upp en ny version.

# Mer
- [[docs.copy]] ger dig en privat kopia av den aktuella versionen.
- [[common.edit]] ändrar namn, mapp och beskrivning.
- Ägaren kan ändra delningen med [[share.button]] och ta bort dokumentet med [[common.delete]].
- Sökningen hittar dokument på namn, mapp och beskrivning, inte på det som står i filen. Ge dokumenten namn som säger vad de är.
- Din administratör bestämmer hur stor en fil får vara.`,

  'help.lists.title': 'Listor',
  'help.lists.intro': 'Att göra-listor, prioriteringslistor och enkla listor – egna eller delade med andra.',
  'help.lists.body': `# Tre typer
- [[lists.kind_todo]] – rader som du bockar av, med ett datum när det ska vara klart om du vill.
- [[lists.kind_priority]] – som ovan, och varje rad har dessutom en prioritet. De viktigaste raderna visas först.
- [[lists.kind_plain]] – bara rader, till exempel en packlista.

# Skapa en lista
1. Välj [[lists.new]].
2. Ge listan en rubrik och välj [[lists.kind]]. Typen väljs en gång, när listan skapas.
3. Välj vilka du vill dela med, eller låt listan vara privat, och välj [[common.create]].

# Använda en lista
1. Skriv i fältet [[lists.add_placeholder]], välj prioritet och datum om listan har det, och välj [[common.add]].
2. Bocka i rutan när något är klart.
3. Flytta en rad med pilarna och ta bort den med papperskorgen. I en prioriteringslista styr prioriteten ordningen i stället.

# Ansvariga, bilder och skannad text
- Välj en [[lists.owner]] för en rad för att visa vem som tar hand om den. Alla som kan ändra listan kan välja eller byta ansvarig.
- Under [[att.title]] kan du bifoga filer eller ta en bild med kameran – ett kvitto, en skiss, anslagstavlan.
- [[cam.scan]] läser av en tryckt lista från en bild. Kontrollera texten, så blir varje rad en rad i listan.

# Bra att veta
- Ett datum som har passerat visas i rött.
- När någon annan ändrar i en delad lista ser du det direkt.
- Med [[share.read]] kan du se listan men inte ändra i den. Med [[share.write]] kan du lägga till, bocka av och ta bort rader.`,

  'help.polls.title': 'Omröstningar',
  'help.polls.intro': 'Ställ en fråga och låt medlemmarna rösta – från ”vilken dag passar bäst?” till ett formellt ja eller nej.',
  'help.polls.body': `# Rösta
1. Öppna omröstningen från [[nav.polls]], eller från [[home.to_vote]] på startsidan.
2. Gör ditt val och välj [[polls.vote]].
3. Om omröstningen tillåter det kan du välja [[polls.change]] fram till att den avslutas.

# Skapa en omröstning
1. Välj [[polls.new]] och skriv din [[polls.question]].
2. Välj [[polls.kind]] och fyll i alternativen, mellan 2 och 30 stycken.
3. Välj när resultatet ska visas ([[polls.results]]) och, om du vill, en sluttid ([[polls.closes]]).
4. Välj under [[polls.who]] vilka som ska kunna se omröstningen och rösta. Välj sedan [[common.create]].

# Röstningsmetoder
- [[polls.kind_single]] – alla väljer exakt ett alternativ.
- [[polls.kind_multi]] – alla får välja flera, med en övre gräns om du sätter en.
- [[polls.kind_yesno]] – ja, nej eller avstår. Passar för formella omröstningar.
- [[polls.kind_ranked]] – alla ordnar alternativen. En högre placering ger fler poäng.
- [[polls.kind_score]] – alla ger varje alternativ 0 till 5 poäng.

# Bra att veta
- Med [[polls.anonymous]] sparas rösterna utan koppling till den som har röstat. Ingen, inte ens en administratör, kan se vem som röstade på vad. En anonym röst går inte att ändra i efterhand.
- Ägaren kan avsluta omröstningen i förtid med [[polls.close]]. Resultatet finns kvar.
- En medlem som får föra in beslut kan göra ett beslut av en avslutad omröstning med [[polls.to_decision]].`,

  'help.calendar.title': 'Kalender',
  'help.calendar.intro': 'Dina egna händelser, händelser som andra har delat med dig och de möten du är kallad till.',
  'help.calendar.body': `# Lägg till en händelse
1. Välj [[cal.new]], eller välj datumsiffran för en dag i månadsvyn.
2. Fyll i rubrik, start och slut. Bocka för [[cal.all_day]] om händelsen inte har några klockslag.
3. Lägg till plats, länk (till exempel till ett videomöte) och beskrivning om du vill.
4. Välj under [[share.who]] vilka du vill dela med, eller lämna tomt för en privat händelse. Välj [[common.save]].

# Titta, ändra och ta bort
- Pilarna byter månad och [[cal.today]] tar dig tillbaka.
- Välj en händelse för att se detaljerna. Därifrån kan du använda [[common.edit]], [[share.button]] och [[common.delete]] på dina egna händelser.
- [[cal.mail]] öppnar ditt e-postprogram med uppgifterna ifyllda, och knappen .ics laddar ner just den händelsen.
- [[cal.upcoming]] under månaden visar vad som kommer härnäst.
- Möten som planeras i Samling dyker upp av sig själva. Dem ändrar du från mötet: välj [[cal.open_meeting]].

# Andra kalendrar
- [[cal.export]] laddar ner de händelser du kan se som en .ics-fil, som Outlook, Google Kalender, Apple Kalender med flera kan läsa. Det är en kopia av läget just nu och den uppdateras inte av sig själv.
- [[cal.import]] lägger in händelserna från en .ics-fil i din privata kalender. Importerar du samma fil igen uppdateras händelserna i stället för att dubbleras.

> Samling har inga återkommande händelser. Av en återkommande händelse i en importerad fil tas bara det första tillfället med.`,

  'help.meetings.title': 'Möten',
  'help.meetings.intro': 'Från kallelse till justerat protokoll i fyra steg: planera, dagordning, protokoll, justera.',
  'help.meetings.body': `Alla som är kallade kan följa mötet, dagordningen och protokollet. För att planera ett möte krävs behörigheten [[perm.meetings.create]], som ledare vanligtvis har.

# 1. Planera
1. Välj [[meet.new]] och fyll i rubrik, tid och en plats eller en länk.
2. Låt [[meet.use_template]] vara förbockad för att få de vanliga punkterna att utgå från.
3. Välj under [[meet.invited]] vilka mötet är till för. Med [[share.write]] kan en person hjälpa till att skriva protokollet.
4. Välj [[common.create]].

Mötet läggs in i kalendern hos alla kallade och följer de ändringar du gör.

# 2. Dagordning
1. Öppna mötet och gå till steget [[meet.step_agenda]].
2. Ändra punkterna: rubrik, [[meet.presenter]], tid i minuter och [[meet.background]]. Pilarna ändrar ordningen och [[meet.add_item]] lägger till en punkt.
3. Välj [[common.save]]. Den sammanlagda planerade tiden visas under punkterna.

# 3. Protokoll
1. Gå till steget [[meet.step_protocol]] och fyll i [[meet.chair]], [[meet.secretary]] och [[meet.attendees]], ett namn per rad.
2. Skriv anteckningar under varje punkt.
3. När mötet beslutar något väljer du [[dec.record]] under punkten. Beslutet förs direkt in i beslutsloggen.
4. Välj [[common.save]] då och då. Inget sparas förrän du gör det.

# 4. Justera
1. Gå till steget [[meet.step_finalize]] och välj [[meet.finalize]]. Protokollet låses då.
2. [[meet.export_md]] laddar ner protokollet som en textfil, och [[meet.print]] skriver ut det eller sparar det som PDF via webbläsaren.

> Om något behöver rättas kan mötets ägare välja [[meet.reopen]]. Både justeringen och att protokollet öppnas igen noteras i loggen.`,

  'help.decisions.title': 'Beslut',
  'help.decisions.intro': 'Beslutsloggen: vad som har beslutats, när, och hur det har ändrats sedan dess.',
  'help.decisions.body': `# Läsa beslut
- [[dec.log]] visar de beslut du får se. Varje beslut har ett nummer inom sitt år, till exempel 2026-001. Begränsa listan med år eller genom att skriva i [[common.filter]].
- [[dec.timeline]] lägger beslut och avslutade omröstningar på en gemensam tidslinje.
- Öppna ett beslut för att läsa hela texten, se om det har status [[dec.status_decided]], [[dec.status_revoked]] eller [[dec.status_superseded]] och följa dess historik. Kommer beslutet från ett möte eller en omröstning finns en länk dit.
- [[dec.export]] laddar ner loggen som en fil för ett kalkylprogram.

# Föra in ett beslut
Det här kräver behörigheten [[perm.decisions.create]], som ledare vanligtvis har.
1. Välj [[dec.record]].
2. Skriv en kort rubrik, [[dec.text]] och [[dec.date]].
3. Välj under [[dec.audience]] vilka som får läsa beslutet.
4. Välj [[dec.record]].

Du kan också föra in ett beslut direkt från en punkt i ett mötesprotokoll, eller från en avslutad omröstning med [[polls.to_decision]].

# Revidera ett beslut
1. Öppna beslutet och välj [[dec.revise]].
2. Ändra texten eller statusen och ange [[dec.reason]]. Ett skäl måste anges.
3. Välj [[common.save]].

> Ett beslut tas aldrig bort och skrivs aldrig över. Varje ändring, även att beslutet upphävs, sparas i beslutets historik tillsammans med den tidigare texten.`,

  'help.boards.title': 'Tavlor',
  'help.boards.intro': 'Följ arbetet från idé till klart med kort som flyttas mellan kolumner.',
  'help.boards.body': `# Skapa en tavla
1. Välj [[boards.new]] och ge tavlan en rubrik.
2. Skriv kolumnerna med kommatecken emellan, eller behåll förslaget. En tavla kan ha 1 till 12 kolumner.
3. Välj vilka du vill dela med, eller låt tavlan vara privat, och välj [[common.create]].

# Arbeta med kort
1. Välj [[boards.new_card]] under en kolumn. Ge kortet en rubrik och, om du vill, en beskrivning, [[boards.assignee]] och [[lists.due]].
2. Flytta ett kort genom att dra det till en annan kolumn, eller använd pilarna under kortet. Pilarna fungerar också med tangentbordet och på en telefon.
3. Välj ett kort för att öppna det, ändra det eller ta bort det.

# Färger och bilder
- Ge ett kort en [[color.label]] när du öppnar det, till exempel rött för brådskande. Färgen visas som en rand på kortet.
- Tavlans ägare kan välja [[boards.color]], så blir tavlan lätt att känna igen.
- Ett öppnat kort har [[att.title]], där du kan bifoga filer eller en bild från kameran. [[cam.scan]] under beskrivningen läser in text från en bild till beskrivningen.

# Kolumner
Tavlans ägare sköter kolumnerna:
- [[boards.add_column]] lägger till en kolumn.
- Pennan byter namn på en kolumn och pilarna flyttar den åt vänster eller höger.
- En tom kolumn kan tas bort med papperskorgen. Flytta korten först.

# Bra att veta
- Med [[share.write]] kan du lägga till, ändra och flytta kort. Med [[share.read]] kan du bara titta.
- Ett datum som har passerat visas i rött på kortet.`,

  'help.search.title': 'Sök',
  'help.search.intro': 'En sökning i allt du har åtkomst till: meddelanden, dokument, listor, omröstningar, kalendern, möten, beslut, tavlor och personer.',
  'help.search.body': `# Söka
1. Öppna [[nav.search]] i menyn. På en telefon finns också ett förstoringsglas högst upp.
2. Skriv det du letar efter och välj [[search.go]].
3. Träffarna är ordnade efter typ. Välj en träff för att öppna den.

# Avgränsa sökningen
Välj [[search.advanced]] för att få fler val:
- [[search.mode]]: [[search.mode_plain]] hittar texten precis som du skrev den. Med [[search.mode_wildcard]] står en stjärna för valfri text och ett frågetecken för ett tecken, så budget 202? hittar både budget 2025 och budget 2026. [[search.mode_regex]] är till för den som kan sådana uttryck.
- [[search.from]] och [[search.to]] begränsar sökningen till en period.
- [[search.where]] begränsar sökningen till vissa typer av innehåll. Är inget förbockat söker du överallt.
- [[search.case]] gör att sökningen skiljer på stora och små bokstäver. Annars gör den inte det.

# Bra att veta
- Du får bara träffar från sådant som du har åtkomst till.
- Innehållet i uppladdade filer genomsöks inte – bara deras namn, mappar och beskrivningar.
- Andra medlemmars kontaktuppgifter går bara att hitta om de har valt att visa dem.`,

  'help.people.title': 'Medlemmar',
  'help.people.intro': 'Medlemsförteckningen, och grupperna som gör det snabbt att dela.',
  'help.people.body': `# Hitta en medlem
- Fliken [[people.directory]] visar alla medlemmar med namn, användarnamn och roll. Skriv i sökfältet för att hitta någon.
- E-post och telefon visas för de medlemmar som har valt att visa dem. Annars står det [[people.no_contact]]. Välj en e-postadress för att skriva ett meddelande i ditt e-postprogram.
- [[people.message]] startar ett direktmeddelande i chatten.
- Medlemmar kan också visa adress, intressen, specialkunskaper och en kort presentation. Du ser bara det som var och en har valt att visa.
- Den överstrukna cirkeln ([[block.do]]) stoppar direktmeddelanden från personen. Läs mer i avsnittet Chatt.

# Grupper
På fliken [[people.groups]] finns två sorter:
- [[people.global_groups]] skapas av administratörerna – sektioner, lag, styrelsen. Alla kan se dem och använda dem vid delning.
- [[people.local_groups]] är bara dina. Använd dem för att snabbt dela med samma personer.

# Gör en egen grupp
1. Gå till [[people.groups]] och välj [[people.new_group_local]].
2. Ge gruppen ett namn, bocka för medlemmarna och välj [[common.save]].
3. När du delar något finns gruppen nu i listan som [[share.my_group]].

# Bra att veta
- Ingen annan kan se dina egna grupper. När du delar med en sådan ser andra bara att det är delat med [[share.hidden_group]].
- Ändra eller ta bort en grupp med pennan på gruppen.`,

  'help.history.title': 'Historik',
  'help.history.intro': 'Din egen logg över vad du har gjort i Samling. Bara du kan se den.',
  'help.history.body': `# Det här hittar du
- Vad du har skapat, ändrat, röstat i och laddat ner, dag för dag och med klockslag.
- Många rader är länkar som tar dig tillbaka till det som raden gäller.

# Använda historiken
- [[history.show]] begränsar listan till en typ av innehåll.
- [[history.more]] längst ner hämtar äldre aktivitet.

# Bra att veta
- Historiken är praktisk när du vill hitta något som du nyligen arbetade med.
- Ingen annan kan se din historik, inte ens en administratör. Säkerhetshändelser, som inloggningar och ändrad delning, noteras också i en särskild logg som bara de med rätt behörighet kan läsa.
- Historiken följer med när du laddar ner dina uppgifter under [[nav.settings]].`,

  'help.settings.title': 'Inställningar',
  'help.settings.intro': 'Språk, utseende, din profil och säkerheten för ditt konto.',
  'help.settings.body': `# Utseende
- [[settings.language]]: svenska eller engelska. Du kan byta när du vill.
- [[settings.theme]]: sex teman, bland dem [[theme.dark]] och [[theme.contrast]].
- [[settings.text_size]]: fyra storlekar, från [[size.s]] till [[size.xl]].
- [[settings.a11y]]: se ämnet Tillgänglighet.

Dina val börjar gälla direkt och följer med ditt konto till andra enheter.

# Profil
1. Ändra [[people.name]], [[people.email]] och [[people.phone]] under [[settings.profile]].
2. Bocka i eller ur [[settings.contact_visible]]. När rutan inte är ibockad kan bara kontoadministratörer se din e-post och ditt telefonnummer.
3. Välj [[common.save]].

Bara en administratör kan ändra ditt användarnamn.

# Profilbild och mer om dig
- Under [[profile.picture]] väljer du [[profile.upload]], tar en bild med kameran eller väljer en av de färdiga bilderna. Utan bild visas din initial.
- Under [[profile.more]] kan du fylla i [[profile.address]], [[profile.interests]], [[profile.specialities]] och [[profile.about]]. Varje fält visas för andra medlemmar bara om du bockar för [[profile.share]] bredvid det. Kontoadministratörer kan se fälten ändå.
- Välj [[common.save]] när du är klar.

# Blockerade personer
[[block.title]] visar de personer vars direktmeddelanden du har blockerat. Med [[block.undo]] kan de skriva till dig igen.

# Tvåstegsverifiering
Med tvåstegsverifiering loggar du in med lösenordet och en kod från din telefon. Då räcker det inte med ett stulet lösenord.
1. Installera en autentiseringsapp på telefonen, till exempel Google Authenticator eller Microsoft Authenticator.
2. Välj [[settings.mfa_start]] och skanna QR-koden med appen, eller skriv in nyckeln för hand.
3. Skriv den sexsiffriga kod som appen visar och välj [[settings.mfa_confirm]].

För att stänga av skriver du ditt lösenord och väljer [[settings.mfa_disable]] – om inte en administratör kräver tvåstegsverifiering för ditt konto. Har du blivit av med telefonen kan en administratör återställa den åt dig.

> När du slår på tvåstegsverifiering får du tio återställningskoder. Varje kod fungerar en gång i stället för koden från appen, till exempel om du blir av med telefonen. Spara dem på ett säkert ställe, skilt från telefonen; [[settings.recovery_new]] ersätter dem med en ny uppsättning.

# Passnycklar

Med en passnyckel loggar du in med fingeravtryck, ansikte eller enhetens PIN-kod, utan att skriva lösenord eller kod.

1. Under [[settings.passkeys]] ger du passnyckeln ett namn, anger ditt lösenord och väljer [[settings.passkey_add]].
2. Följ anvisningarna på din enhet.
3. Nästa gång väljer du [[login.passkey]] på inloggningssidan.

# Inloggade enheter

[[settings.sessions_end]] loggar ut dig på alla andra telefoner, surfplattor och datorer – gör det om du blir av med en enhet. Om föreningen har satt en tidsgräns loggas du också ut automatiskt efter en stund utan aktivitet.

# Lösenord
- Under [[settings.change_password]] anger du ditt nuvarande lösenord och det nya två gånger. En mening med flera ord är både stark och lätt att komma ihåg.
- När du byter lösenord loggas du ut på dina andra enheter.
- För din säkerhet kan du loggas ut automatiskt när du inte har använt Samling på en stund.

# Mina uppgifter
[[settings.export]] laddar ner en kopia av de personuppgifter som finns sparade om dig. Läs mer i ämnet Integritet och dina uppgifter.`,

  'help.privacy.title': 'Integritet och dina uppgifter',
  'help.privacy.intro': 'Vad andra kan se om dig, vad som förblir privat och hur du får en kopia av dina uppgifter.',
  'help.privacy.body': `# Det här ser andra
- Ditt namn, ditt användarnamn och dina roller visas för alla medlemmar i medlemsförteckningen.
- Din e-post och ditt telefonnummer visas för andra medlemmar bara om du har bockat för [[settings.contact_visible]] under [[nav.settings]]. Annars ser bara kontoadministratörer dem.
- Din profilbild visas för alla medlemmar. De frivilliga profilfälten – adress, intressen, specialkunskaper och presentation – visas för andra medlemmar bara om du bockar för [[profile.share]] för vart och ett av dem.
- Det du skriver eller laddar upp på en delad plats kan ses av dem som det delas med.

# Det här förblir privat
- Sådant som du inte har delat kan bara du se. Administratörer kan inte se det.
- Direktmeddelanden kan bara läsas av deltagarna.
- Din [[nav.history]] visas bara för dig.
- I en anonym omröstning kan ingen se hur du röstade.
- Text som du skannar med kameran läses av på din egen enhet. Bilden sparas bara om du väljer att bifoga den.
- Ingen får veta att du har blockerat deras direktmeddelanden.

# Säkerhetsloggen
Händelser som har betydelse för säkerheten – inloggningar, ändringar av konton och roller, ändrad delning, säkerhetskopior och exporter – noteras med tidpunkt, konto och nätverksadress. Loggen kan bara läsas av dem som har fått den behörigheten, och posterna sparas i två år.

# Få en kopia av dina uppgifter
1. Öppna [[nav.settings]] och leta upp [[settings.privacy]].
2. Välj [[settings.export]]. Du får en fil med din profil, din aktivitet och dina egna meddelanden.

# Lämna Samling
Be en administratör att ta bort ditt konto. Då raderas dina personuppgifter och ditt privata innehåll. Det du har bidragit med på delade platser finns kvar och visas som skrivet av [[common.deleted_user]].

> Samling installeras och drivs av din egen organisation. Fråga din administratör om du vill veta var servern finns eller vem som ansvarar för personuppgifterna.`,

  'help.mentions.title': 'Omnämnanden och länkar',
  'help.mentions.intro': 'Få någons uppmärksamhet med @, och länka till omröstningar, tavlor och annat i Samling.',
  'help.mentions.body': `# Nämn någon
1. Skriv @ i ett chattmeddelande eller en kommentar och börja skriva ett namn. Välj personen i listan, eller skriv användarnamnet själv – till exempel @robert.
2. Skicka meddelandet. Namnet markeras, och personen får en avisering.

Personen ser aviseringen som en siffra bredvid [[nav.mentions]] i menyn. [[nav.mentions]] visar meddelandena; välj ett för att öppna samtalet. När samtalet öppnas markeras dess omnämnanden som lästa, och [[mention.read_all]] rensar alla.

# Vem får veta
- Bara personer som kan läsa samtalet får en avisering. Om du nämner någon i en kanal som personen inte har åtkomst till, eller i ett direktmeddelande som personen inte deltar i, händer ingenting: personen får ingen avisering och får inte veta något om meddelandet.
- Ett omnämnande ger aldrig någon åtkomst.
- Om en person senare förlorar åtkomsten till kanalen, eller om meddelandet tas bort, försvinner omnämnandet ur personens lista.
- Du får ingen avisering när du nämner dig själv.

# Länka till något i Samling
1. Välj [[link.insert]] – kedjesymbolen ovanför meddelandefältet i chatten, eller under textfälten i möten, kalenderhändelser, kort, omröstningar, beslut och dokument.
2. Sök eller välj en typ, och välj det du vill länka till: en omröstning, en tavla, ett dokument, en lista, ett möte, ett beslut, en kalenderhändelse eller en kanal.
3. En kort kod läggs in i din text. När texten visas blir koden en länk med det aktuella namnet på det du länkar till.

Du kan också kopiera adressen till en sida i Samling från webbläsaren och klistra in den i en text; den blir samma sorts länk.

I mötesanteckningar och andra texter som du redigerar i ett textfält visas länkarna under fältet som [[link.in_text]], så att de kan öppnas därifrån.

# Länkar och åtkomst
- En länk visar namnet på det den pekar på bara för läsare som har åtkomst till det. Andra ser bara att det finns en länk till exempelvis en omröstning som de inte kan öppna.
- En länk delar ingenting. För att någon ska kunna öppna det du länkar till delar du det med personen – se avsnittet Delning och åtkomst.`,

  'help.camera.title': 'Bilder och kameran',
  'help.camera.intro': 'Ta en bild där du står, eller låt Samling läsa av texten i en bild.',
  'help.camera.body': `# Ta en bild
Överallt där du ser kameraknappen ([[cam.photo]]) kan du ta en bild direkt in i Samling:
- **Chatt** – bilden bifogas till ditt meddelande.
- **Dokument** – bilden blir dokumentet, eller en ny version av det.
- **Listor, kalenderhändelser, kort på tavlor, möten, omröstningar och beslut** – bilden läggs under [[att.title]].
- **Inställningar** – som din profilbild.

1. Välj [[cam.photo]]. Första gången frågar webbläsaren om Samling får använda kameran – tillåt det.
2. Sikta och välj [[cam.take]].

Utan kamera, eller om du hellre använder en bild du redan har, väljer du [[cam.choose]].

# Skanna text
[[cam.scan]] läser av tryckt text i en bild, så att du slipper skriva av den.
1. Välj [[cam.scan]] och ta en bild av texten – rakt framifrån, nära och i bra ljus.
2. Vänta medan texten läses av. Det kan ta upp till en minut första gången.
3. Kontrollera texten och rätta där det behövs. Bocka för [[cam.keep_photo]] om du vill spara bilden också.
4. Välj [[cam.use_text]].

Här hamnar texten:
- **Chatt** – i meddelandefältet.
- **Listor** – varje rad blir en rad i listan.
- **Dokument, kalenderhändelser, kort, omröstningar och beslut** – i beskrivningen, eller i beslutets text.

# Bra att veta
- Texten läses av på din egen enhet. Bilden skickas inte någonstans om du inte väljer att bifoga den.
- Tryckt text på svenska och engelska fungerar bäst. Handskrift blir sällan rätt.
- Bilder förminskas innan de skickas, så att de tar mindre av ditt lagringsutrymme.
- Vem som kan se en bifogad bild följer delningen av det den är bifogad till.
- Kameran kräver en säker anslutning (https).`,

  'help.accessibility.title': 'Tillgänglighet',
  'help.accessibility.intro': 'Samling är byggt för att fungera med tangentbord, skärmläsare, förstorad text och små skärmar – och du kan anpassa det ytterligare.',
  'help.accessibility.body': `# Anpassa utseendet
Öppna [[nav.settings]]. Under [[settings.appearance]] finns:
- [[settings.text_size]] – upp till [[size.xl]]. Webbläsarens zoom fungerar också.
- [[settings.theme]] – [[theme.contrast]] ger vitt och gult på svart. [[theme.dark]] är skonsammare på kvällen.
- [[a11y.motion]] – stänger av animationer och övergångar.
- [[a11y.underline]] – så att länkar inte bara känns igen på färgen.
- [[a11y.readable]] – ett bredare typsnitt med mer luft mellan bokstäver, ord och rader.
- [[a11y.focus]] – en tjockare ram runt det som tangentbordet står på.

Valen sparas med ditt konto och gäller på alla enheter där du loggar in.

# Tangentbord och skärmläsare
- Allt går att göra med tangentbordet. Se ämnet Tangentbordstips.
- Varje sida börjar med länken [[nav.skip]], som hoppar förbi menyn.
- Sidorna har rubriker, fälten har etiketter och meddelanden som ”sparat” läses upp av skärmläsare.
- Kort på en tavla och rader i en lista kan flyttas med knappar, så att du slipper dra.

# Om något inte fungerar
Berätta för din administratör vad du försökte göra och vilket hjälpmedel du använder. Det är så tillgänglighetsproblem upptäcks och rättas.`,

  'help.install.title': 'Installera som app på telefonen',
  'help.install.intro': 'Lägg Samling på hemskärmen så öppnas det som en app – utan någon appbutik.',
  'help.install.body': `# iPhone och iPad
1. Öppna Samling i Safari och logga in.
2. Tryck på delningsknappen, fyrkanten med en pil uppåt.
3. Välj ”Lägg till på hemskärmen” och sedan ”Lägg till”.

# Android
1. Öppna Samling i Chrome och logga in.
2. Tryck på menyn med tre prickar.
3. Välj ”Installera appen” eller ”Lägg till på startskärmen”.

# Dator
I Chrome och Edge kan du installera Samling som ett eget fönster: leta efter ”Installera” i adressfältet eller i webbläsarens meny.

# Bra att veta
- Appen är samma Samling som i webbläsaren, med samma konto och samma innehåll.
- Du behöver vara ansluten till internet. Inget sparas på telefonen för att läsas utan uppkoppling.
- Det finns inga aviseringar. Öppna appen för att se vad som är nytt – startsidan och siffran bredvid Chatt visar vad du inte har läst.
- Vad menyvalen heter varierar lite mellan olika versioner av iOS, Android och webbläsarna.
- Du tar bort appen som vilken app som helst. Inget i Samling raderas när du gör det.`,

  'help.keyboard.title': 'Tangentbordstips',
  'help.keyboard.intro': 'Samling går att använda helt utan mus. Här är tangenterna som är bra att känna till.',
  'help.keyboard.body': `# Överallt
- **Tabb** går till nästa länk, knapp eller fält, och **Skift+Tabb** går tillbaka.
- **Enter** följer en länk eller trycker på en knapp. **Mellanslag** trycker på en knapp och bockar i en ruta.
- Första trycket på Tabb på en sida visar länken [[nav.skip]]. Tryck Enter för att hoppa förbi menyn.
- **Escape** stänger en dialogruta. Så länge en dialogruta är öppen stannar Tabb inne i den.
- I en lista med val väljer du med **piltangenterna**.

# I chatten
- **Enter** skickar meddelandet.
- **Skift+Enter** ger en ny rad.

# Listor och tavlor
- Rader och kort har pilknappar för att flytta. Gå till en knapp med Tabb och tryck Enter.
- På en tavla väljer du ett kort med Enter för att öppna det.

# Kalendern
- Varje dag har en knapp med datumsiffran som lägger till en händelse den dagen.
- Varje händelse är en knapp som öppnar händelsen.

# Välkomstguiden
- **Enter** tar dig till nästa steg, **vänsterpil** och **högerpil** går mellan stegen och **Escape** hoppar över guiden.

# Tips
- [[a11y.focus]] under [[nav.settings]] gör det lättare att se var tangentbordet står.
- Webbläsarens kortkommandon fungerar som vanligt: Ctrl och plus eller minus ändrar zoomen (Cmd på en Mac), och Ctrl+F söker på sidan.`,

  'help.admin.title': 'För administratörer',
  'help.admin.intro': 'Konton, roller och behörigheter, grupper, loggen och säkerhetskopior.',
  'help.admin.body': `[[nav.admin]] visas i menyn för den som har minst en administrativ behörighet. Du ser bara de flikar som dina behörigheter omfattar.

# Konton
1. Välj [[admin.new_user]] på fliken [[admin.tab_accounts]].
2. Fyll i användarnamn, namn och roller. Välj [[admin.generate]] för att få ett [[admin.temp_password]].
3. Lämna lösenordet till personen på ett säkert sätt. Personen måste välja ett eget lösenord vid första inloggningen.

Välj [[common.edit]] på ett konto för att:
- ändra användarnamn, namn och roller
- bocka för [[admin.mfa_required]], så att personen måste aktivera tvåstegsverifiering innan kontot kan användas
- bocka för [[admin.disabled]], som hindrar personen från att logga in utan att något tas bort
- ge ett nytt tillfälligt lösenord med [[admin.set_password]], vilket också loggar ut personen överallt
- använda [[admin.reset_mfa]] när någon har blivit av med telefonen, eller [[admin.unlock]] efter för många felaktiga lösenord
- välja [[admin.delete_user]]. Då raderas personens personuppgifter och privata innehåll, och det går inte att ångra. Det personen har bidragit med på delade platser finns kvar.

> Det måste alltid finnas minst en administratör.

Om någon blir av med en telefon eller dator öppnar du personens konto och väljer knappen för att logga ut överallt: alla enheter loggas ut direkt. [[admin.reset_mfa]] tar bort personens autentiseringsapp, återställningskoder och passnycklar, så att de kan ställas in på nytt. Servern kan också ställas in så att personer loggas ut automatiskt efter en stund utan aktivitet.

# Roller och behörigheter
- Tre roller är inbyggda: [[role.admin]] (allt), [[role.leader]] (skapa kanaler, moderera delat innehåll, föra in beslut, planera möten) och [[role.member]].
- På fliken [[admin.tab_roles]] kan du lägga till egna roller med [[admin.new_role]] genom att kombinera behörigheter – till exempel en sekreterare som får planera möten.
- Roller kan också användas vid delning.

# Grupper
Gemensamma grupper hanteras under [[nav.people]], på fliken [[people.groups]]: välj [[people.new_group_global]]. Det kräver behörigheten [[perm.roles.manage]]. Låt grupperna spegla hur organisationen faktiskt ser ut – sektioner, lag, styrelsen.

# Loggen
Fliken [[admin.tab_logs]] visar händelser som har betydelse för säkerheten, med tidpunkt, konto och nätverksadress: inloggningar och misslyckade inloggningar, ändringar av konton, roller och delning, säkerhetskopior och exporter. Du kan begränsa listan till en typ av händelse. Posterna sparas i två år.

# Säkerhetskopia och export
1. Välj en [[admin.passphrase]] på minst 12 tecken på fliken [[admin.tab_backup]] och skriv den två gånger.
2. Välj [[admin.backup_create]]. En krypterad fil med databasen och alla filer laddas ner.
3. Förvara filen någon annanstans än på servern, och förvara lösenfrasen säkert och skilt från filen.

- Utan lösenfrasen går säkerhetskopian inte att återställa. Den som har både filen och lösenfrasen har alla uppgifter.
- Återställning görs på servern från kommandoraden. Se serverns dokumentation.
- [[admin.export_members]] laddar ner medlemsregistret som en fil med namn, kontaktuppgifter och roller. Den innehåller personuppgifter. Med en lösenfras krypteras filen.

# Goda vanor
- Kräv tvåstegsverifiering för ledare och administratörer.
- Ha minst två administratörer.
- Dela med grupper och roller hellre än med långa listor av enskilda personer.
- Prova att återställa en säkerhetskopia någon gång, så att du vet att det fungerar.
- Kom ihåg att administratörer inte kan läsa medlemmarnas privata innehåll eller direktmeddelanden – och inte heller ska behöva det.`,
};
