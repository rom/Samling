# User guide

Samling runs in the browser on computers, tablets and phones. On a phone the
navigation collapses into a menu, and the site can be added to the home
screen (see [mobile.md](mobile.md)).

## Signing in

Open the address your administrator gave you and sign in with your user name
and password.

* **First sign-in / after a reset**: you must choose a new password
  (at least 12 characters unless your organisation changed the minimum).
* **Two-factor authentication**: if it is enabled for your account you are
  asked for the 6-digit code from your authenticator app. If your
  administrator requires it, you are guided through the set-up before you
  can continue.
* After 10 wrong passwords the account is locked for 15 minutes. An
  administrator can unlock it or give you a temporary password.

## Sharing — the same everywhere

Lists, documents, polls, calendar events, boards, meetings, decisions and
channels are all shared in the same way. Something you create is **private**
until you share it. In the sharing dialog you add one or more of:

| Share with | Meaning |
|------------|---------|
| Everyone | All members |
| Group | A global group, or one of your own local groups |
| Role | Everyone holding a role, e.g. leaders |
| Person | One member |

and choose **read** (can see; can vote in polls) or **write** (can
contribute: post, add items, upload new versions, edit). Only you, as owner,
can change the sharing or delete the item — plus moderators (leaders) for
items that are shared. Administrators cannot see your private items.

## The sections

### Home

An overview and entry point to the other sections.

### Chat

* **Channels** are topic based conversations. Who can read and who can post
  is decided by the channel's sharing — a channel that everyone can read but
  only leaders can post in works as a notice board. Creating channels
  requires the leader role (permission *create channels*).
* **Direct messages**: start a private conversation with one or several
  members. Only the participants can read it — not administrators.
* **Pin** important messages; pinned messages are listed per channel.
  Anyone who may post in a channel may pin.
* **Attachments**: attach files (Word, Excel, PowerPoint, PDF, images, …) to
  a message. Everyone who can read the conversation can download them.
* You can edit and delete your own messages; moderators can delete messages
  in shared channels. Unread counts are shown per channel, and new messages
  appear without reloading.

### Documents

The archive for files that should be kept: statutes, protocols, forms.

* Upload a file, give it a name, an optional folder (`Board/2026`) and a
  description, and share it.
* **Versions**: upload a new version with a comment; all earlier versions
  stay available for download.
* **Copy** makes a private copy of the current version for yourself.
* Download always gives you the file to open in your own application —
  documents are never opened inside Samling.

### Lists

To-do lists, priority lists and plain lists, private or shared. Items can be
ticked off, given a priority and a due date, and reordered. Changes by
others show up live.

### Polls

Ask a question and let people vote. Kinds:

| Kind | How it works |
|------|--------------|
| Single choice | Pick one option |
| Multiple choice | Pick several, optionally with a maximum |
| Yes / No / Abstain | A formal vote |
| Ranked | Order all options; counted with points per rank (Borda) |
| Score | Give each option 0–5 points |

Options when creating a poll: **anonymous** (nobody — not even an
administrator — can see who voted for what; such a vote cannot be changed),
**allow changing the vote**, a **closing time**, and when **results** are
visible (always, after you voted, or only after closing). The owner can
close a poll early. A closed poll keeps its result and can be the basis for
a recorded decision.

### Calendar

* Month overview of your own and shared events; events can be timed or
  all-day, with place, description and a link (for example a Teams, Zoom,
  Google Meet or Jitsi meeting link).
* **Private** by default; share with people, groups, roles or everyone.
* **Export** the events you can see as an `.ics` file for Outlook, Google
  Calendar, Apple Calendar and others; **import** an `.ics` file to add
  events to your private calendar (importing the same file again updates
  instead of duplicating). Recurring events are imported as their first
  occurrence only.
* Meetings planned in Samling appear automatically.

### Meetings

A simple flow from invitation to signed-off protocol (requires the
permission *plan meetings*, which leaders have):

1. **Plan** — title, time, place or meeting link, and who it is for
   (sharing). The meeting appears in the invitees' calendars.
2. **Agenda** — add, reorder and describe agenda items, with presenter and
   planned duration.
3. **Protocol** — during or after the meeting: who attended, chair,
   secretary, notes per agenda item, and decisions taken under an item
   (recorded straight into the decision log).
4. **Finalise** — locks the protocol. It can be exported as Markdown (or
   printed from the browser). The owner can reopen it if a correction is
   needed; both actions are logged.

### Decisions

The organisation's decision log.

* Each decision gets a number per year (`2026-001`), a date, a text and an
  audience: everyone, certain groups or roles, or specific people.
* Decisions can be linked to the meeting item or poll they came from.
* **Decisions are never deleted or silently changed.** A change — including
  revoking or superseding — is a new revision with a required comment; the
  full history is shown on the decision.
* The log can be exported as CSV. Together with closed polls it forms the
  decision history.

Recording and revising decisions requires the permission *record decisions*
(leaders).

### Boards

Kanban boards for work in progress — private, or shared with a team. A board
has columns (for example *To do*, *Doing*, *Done*) and cards with
description, assignee and due date. Move cards between columns by dragging
or with the card's move controls. People with write access can add and move
cards; the owner manages columns.

### Search

One search across everything you have access to: messages, documents,
lists, polls, events, meetings, decisions, boards and people.

| Mode | Example | Finds |
|------|---------|-------|
| Plain | `summer camp` | Text containing exactly that |
| Wildcard | `protocol 2026-*`, `budget 202?` | `*` = any text, `?` = one character |
| Regular expression | `protocol 20(25\|26)-\d{2}$` | RE2 syntax |

You can limit the search to a period (from/to dates), to certain kinds of
content, and make it case sensitive. You never get hits from things you may
not see. Other members' contact details are only searchable if they have
chosen to show them. The contents of uploaded files are not searched — only
their names, folders and descriptions.

### People

* The **member directory** with names, roles and — where the person allows
  it — e-mail and phone, with links to write an e-mail.
* **Groups**: *global groups* are created by administrators and visible to
  all (sections, teams, the board). *Local groups* are your own: only you
  see them, and you can use them to share things with the same set of people
  quickly.

### History

Your own activity in Samling over time: what you created, changed, voted
on, downloaded and when. Only you can see it.

### Settings

* **Profile**: display name, e-mail, phone, and whether your contact details
  are visible to other members.
* **Language**: Swedish or English, switchable at any time.
* **Appearance**: theme (light, dark, spring, autumn, sand, high contrast)
  and text size.
* **Accessibility**: options such as reduced motion and stronger focus and
  link indication.
* **Password**: change it (this signs out your other devices).
* **Two-factor authentication**: scan the QR code with an authenticator app
  (or enter the key by hand) and confirm with a code. To disable, enter your
  password — unless your administrator requires two-factor.
* **My data**: download a copy of the personal data Samling holds about you.

## For administrators

The Admin section is visible to people with administrative permissions.

### Accounts

* **Create** an account with a temporary password; the person must change it
  at first sign-in. Hand the password over in a safe way.
* **Rename**, change display name or e-mail, **disable** (signs the person
  out), **unlock** after too many failed sign-ins.
* **Reset password** (temporary, signs the person out everywhere).
* **Two-factor**: require it for an account, or reset it when someone lost
  their phone and has no recovery code left. Resetting removes the
  authenticator app, the recovery codes and any passkeys.
* **Sign out everywhere**: ends all sessions of an account at once, for
  example when a device is lost.
* **Delete**: erases the person's personal data, credentials and private
  content. What they contributed to shared places remains, shown as from a
  deleted user. This cannot be undone.
* The last administrator cannot be removed or demoted. If every
  administrator is locked out, use `samling admin passwd` on the server
  (see [deployment.md](deployment.md)).

### Roles and permissions

Three roles are built in: **Administrator** (everything), **Leader** (create
channels, moderate shared content, record decisions, plan meetings) and
**Member**. You can add roles — for example a *Secretary* who may plan
meetings — by combining permissions:

| Permission | Allows |
|------------|--------|
| Manage accounts | Create, rename, disable, delete accounts; passwords; two-factor |
| Manage roles | Roles, role assignment, global groups |
| View logs | The audit log |
| Backup | Create backups and exports |
| Create channels | New chat channels |
| Moderate content | Manage anything that is shared |
| Record decisions | Add and revise decisions |
| Plan meetings | Create meetings |

Roles can also be used as a target when sharing.

### Audit log

Shows security relevant events with time, account and address: sign-ins and
failures, password and two-factor changes, account and role administration,
sharing changes, deletions by moderators, decisions, finalised protocols,
backups and exports. Entries are kept for two years.

### Backup and export

* **Backup**: choose a passphrase (12+ characters) and download an encrypted
  backup of everything. Keep the passphrase safe — it is needed to restore,
  and whoever has file and passphrase has all data. Restoring is done on the
  server, see [deployment.md](deployment.md#backup-and-restore).
* **Member export**: a CSV of the member directory, optionally encrypted
  with a passphrase (decrypt with `samling decrypt`).

### Good practice

* Give leaders and administrators two-factor authentication.
* Have at least two administrators.
* Use global groups for the organisation's real structure, and share with
  groups or roles rather than with long lists of individuals.
* Make sure backups leave the server, and try a restore once.
