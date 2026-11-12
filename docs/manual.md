# Samling user manual

This manual walks through Samling with a fictional club, *Björkens IF*, as the
example. The same guidance is available inside the application under **Help**,
in Swedish and English.

The screenshots are produced automatically from example data with
`make manual-screenshots`, so they can be refreshed whenever the interface
changes.

**Contents**

1. [Signing in](#1-signing-in)
2. [The welcome tour and finding your way](#2-the-welcome-tour-and-finding-your-way)
3. [Sharing: who can see what](#3-sharing-who-can-see-what)
4. [Chat](#4-chat)
5. [Documents](#5-documents)
6. [Lists](#6-lists)
7. [Polls and votes](#7-polls-and-votes)
8. [Calendar](#8-calendar)
9. [Meetings: agenda and minutes](#9-meetings-agenda-and-minutes)
10. [Decisions](#10-decisions)
11. [Boards](#11-boards)
12. [Search](#12-search)
13. [People and groups](#13-people-and-groups)
14. [History](#14-history)
15. [Settings](#15-settings)
16. [Keeping your account safe](#16-keeping-your-account-safe)
17. [Help](#17-help)
18. [On a phone](#18-on-a-phone)
19. [For administrators](#19-for-administrators)
20. [Pictures and the camera](#20-pictures-and-the-camera)
21. [Mentions and links](#21-mentions-and-links)

---

## 1. Signing in

Open the address your club has given you. Enter your username and password
and choose **Sign in**. The language can be switched in the top right corner
before you sign in.

![The sign-in page](manual/img/01-sign-in.jpg)

* The first time, you sign in with a temporary password from an administrator
  and are asked to choose your own. Use at least 12 characters; a sentence of
  several words works well.
* If you have turned on two-step verification you are asked for the 6-digit
  code from your authenticator app. One of your recovery codes works too.
* If you have added a passkey, choose **Sign in with a passkey** and confirm
  with your fingerprint, face or device PIN. No username or password is
  needed.
* After ten wrong attempts the account is locked for 15 minutes.

## 2. The welcome tour and finding your way

The first time you sign in, a short tour points out each part of the
application. Use **Next** and **Back**, or **Skip** to leave it. You can run it
again at any time from **Help**.

![The welcome tour](manual/img/02-welcome-tour.png)

![A tour step pointing at Chat](manual/img/02b-welcome-tour-step.png)

After the tour you land on **Home**, which gathers what needs your attention:
unread conversations, what is coming up in the calendar, polls waiting for
your vote and the latest decisions.

![Home](manual/img/03-home.png)

The menu on the left leads to every section. The **?** in the top right
corner of each page opens the help for that page.

## 3. Sharing: who can see what

One idea applies everywhere in Samling: **what you create is private until
you share it.** Documents, lists, polls, calendar events, boards, meetings
and decisions all work the same way.

Choose **Share** on an item to decide who gets access:

![The sharing dialog](manual/img/07-sharing.png)

* **Everyone** – all members.
* **Group** – a shared group such as *The board*, or one of your own private
  groups.
* **Role** – for example all leaders.
* A single **person**.

For each one you choose **Can view** or **Can edit**. A badge shows whether
an item is *Private* or *Shared*. Administrators cannot read your private
items or your direct messages.

## 4. Chat

Channels are for subjects that concern many; direct messages are private
conversations between the people in them.

![A channel with a pinned message, comments, reactions and an attachment](manual/img/04-chat.png)

* **Write** in the box at the bottom and press Enter. Shift+Enter makes a new
  line.
* **Attach files** with the paper-clip button – spreadsheets, presentations,
  documents, PDFs and so on.
* **Pin** an important message with the pin icon that appears on the message.
  **Pinned** at the top shows all pinned messages.
* Links are clickable, and links to Teams, Zoom, Meet or Jitsi are shown as a
  **Join** button.
* **Edit** or **delete** your own messages with the icons on the message.
* **Format** text with the **B**, *I* and U buttons above the message box, or
  with Ctrl+B, Ctrl+I and Ctrl+U. The smiley inserts an emoji.
* **React** to a message with the smiley on the message – a thumbs up, a
  heart, a crying face and so on. Select your reaction again to remove it.
* **Comment** on a message with the arrow on the message. Comments are shown
  indented under the message they belong to.
* **Take a picture** with the camera button; it is attached to your message
  and shown in the conversation. **Scan text with the camera** reads the text
  in a picture into the message box (see [Pictures and the camera](#20-pictures-and-the-camera)).
* **Mention** someone by typing `@` and their username, for example
  `@bertil`; a list of people appears as you type. The person gets a notice
  under **Mentions** in the menu – but only if they can read the
  conversation. Mentioning someone in a channel they have no access to, or
  in a direct message they are not part of, tells them nothing.
* **Link to something in Samling** – a poll, a board, a document, a meeting
  and so on – with the chain button. The link shows the current name of the
  thing and opens it. See [Mentions and links](#21-mentions-and-links).
* **Block direct messages** from a person with the button at the top of the
  conversation with them. They can then neither write to you nor start a new
  conversation with you, and they are not told. Channels are not affected.
* Start a private conversation with the **+** next to *Direct messages*.
* A lock on a channel means you can read but not write there, as in an
  announcements channel.

The small `@name` beside each person is their unique username. Display names
are unique too, but the username is the one that can never be confused.

## 5. Documents

The archive keeps files worth keeping, with their history.

![The document archive](manual/img/05-documents.png)

1. Choose **Upload document**, pick the file, and optionally give it a name,
   a folder (for example `Board/2026`) and a description.
2. Decide who it is shared with, then upload.

Open a document to download it, see earlier versions, or upload a new one.

![A document with two versions](manual/img/06-document.png)

* **Upload new version** keeps every earlier version; add a comment saying
  what changed.
* **Make a copy** gives you a private copy to work on.
* Each person has a storage limit set by the club. When it is full, remove
  documents you no longer need.

## 6. Lists

Three kinds of lists: to-do lists (tick things off), priority lists (sorted by
priority) and simple lists.

![A shared priority list](manual/img/08-list.png)

Add a line at the bottom, optionally with a priority and a date. Tick an item
when it is done. Shared lists update for everyone straight away.

* Choose an **owner** for a row to show who takes care of it.
* Under **Pictures and files** you can attach files or take a picture.
* **Scan text with the camera** reads a printed list from a picture; each
  line becomes a row.

## 7. Polls and votes

Ask a question and let the members vote.

![A poll with results](manual/img/09-poll.png)

Choose **New poll** and pick a voting method:

| Method | Use it for |
|--------|------------|
| One choice | "Which day suits best?" |
| Multiple choice | Picking several activities, optionally with a maximum |
| Yes / No | Formal votes, with *abstain* |
| Ranking | Ordering alternatives; higher ranks give more points |
| Points | Giving each alternative 0–5 points |

Options you can set: a closing time, whether votes may be changed, and when
the result is shown.

**Anonymous voting** stores votes without any link to the voter. An anonymous
vote cannot be changed, and the result is shown only when the poll has
closed. When a poll has closed, someone with the right to record decisions
can turn it into a decision with **Record as decision**.

## 8. Calendar

Your own events and everything shared with you, in one month view.

![The calendar](manual/img/10-calendar.png)

* Choose **New event**, or click a day. Events are private until shared.
* Add a link to an online meeting and it becomes a **Join** button.
* **Export** downloads a calendar file (.ics) for Outlook, Google Calendar or
  Apple Calendar. **Import** reads such a file into your private calendar.
* Meetings planned under *Meetings* appear here automatically for everyone
  invited.

Repeating events are not supported; add each occasion separately.

## 9. Meetings: agenda and minutes

A meeting goes through four steps, shown as tabs: **Plan → Agenda → Minutes →
Finalize**.

![The agenda of a meeting](manual/img/11-meeting-agenda.png)

1. **Plan** – time, place, an optional online link, and who is invited. The
   meeting is added to the invited people's calendars.
2. **Agenda** – add, reorder and describe the items. *Start from a standard
   agenda* gives the usual items of a board meeting.
3. **Minutes** – note who was present, write notes under each item and record
   decisions as they are taken.

   ![Writing the minutes](manual/img/12-meeting-minutes.png)

4. **Finalize** – locks the minutes. They can then be exported or printed.

## 10. Decisions

The decision log is the club's record of what has been decided. Each decision
gets a number such as `2026-001`.

![The decision log](manual/img/13-decisions.png)

A decision is never overwritten or deleted. **Revise** saves the change
together with your reason, and the earlier wording stays in the history.

![A decision with its history](manual/img/14-decision.png)

The **History** tab shows decisions and concluded polls on one timeline, and
**Export (CSV)** downloads the log for a spreadsheet.

## 11. Boards

A board shows work moving from idea to done.

![A board](manual/img/15-board.png)

Add cards, give them an owner and a date, and drag them between columns. The
arrow buttons under each card do the same without dragging, which is handy on
a phone or with a keyboard.

* Give a card a **colour mark** when you open it, for example red for urgent.
  It is shown as a stripe on the card.
* The owner of the board can choose the **colour of the board**, which makes
  it easy to recognise in the list of boards.
* An opened card can have pictures and files attached.

## 12. Search

One search covers everything you have access to – and nothing else.

![Search with wildcards](manual/img/16-search.png)

**Advanced search** lets you:

* match **wildcards** (`*` any text, `?` one character) or a **regular
  expression**;
* limit the search to a **period**;
* choose **where** to search: messages, documents, lists, polls, the
  calendar, meetings, decisions, boards, cards or people.

Search looks at names, titles and text, not inside uploaded files.

## 13. People and groups

**People** lists the members with their profile picture and the contact
details and other information they have chosen to show. **Message** starts a
direct conversation; the crossed circle beside it blocks direct messages from
that person.

![The member list](manual/img/17-people.png)

Groups make sharing quick. *Shared groups* are created by administrators and
can be used by everyone. *My own groups* are visible only to you.

![Groups](manual/img/18-groups.png)

## 14. History

**History** shows what you yourself have done, day by day. Only you can see
it.

![Personal history](manual/img/19-history.png)

## 15. Settings

![Appearance settings](manual/img/20-settings-appearance.png)

* **Language** – Swedish or English.
* **Theme** – Light, Dark, Spring, Autumn, Sand or High contrast.
* **Text size** – four sizes.
* **Accessibility** – reduce motion, underline links, easy-to-read text and a
  stronger keyboard focus marker.
* **Profile** – your name and contact details, and whether other members may
  see them.
* **Profile picture** – upload a photo, take one with the camera, or pick one
  of the ready-made pictures.
* **More about me** – optional address, interests, specialities and a short
  presentation. Each field has its own **Show to other members** box; a field
  is shown to others only when its box is ticked.
* **Blocked people** – the people whose direct messages you have blocked.
* **My data** – download a copy of the personal data stored about you.

![Profile picture and optional profile fields](manual/img/33-profile.png)

The dark theme, as an example:

![The dark theme](manual/img/29-dark-theme.png)

## 16. Keeping your account safe

Everything about signing in is under **Settings → Security**.

![Security settings](manual/img/21-settings-security.png)

**Two-step verification.** Enter your password, choose **Set up two-step
verification**, scan the code with an authenticator app and enter the code
it shows. You then get ten **recovery codes**. Each works once in place of
the app's code, for instance if you lose your phone. They are shown only
once: save or print them. **Create new recovery codes** replaces them.

**Passkeys.** A passkey lets you sign in with your fingerprint, face or
device PIN. Give it a name, enter your password and choose **Add a passkey**.
Passkeys need a secure (https) connection.

**Signed-in devices.** **Sign out everywhere else** signs you out on all
other devices – do this if a phone or computer is lost. Your club may also
have set an automatic sign-out after a period without activity.

**Password.** Changing it signs out all your other devices.

Changes to how you sign in always ask for your current password.

## 17. Help

**Help** in the menu has a topic for every part of Samling, a search box, and
the button that restarts the welcome tour.

![The help page](manual/img/22-help.png)

![A help topic](manual/img/23-help-topic.png)

## 18. On a phone

Samling adapts to the screen. On a phone the menu opens from the button in
the top left corner.

| Chat | Menu | Calendar |
|------|------|----------|
| ![Chat on a phone](manual/img/30-phone-chat.png) | ![The menu on a phone](manual/img/31-phone-menu.png) | ![The calendar on a phone](manual/img/32-phone-calendar.png) |

You can add Samling to the home screen: in the browser's menu, choose *Add to
Home Screen* (iPhone, Safari) or *Install app* (Android, Chrome).

## 19. For administrators

**Administration** appears in the menu for people with administrative
permissions.

![The list of accounts](manual/img/24-admin-accounts.png)

**Accounts.** Create an account with **New account** and give the person the
temporary password in a safe way. Open an account to change it:

![An account](manual/img/25-admin-account.png)

* rename it, set roles, or disable it;
* **Require two-step verification** – the person must set it up, with an app
  or a passkey, before the account can be used;
* **Set password** – a new temporary password, which also signs the person
  out everywhere;
* **Sign out everywhere** – ends all the person's sessions at once, for
  example when a device is lost;
* **Reset two-step verification** – removes the authenticator app, recovery
  codes and passkeys;
* **Unlock** after too many failed sign-ins;
* **Delete account** – erases personal data and private content. Shared
  posts and documents remain, shown as written by a deleted user. Decisions
  are always kept.

**Roles** decide what people may do, and can also be used when sharing.

![Roles](manual/img/26-admin-roles.png)

**Log** shows security-related events: sign-ins, account and role changes,
changed sharing, backups and exports.

![The log](manual/img/27-admin-log.png)

**Backup & export** downloads a complete, encrypted backup. Keep the
passphrase safe; without it the backup cannot be restored. Restoring is done
on the server, see [deployment.md](deployment.md).

![Backup and export](manual/img/28-admin-backup.png)

Installation, configuration and server administration are described in
[deployment.md](deployment.md); the security design in
[security.md](security.md).

## 20. Pictures and the camera

Wherever there is a camera button you can take a picture straight into
Samling: in the chat, as a document (or a new version of one), on lists,
calendar events, cards, meetings, polls and decisions (under **Pictures and
files**), and as your profile picture. The first time, the browser asks
whether Samling may use the camera. Without a camera you can choose a picture
you already have.

**Scan text with the camera** reads printed text in a picture so that you do
not have to type it:

1. Choose **Scan text with the camera** and take a picture of the text –
   straight on, close and in good light.
2. Wait while the text is read (up to a minute the first time).
3. Check and correct the text, then choose **Use the text**.

![Text read from a picture of a note](manual/img/34-scan-text.png)

The text goes into the message box in the chat, becomes rows in a list, or is
added to the description of a document, event, card, poll or decision.

* The text is read on your own device; the picture is stored only if you
  choose to attach it.
* Printed text in Swedish and English works best. Handwriting is seldom read
  correctly.
* Who can see an attached picture follows the sharing of the thing it is
  attached to.
* The camera needs a secure (https) connection.

## 21. Mentions and links

**Mentions.** Write `@` followed by a username in a chat message or a
comment. The person's name is highlighted, and they see a number beside
**Mentions** in the menu. The Mentions page lists the messages; selecting
one opens the conversation.

![The Mentions page](manual/img/35-mentions.png)

* Only people who can read the conversation are told. A mention never gives
  anyone access, and never reveals a message to someone who cannot read it.
* If the person later loses access to the channel, or the message is
  deleted, the mention disappears from their list.

**Links inside Samling.** The chain button (**Link to something in
Samling**) is found above the message box in the chat and under the text
fields of meetings (description, background and notes of each agenda item),
calendar events, cards, polls, decisions and documents. Pick what to link
to; a short code such as `[[poll:…]]` is put in the text and is shown as a
link with the current name of the poll. Pasting the address of a page in
Samling works the same way.

* In fields that you edit as text, such as meeting notes, the links are
  listed under the field as **Links in the text**.
* A link shows the name only to readers who have access to the thing it
  points to. Others see just that there is a link they cannot open.
* A link does not share anything; share the item itself to let others in.
