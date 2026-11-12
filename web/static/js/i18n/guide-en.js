// English texts of the welcome tour and the online help. Keys must match guide-sv.js.
//
// The help bodies use a small line format that views/help.js parses:
//   "# " sub-heading, "1. " numbered step, "- " bullet, "> " note, blank line = new paragraph,
//   [[some.key]] inserts the label of a button or menu from the dictionary (so the
//   help always quotes the real label), **text** is emphasis.
export default {
  'nav.help': 'Help',

  // ---------- welcome tour ----------
  'tour.step': 'Step {n} of {total}', 'tour.skip': 'Skip', 'tour.back': 'Back', 'tour.next': 'Next', 'tour.done': 'Done',
  'tour.welcome.title': 'Welcome to Samling',
  'tour.welcome.text': 'Samling is your club\'s own place to talk, keep documents, plan and decide together. This short tour shows where things are. It takes about a minute.',
  'tour.nav.title': 'The menu',
  'tour.nav.text': 'The menu on the left takes you to every section. Home gives you an overview of what is new.',
  'tour.nav.text_narrow': 'The menu button in the top left corner opens the menu with every section. Home gives you an overview of what is new.',
  'tour.chat.title': 'Chat',
  'tour.chat.text': 'Talk in channels that many can follow, or send direct messages that only the recipients can read. You can pin important messages and attach files.',
  'tour.documents.title': 'Documents',
  'tour.documents.text': 'Keep the club\'s files in one place, sorted into folders. When you upload a new version, the earlier ones are kept.',
  'tour.sharing.title': 'Private until you share',
  'tour.sharing.text': 'Everything you create is private at first. You choose who gets access – everyone, a group, a role or a single person – and whether they can only view or also edit.',
  'tour.lists.title': 'Lists, boards and polls',
  'tour.lists.text': 'Tick off tasks in a list, follow work on a board with cards, and let the members vote in a poll.',
  'tour.calendar.title': 'Calendar, meetings and decisions',
  'tour.calendar.text': 'The calendar shows what is coming up. Meetings have an agenda and minutes, and what is decided is kept in the decision log.',
  'tour.search.title': 'Search',
  'tour.search.text': 'One search finds messages, documents, people and more – but only things you have access to.',
  'tour.settings.title': 'Settings',
  'tour.settings.text': 'Choose language, theme and text size, and turn on the accessibility options you need. Protect your account with two-step verification.',
  'tour.help.title': 'Help is always close',
  'tour.help.text': 'Help explains every section step by step, and the question mark at the top of a page opens the help for that page. You can watch this tour again from Help.',

  // ---------- help: interface ----------
  'help.intro': 'Here you can read how Samling works, one section at a time. Search for a word, or pick a topic below.',
  'help.tour_again': 'Show the welcome tour again', 'help.search': 'Search the help', 'help.none': 'No help topic matches your search.',
  'help.count': '{n} topics found', 'help.open': 'Open {name}', 'help.related': 'Related topics', 'help.all': 'All help topics',
  'help.group.basics': 'Start here', 'help.group.sections': 'The sections', 'help.group.more': 'More', 'help.group.admin': 'Administration',

  // ---------- help: topics ----------
  'help.start.title': 'Getting started',
  'help.start.intro': 'Sign in, find your way around and make a few good first choices.',
  'help.start.body': `# Signing in
1. Open the address your administrator gave you.
2. Enter your username and password and choose [[login.submit]].
3. The first time, and after a password reset, you are asked to choose a new password of your own.
4. If two-step verification is on for your account, enter the 6-digit code from your authenticator app.

> After 10 wrong passwords the account is locked for 15 minutes. An administrator can unlock it or give you a temporary password.

# Finding your way
- The menu leads to every section. On a phone you open it with the [[nav.menu]] button in the top left corner.
- [[nav.home]] sums up what is waiting for you: unread conversations, coming events, polls to vote in and the latest decisions.
- The question mark at the top right of a page opens the help for that page.
- To sign out, use the [[nav.logout]] button beside your name at the bottom of the menu.

# Three things worth doing first
1. Open [[nav.settings]] and choose language, theme and text size.
2. Check your name and contact details under [[settings.profile]], and decide whether other members may see them.
3. Turn on [[settings.mfa]] to protect your account.

# Good to know
- What you create is private until you share it. The topic Sharing and access explains how.
- New messages and most other changes show up by themselves. You rarely need to reload the page.
- Samling works in the browser on computers, tablets and phones, and can be put on the home screen of a phone.`,

  'help.sharing.title': 'Sharing and access',
  'help.sharing.intro': 'Everything you create is private until you share it – and sharing works the same way everywhere.',
  'help.sharing.body': `Lists, documents, polls, calendar events, boards, meetings, decisions and channels are all shared in the same way. Items are marked [[share.private]] or [[share.shared]] so that you can see the difference at a glance.

# Who you can share with
- [[share.everyone]] – all members.
- A group – one of the club's shared groups, or one of your own private groups.
- A role – everyone who holds the role, for example Leader.
- A person – a single member.

# View or edit
- [[share.read]]: the person can see the item. In a poll it also means that they can vote.
- [[share.write]]: the person can also contribute – write in a channel, add rows to a list, upload a new version of a document, move cards on a board.

# Share when you create something
1. Find [[share.who]] in the form. (For a meeting the field is called [[meet.invited]], for a poll [[polls.who]] and for a decision [[dec.audience]].)
2. Pick someone in the list [[share.add]]. Repeat to add more.
3. Choose [[share.read]] or [[share.write]] on each row.
4. Leave it empty if you want to keep the item to yourself.

# Change the sharing later
1. Open the item and choose [[share.button]].
2. Add rows, change the access level, or remove a row with the cross.
3. Choose [[common.save]]. If you remove every row, the item is private again.

# Good to know
- Only the owner can change the sharing or delete the item. For things that are shared, a moderator (normally a leader) can do it too.
- Administrators cannot see your private items or your direct messages.
- New channels and polls suggest [[share.everyone]] from the start. Everything else starts out private.
- Do you often share with the same people? Make a group of your own under [[nav.people]].
- Polls and decisions have no access level: those you share a poll with can see it and vote, and those you share a decision with can read it.`,

  'help.home.title': 'Home',
  'help.home.intro': 'The overview you see when you sign in: what is new and what is waiting for you.',
  'help.home.body': `# What the page shows
- [[home.unread]] – channels and direct messages with new messages. Select one to open it.
- [[home.upcoming]] – events and meetings in the coming 30 days.
- [[home.to_vote]] – open polls that you have not voted in yet.
- [[home.decisions]] – the most recently recorded decisions.

# Tips
- [[home.all]] in a box opens the whole section.
- The page updates by itself when something happens.
- You only see things that you have access to, so two members can have different Home pages.`,

  'help.chat.title': 'Chat',
  'help.chat.intro': 'Channels for subjects that concern many, and direct messages for private conversations.',
  'help.chat.body': `# Read and write
1. Open [[nav.chat]] and select a channel under [[chat.channels]] or a conversation under [[chat.dms]].
2. Write in the field at the bottom and press Enter or choose [[chat.send]]. Shift+Enter starts a new line.
3. A number beside a name means unread messages. New messages appear by themselves.

# Start a direct message
1. Select the plus sign beside [[chat.dms]] ([[chat.new_dm]]).
2. Tick one or more recipients – up to 19 – and choose [[chat.start]].

Only the participants can read a direct message. Not even an administrator can. You can also choose [[people.message]] beside a name under [[nav.people]].

# Attach files
1. Select the paper clip ([[chat.attach]]) and pick one or more files, at most 10 per message.
2. Send the message. Everyone who can read the conversation can download a file by selecting it.

# Pin, edit and delete
- Point at a message to see its buttons. On a phone they are always visible.
- The pin ([[chat.pin]]) keeps something important easy to find. [[chat.pins]] at the top of the conversation lists everything that is pinned. Anyone who may write in the conversation may pin.
- The pencil and the bin let you edit and delete your own messages. An edited message is marked as such.

# Channels
- Creating a channel requires the permission [[perm.channels.create]], which leaders normally have. Use the plus sign beside [[chat.channels]] ([[chat.new_channel]]).
- The sharing of a channel decides who can read and who can write. A channel that everyone can read but only a few can write in works as a notice board. In such a channel you see a padlock and the text [[chat.readonly]].
- Links to video meetings in Teams, Zoom, Google Meet and similar services are shown as a button to join.`,

  'help.documents.title': 'Documents',
  'help.documents.intro': 'The archive for files worth keeping: statutes, minutes, forms and the like.',
  'help.documents.body': `# Upload a document
1. Choose [[docs.upload]].
2. Pick the file. Give it a name if you want something other than the file name.
3. If you like, type a folder such as Board/2026 (a slash makes a subfolder) and a description.
4. Under [[share.who]], choose who gets access – or leave it empty to keep the document private.
5. Choose [[docs.upload]].

# Find and open
- Limit the list to one folder with the folder list at the top, or type part of the name in [[common.filter]].
- Select a document and choose [[common.download]]. The file is saved on your device and opens in your own program. Documents are never opened inside Samling.

# Upload a new version
1. Open the document.
2. Describe what has changed in the comment field, if you like.
3. Choose [[docs.new_version]] and pick the file.

Every earlier version stays in the list under [[docs.versions]] and can still be downloaded. You need [[share.write]] to upload a new version.

# More
- [[docs.copy]] gives you a private copy of the current version.
- [[common.edit]] changes the name, folder and description.
- The owner can change the sharing with [[share.button]] and remove the document with [[common.delete]].
- Search finds documents by name, folder and description, not by what the file contains. Give documents names that say what they are.
- Your administrator decides how large a file may be.`,

  'help.lists.title': 'Lists',
  'help.lists.intro': 'To-do lists, priority lists and simple lists – your own or shared with others.',
  'help.lists.body': `# Three types
- [[lists.kind_todo]] – rows that you tick off, with a due date if you like.
- [[lists.kind_priority]] – as above, and each row also has a priority. The most important rows are shown first.
- [[lists.kind_plain]] – just rows, for example a packing list.

# Create a list
1. Choose [[lists.new]].
2. Give it a title and choose the [[lists.kind]]. The type is chosen once, when the list is created.
3. Choose who to share it with, or leave it private, and choose [[common.create]].

# Use a list
1. Type in the field [[lists.add_placeholder]], choose a priority and a date if the list has them, and choose [[common.add]].
2. Tick the box when something is done.
3. Move a row with the arrows and remove it with the bin. In a priority list the order follows the priority instead.

# Good to know
- A date that has passed is shown in red.
- When someone else changes a shared list, you see it at once.
- With [[share.read]] you can see the list but not change it. With [[share.write]] you can add, tick off and remove rows.`,

  'help.polls.title': 'Polls',
  'help.polls.intro': 'Ask a question and let the members vote – from "which day suits best?" to a formal yes or no.',
  'help.polls.body': `# Vote
1. Open the poll from [[nav.polls]], or from [[home.to_vote]] on the Home page.
2. Make your choice and choose [[polls.vote]].
3. If the poll allows it, you can choose [[polls.change]] until the poll closes.

# Create a poll
1. Choose [[polls.new]] and write the [[polls.question]].
2. Choose a [[polls.kind]] and fill in the options, between 2 and 30.
3. Choose when the result is shown ([[polls.results]]) and, if you like, a closing time ([[polls.closes]]).
4. Under [[polls.who]], choose who can see the poll and vote. Then choose [[common.create]].

# Voting methods
- [[polls.kind_single]] – everyone picks exactly one option.
- [[polls.kind_multi]] – everyone may pick several, with an upper limit if you set one.
- [[polls.kind_yesno]] – yes, no or abstain. Suited to formal votes.
- [[polls.kind_ranked]] – everyone puts the options in order. A higher place gives more points.
- [[polls.kind_score]] – everyone gives each option 0 to 5 points.

# Good to know
- With [[polls.anonymous]] the votes are stored without any link to the voter. Nobody, not even an administrator, can see who voted for what. An anonymous vote cannot be changed afterwards.
- The owner can end the poll early with [[polls.close]]. The result is kept.
- A member who may record decisions can turn a closed poll into a decision with [[polls.to_decision]].`,

  'help.calendar.title': 'Calendar',
  'help.calendar.intro': 'Your own events, events that others have shared with you, and the meetings you are invited to.',
  'help.calendar.body': `# Add an event
1. Choose [[cal.new]], or select the date number of a day in the month view.
2. Fill in the title, start and end. Tick [[cal.all_day]] for an event without times.
3. Add a location, a link (for example to a video meeting) and a description if you like.
4. Choose who to share with under [[share.who]], or leave it empty for a private event. Choose [[common.save]].

# Look, change and remove
- The arrows change month and [[cal.today]] takes you back.
- Select an event to see the details. From there you can use [[common.edit]], [[share.button]] and [[common.delete]] on your own events.
- [[cal.mail]] opens your e-mail program with the details filled in, and the .ics button downloads that single event.
- [[cal.upcoming]] below the month lists what comes next.
- Meetings planned in Samling appear by themselves. You change them from the meeting: choose [[cal.open_meeting]].

# Other calendars
- [[cal.export]] downloads the events you can see as an .ics file that Outlook, Google Calendar, Apple Calendar and others can read. It is a copy as of now and does not update by itself.
- [[cal.import]] adds the events of an .ics file to your private calendar. If you import the same file again, the events are updated instead of duplicated.

> Samling has no repeating events. Of a repeating event in an imported file, only the first occasion is brought in.`,

  'help.meetings.title': 'Meetings',
  'help.meetings.intro': 'From notice to finalized minutes in four steps: plan, agenda, minutes, finalize.',
  'help.meetings.body': `Everyone who is invited can follow the meeting, its agenda and its minutes. Planning a meeting requires the permission [[perm.meetings.create]], which leaders normally have.

# 1. Plan
1. Choose [[meet.new]] and fill in the title, time and a location or a link.
2. Leave [[meet.use_template]] ticked to get the usual items to start from.
3. Under [[meet.invited]], choose who the meeting is for. With [[share.write]] a person can help write the minutes.
4. Choose [[common.create]].

The meeting is put in the calendar of everyone invited and follows the changes you make.

# 2. Agenda
1. Open the meeting and go to the step [[meet.step_agenda]].
2. Change the items: title, [[meet.presenter]], time in minutes and [[meet.background]]. The arrows change the order and [[meet.add_item]] adds an item.
3. Choose [[common.save]]. The planned time in total is shown below the items.

# 3. Minutes
1. Go to the step [[meet.step_protocol]] and fill in [[meet.chair]], [[meet.secretary]] and [[meet.attendees]], one name per line.
2. Write notes under each item.
3. When the meeting decides something, choose [[dec.record]] under the item. The decision goes straight into the decision log.
4. Choose [[common.save]] every now and then. Nothing is saved until you do.

# 4. Finalize
1. Go to the step [[meet.step_finalize]] and choose [[meet.finalize]]. The minutes are then locked.
2. [[meet.export_md]] downloads the minutes as a text file, and [[meet.print]] prints them or saves them as a PDF through the browser.

> If a correction is needed, the owner of the meeting can choose [[meet.reopen]]. Both finalizing and reopening are noted in the log.`,

  'help.decisions.title': 'Decisions',
  'help.decisions.intro': 'The decision log: what has been decided, when, and how it has changed since.',
  'help.decisions.body': `# Read decisions
- [[dec.log]] lists the decisions you may see. Each has a number within its year, such as 2026-001. Limit the list by year or by typing in [[common.filter]].
- [[dec.timeline]] puts decisions and closed polls on one timeline.
- Open a decision to read the whole text, see whether it is [[dec.status_decided]], [[dec.status_revoked]] or [[dec.status_superseded]], and follow its history. If it came from a meeting or a poll there is a link to it.
- [[dec.export]] downloads the log as a file for a spreadsheet program.

# Record a decision
This requires the permission [[perm.decisions.create]], which leaders normally have.
1. Choose [[dec.record]].
2. Write a short title, the [[dec.text]] and the [[dec.date]].
3. Under [[dec.audience]], choose who may read the decision.
4. Choose [[dec.record]].

You can also record a decision straight from an item in the minutes of a meeting, or from a closed poll with [[polls.to_decision]].

# Revise a decision
1. Open the decision and choose [[dec.revise]].
2. Change the text or the status, and state the [[dec.reason]]. A reason is required.
3. Choose [[common.save]].

> A decision is never deleted or overwritten. Every change, including revoking it, is saved in the history of the decision together with the earlier text.`,

  'help.boards.title': 'Boards',
  'help.boards.intro': 'Follow work from idea to done with cards that move between columns.',
  'help.boards.body': `# Create a board
1. Choose [[boards.new]] and give the board a title.
2. Type the columns with commas between them, or keep the suggestion. A board can have 1 to 12 columns.
3. Choose who to share with, or keep the board private, and choose [[common.create]].

# Work with cards
1. Choose [[boards.new_card]] under a column. Give the card a title and, if you like, a description, [[boards.assignee]] and [[lists.due]].
2. Move a card by dragging it to another column, or use the arrows under the card. The arrows work with the keyboard and on a phone too.
3. Select a card to open it, change it or remove it.

# Columns
The owner of the board looks after the columns:
- [[boards.add_column]] adds a column.
- The pencil renames a column and the arrows move it left or right.
- An empty column can be removed with the bin. Move the cards away first.

# Good to know
- With [[share.write]] you can add, change and move cards. With [[share.read]] you can only look.
- A date that has passed is shown in red on the card.`,

  'help.search.title': 'Search',
  'help.search.intro': 'One search for everything you have access to: messages, documents, lists, polls, the calendar, meetings, decisions, boards and people.',
  'help.search.body': `# Search
1. Open [[nav.search]] in the menu. On a phone there is also a magnifying glass at the top.
2. Type what you are looking for and choose [[search.go]].
3. The hits are grouped by kind. Select a hit to open it.

# Narrow it down
Choose [[search.advanced]] to get more choices:
- [[search.mode]]: [[search.mode_plain]] finds the text exactly as you wrote it. With [[search.mode_wildcard]] a star stands for any text and a question mark for one character, so budget 202? finds budget 2025 and budget 2026. [[search.mode_regex]] is for those who know such expressions.
- [[search.from]] and [[search.to]] limit the search to a period.
- [[search.where]] limits the search to certain kinds of content. With nothing ticked, everything is searched.
- [[search.case]] makes the search tell capital letters from small ones. Normally it does not.

# Good to know
- You only get hits from things you have access to.
- The contents of uploaded files are not searched – only their names, folders and descriptions.
- Contact details of other members can only be found if they have chosen to show them.`,

  'help.people.title': 'People',
  'help.people.intro': 'The member directory, and the groups that make sharing quick.',
  'help.people.body': `# Find a member
- The tab [[people.directory]] lists all members with name, username and role. Type in the search field to find someone.
- E-mail and phone are shown for members who have chosen to show them. Otherwise it says [[people.no_contact]]. Select an e-mail address to write a message in your e-mail program.
- [[people.message]] starts a direct message in the chat.

# Groups
The tab [[people.groups]] has two kinds:
- [[people.global_groups]] are created by the administrators – sections, teams, the board. Everyone can see them and use them when sharing.
- [[people.local_groups]] are yours alone. Use them to share quickly with the same people.

# Make a group of your own
1. Go to [[people.groups]] and choose [[people.new_group_local]].
2. Give the group a name, tick its members and choose [[common.save]].
3. When you share something, the group is now in the list as [[share.my_group]].

# Good to know
- Nobody else can see your own groups. When you share with one, others only see that the item is shared with [[share.hidden_group]].
- Change or remove a group with the pencil on the group.`,

  'help.history.title': 'History',
  'help.history.intro': 'Your own log of what you have done in Samling. Only you can see it.',
  'help.history.body': `# What you find here
- What you have created, changed, voted in and downloaded, day by day, with the time.
- Many rows are links that take you back to the thing in question.

# Use it
- [[history.show]] limits the list to one kind of content.
- [[history.more]] at the bottom fetches earlier activity.

# Good to know
- History is handy when you want to find something you worked with recently.
- Nobody else can see your history, not even an administrator. Security events, such as sign-ins and changed sharing, are also noted in a separate log that only those with the right permission can read.
- The history is included when you download your data under [[nav.settings]].`,

  'help.settings.title': 'Settings',
  'help.settings.intro': 'Language, look, your profile and the security of your account.',
  'help.settings.body': `# Appearance
- [[settings.language]]: Swedish or English. You can switch at any time.
- [[settings.theme]]: six themes, among them [[theme.dark]] and [[theme.contrast]].
- [[settings.text_size]]: four sizes, from [[size.s]] to [[size.xl]].
- [[settings.a11y]]: see the topic Accessibility.

Your choices take effect at once and follow your account to other devices.

# Profile
1. Change your [[people.name]], [[people.email]] and [[people.phone]] under [[settings.profile]].
2. Tick or untick [[settings.contact_visible]]. When it is unticked, only account administrators can see your e-mail and phone number.
3. Choose [[common.save]].

Only an administrator can change your username.

# Two-step verification
With two-step verification you sign in with your password and a code from your phone, so a stolen password is not enough.
1. Install an authenticator app on your phone, for example Google Authenticator or Microsoft Authenticator.
2. Choose [[settings.mfa_start]] and scan the QR code with the app, or type in the key by hand.
3. Enter the 6-digit code that the app shows and choose [[settings.mfa_confirm]].

To turn it off, type your password and choose [[settings.mfa_disable]] – unless an administrator requires it for your account. If you lose your phone, an administrator can reset it for you.

> When you turn on two-step verification you get ten recovery codes. Each works once in place of the code from your app, for example if you lose your phone. Save them somewhere safe, apart from your phone; [[settings.recovery_new]] replaces them with a fresh set.

# Passkeys

A passkey lets you sign in with your fingerprint, face or device PIN, without typing a password or a code.

1. Under [[settings.passkeys]], give the passkey a name, enter your password and choose [[settings.passkey_add]].
2. Follow your device's instructions.
3. Next time, choose [[login.passkey]] on the sign-in page.

# Signed-in devices

[[settings.sessions_end]] signs you out on every other phone, tablet and computer, which is the thing to do if a device is lost. If your club has set a time limit, you are also signed out automatically after a period without activity.

# Password
- Under [[settings.change_password]] you give your current password and the new one twice. A sentence of several words is both strong and easy to remember.
- Changing the password signs you out on your other devices.
- For your protection you may be signed out automatically when you have not used Samling for a while.

# My data
[[settings.export]] downloads a copy of the personal data stored about you. Read more in the topic Privacy and your data.`,

  'help.privacy.title': 'Privacy and your data',
  'help.privacy.intro': 'What others can see about you, what stays private, and how to get a copy of your data.',
  'help.privacy.body': `# What others see
- Your name, username and roles are shown to all members in the directory.
- Your e-mail and phone number are shown to other members only if you have ticked [[settings.contact_visible]] under [[nav.settings]]. Otherwise only account administrators see them.
- What you write or upload in a shared place can be seen by those it is shared with.

# What stays private
- Things you have not shared can only be seen by you. Administrators cannot see them.
- Direct messages can only be read by the participants.
- Your [[nav.history]] is shown only to you.
- In an anonymous poll, nobody can see how you voted.

# The security log
Events that matter for security – sign-ins, changes to accounts and roles, changed sharing, backups and exports – are noted with the time, the account and the network address. The log can be read only by those who have been given that permission, and entries are kept for two years.

# Get a copy of your data
1. Open [[nav.settings]] and find [[settings.privacy]].
2. Choose [[settings.export]]. You get a file with your profile, your activity and your own messages.

# Leave Samling
Ask an administrator to delete your account. Your personal data and your private content are then erased. What you have contributed in shared places remains, shown as written by [[common.deleted_user]].

> Samling is installed and run by your own organisation. Ask your administrator if you want to know where the server is, or who is responsible for the personal data.`,

  'help.accessibility.title': 'Accessibility',
  'help.accessibility.intro': 'Samling is built to work with a keyboard, a screen reader, enlarged text and small screens – and you can adapt it further.',
  'help.accessibility.body': `# Adapt the look
Open [[nav.settings]]. Under [[settings.appearance]] you find:
- [[settings.text_size]] – up to [[size.xl]]. The zoom of your browser works as well.
- [[settings.theme]] – [[theme.contrast]] gives white and yellow on black. [[theme.dark]] is gentler in the evening.
- [[a11y.motion]] – turns off animations and transitions.
- [[a11y.underline]] – so that links do not rely on colour alone.
- [[a11y.readable]] – a wider typeface with more space between letters, words and lines.
- [[a11y.focus]] – a thicker frame around the thing the keyboard is on.

The choices are saved with your account and apply on every device where you sign in.

# Keyboard and screen reader
- Everything can be done with the keyboard. See the topic Keyboard tips.
- Each page starts with the link [[nav.skip]], which jumps past the menu.
- Pages have headings, fields have labels, and messages such as "saved" are announced by screen readers.
- Cards on a board and rows in a list can be moved with buttons, so that you do not have to drag.

# If something does not work
Tell your administrator what you were trying to do and which aid you use. That is how accessibility problems get found and fixed.`,

  'help.install.title': 'Install as an app on your phone',
  'help.install.intro': 'Put Samling on the home screen so that it opens like an app – no app store needed.',
  'help.install.body': `# iPhone and iPad
1. Open Samling in Safari and sign in.
2. Tap the share button, the square with an arrow pointing up.
3. Choose "Add to Home Screen" and then "Add".

# Android
1. Open Samling in Chrome and sign in.
2. Tap the menu with three dots.
3. Choose "Install app" or "Add to Home screen".

# Computer
In Chrome and Edge you can install Samling as a window of its own: look for "Install" in the address bar or in the menu of the browser.

# Good to know
- The app is the same Samling as in the browser, with the same account and the same content.
- You need an internet connection. Nothing is stored on the phone for reading offline.
- There are no notifications. Open the app to see what is new – the Home page and the number beside Chat show what you have not read.
- The names of the menu choices vary a little between versions of iOS, Android and the browsers.
- You remove the app like any other app. Nothing in Samling is deleted when you do.`,

  'help.keyboard.title': 'Keyboard tips',
  'help.keyboard.intro': 'Samling can be used entirely without a mouse. These are the keys worth knowing.',
  'help.keyboard.body': `# Everywhere
- **Tab** moves to the next link, button or field, and **Shift+Tab** moves back.
- **Enter** follows a link or presses a button. **Space** presses a button and ticks a box.
- The first press of Tab on a page shows the link [[nav.skip]]. Press Enter to jump past the menu.
- **Escape** closes a dialog. While a dialog is open, Tab stays inside it.
- In a list of choices, use the **arrow keys** to choose.

# In the chat
- **Enter** sends the message.
- **Shift+Enter** starts a new line.

# Lists and boards
- Rows and cards have arrow buttons for moving. Go to a button with Tab and press Enter.
- On a board, select a card with Enter to open it.

# The calendar
- Every day has a button with its date number that adds an event on that day.
- Every event is a button that opens the event.

# The welcome tour
- **Enter** takes you to the next step, **arrow left** and **arrow right** move between steps, and **Escape** skips the tour.

# Tips
- [[a11y.focus]] under [[nav.settings]] makes it easier to see where the keyboard is.
- The shortcuts of your browser work as usual: Ctrl and plus or minus changes the zoom (Cmd on a Mac), and Ctrl+F searches the page.`,

  'help.admin.title': 'For administrators',
  'help.admin.intro': 'Accounts, roles and permissions, groups, the log and backups.',
  'help.admin.body': `[[nav.admin]] is shown in the menu for those who have at least one administrative permission. You only see the tabs that your permissions cover.

# Accounts
1. On the tab [[admin.tab_accounts]], choose [[admin.new_user]].
2. Fill in a username, a name and the roles. Choose [[admin.generate]] to get a [[admin.temp_password]].
3. Give the password to the person in a safe way. They must choose a password of their own the first time they sign in.

Choose [[common.edit]] on an account to:
- change the username, name and roles
- tick [[admin.mfa_required]], so that the person must set up two-step verification before using the account
- tick [[admin.disabled]], which stops the person from signing in without deleting anything
- give a new temporary password with [[admin.set_password]], which also signs the person out everywhere
- use [[admin.reset_mfa]] when someone has lost their phone, or [[admin.unlock]] after too many wrong passwords
- choose [[admin.delete_user]]. This erases the person's personal data and private content and cannot be undone. What they contributed in shared places remains.

> There must always be at least one administrator.

If someone loses a phone or computer, open their account and choose the sign-out-everywhere button: every device is signed out at once. [[admin.reset_mfa]] removes the person's authenticator app, recovery codes and passkeys, so they can set them up again. The server can also be set to sign people out automatically after a period without activity.

# Roles and permissions
- Three roles are built in: [[role.admin]] (everything), [[role.leader]] (create channels, moderate shared content, record decisions, plan meetings) and [[role.member]].
- On the tab [[admin.tab_roles]] you can add roles of your own with [[admin.new_role]] by combining permissions – for example a secretary who may plan meetings.
- Roles can also be used when sharing.

# Groups
Shared groups are managed under [[nav.people]], on the tab [[people.groups]]: choose [[people.new_group_global]]. This requires the permission [[perm.roles.manage]]. Let the groups mirror how the organisation really looks – sections, teams, the board.

# The log
The tab [[admin.tab_logs]] shows events that matter for security, with the time, the account and the network address: sign-ins and failed sign-ins, changes to accounts, roles and sharing, backups and exports. You can limit the list to one kind of event. Entries are kept for two years.

# Backup and export
1. On the tab [[admin.tab_backup]], choose a [[admin.passphrase]] of at least 12 characters and type it twice.
2. Choose [[admin.backup_create]]. An encrypted file with the database and all files is downloaded.
3. Keep the file somewhere other than on the server, and keep the passphrase safe and apart from the file.

- Without the passphrase the backup cannot be restored. Whoever has both the file and the passphrase has all the data.
- Restoring is done on the server from the command line. See the documentation for the server.
- [[admin.export_members]] downloads the member register as a file with names, contact details and roles. It contains personal data. With a passphrase the file is encrypted.

# Good practice
- Require two-step verification for leaders and administrators.
- Have at least two administrators.
- Share with groups and roles rather than with long lists of individuals.
- Try restoring a backup once, so that you know it works.
- Remember that administrators cannot read the private content or the direct messages of members – and should not need to.`,
};
