package db

import "time"

func nowUnix() int64 { return time.Now().Unix() }

// Tables lists every application table in an order that is safe for restore
// (parents before children). Used by backup/restore.
var Tables = []string{
	"users", "roles", "role_perms", "user_roles", "groups", "group_members", "acl",
	"files", "channels", "messages", "message_files", "channel_reads",
	"documents", "doc_versions", "lists", "list_items",
	"polls", "poll_options", "ballots", "ballot_choices", "poll_voters",
	"events", "meetings", "agenda_items", "decisions", "decision_revisions",
	"boards", "board_columns", "cards", "activity", "settings",
	"recovery_codes", "passkeys", "reactions", "dm_blocks", "attachments",
}

// migrations are applied in order; never edit an entry that has been
// released, append a new one instead. Statements are separated by ";\n".
var migrations = []string{`
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  email_enc TEXT NOT NULL DEFAULT '',
  email_idx TEXT NOT NULL DEFAULT '',
  phone_enc TEXT NOT NULL DEFAULT '',
  contact_visible INTEGER NOT NULL DEFAULT 1,
  password_hash TEXT NOT NULL,
  must_change_password INTEGER NOT NULL DEFAULT 0,
  totp_secret_enc TEXT NOT NULL DEFAULT '',
  totp_enabled INTEGER NOT NULL DEFAULT 0,
  totp_last_step BIGINT NOT NULL DEFAULT 0,
  mfa_required INTEGER NOT NULL DEFAULT 0,
  disabled INTEGER NOT NULL DEFAULT 0,
  deleted_at BIGINT NOT NULL DEFAULT 0,
  locale TEXT NOT NULL DEFAULT 'sv',
  settings TEXT NOT NULL DEFAULT '{}',
  failed_logins INTEGER NOT NULL DEFAULT 0,
  locked_until BIGINT NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL,
  last_login_at BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE sessions (
  id_hash TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  csrf TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'cookie',
  ip TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL,
  last_seen BIGINT NOT NULL,
  expires_at BIGINT NOT NULL
);
CREATE INDEX sessions_user ON sessions(user_id);
CREATE TABLE roles (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  builtin INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE role_perms (
  role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  perm TEXT NOT NULL,
  PRIMARY KEY (role_id, perm)
);
CREATE TABLE user_roles (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, role_id)
);
CREATE TABLE groups (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  scope TEXT NOT NULL,
  owner_id TEXT NOT NULL,
  created_at BIGINT NOT NULL
);
CREATE TABLE group_members (
  group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (group_id, user_id)
);
CREATE TABLE acl (
  res_type TEXT NOT NULL,
  res_id TEXT NOT NULL,
  subj_type TEXT NOT NULL,
  subj_id TEXT NOT NULL DEFAULT '',
  level INTEGER NOT NULL,
  PRIMARY KEY (res_type, res_id, subj_type, subj_id)
);
CREATE INDEX acl_subject ON acl(subj_type, subj_id);
CREATE TABLE files (
  id TEXT PRIMARY KEY,
  blob_key TEXT NOT NULL,
  name TEXT NOT NULL,
  mime TEXT NOT NULL,
  size BIGINT NOT NULL,
  sha256 TEXT NOT NULL,
  uploader_id TEXT NOT NULL,
  created_at BIGINT NOT NULL
);
CREATE TABLE channels (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'channel',
  name TEXT NOT NULL DEFAULT '',
  topic TEXT NOT NULL DEFAULT '',
  dm_key TEXT NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
CREATE INDEX channels_dm ON channels(dm_key);
CREATE TABLE messages (
  id TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at BIGINT NOT NULL,
  edited_at BIGINT NOT NULL DEFAULT 0,
  deleted_at BIGINT NOT NULL DEFAULT 0,
  pinned_at BIGINT NOT NULL DEFAULT 0,
  pinned_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX messages_channel ON messages(channel_id, created_at);
CREATE TABLE message_files (
  message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  file_id TEXT NOT NULL REFERENCES files(id),
  PRIMARY KEY (message_id, file_id)
);
CREATE TABLE channel_reads (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  last_read_at BIGINT NOT NULL,
  PRIMARY KEY (user_id, channel_id)
);
CREATE TABLE documents (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL,
  folder TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  current_version INTEGER NOT NULL DEFAULT 1,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
CREATE TABLE doc_versions (
  doc_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  version INTEGER NOT NULL,
  file_id TEXT NOT NULL REFERENCES files(id),
  comment TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL,
  created_at BIGINT NOT NULL,
  PRIMARY KEY (doc_id, version)
);
CREATE TABLE lists (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL,
  title TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'todo',
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
CREATE TABLE list_items (
  id TEXT PRIMARY KEY,
  list_id TEXT NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
  text TEXT NOT NULL,
  done INTEGER NOT NULL DEFAULT 0,
  priority INTEGER NOT NULL DEFAULT 0,
  position INTEGER NOT NULL DEFAULT 0,
  due_at BIGINT NOT NULL DEFAULT 0,
  created_by TEXT NOT NULL,
  created_at BIGINT NOT NULL
);
CREATE INDEX list_items_list ON list_items(list_id);
CREATE TABLE polls (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL,
  anonymous INTEGER NOT NULL DEFAULT 0,
  results_visible TEXT NOT NULL DEFAULT 'after_vote',
  allow_change INTEGER NOT NULL DEFAULT 0,
  max_choices INTEGER NOT NULL DEFAULT 0,
  closes_at BIGINT NOT NULL DEFAULT 0,
  closed_at BIGINT NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL
);
CREATE TABLE poll_options (
  id TEXT PRIMARY KEY,
  poll_id TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
  text TEXT NOT NULL,
  position INTEGER NOT NULL
);
CREATE TABLE ballots (
  id TEXT PRIMARY KEY,
  poll_id TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX ballots_poll ON ballots(poll_id);
CREATE TABLE ballot_choices (
  ballot_id TEXT NOT NULL REFERENCES ballots(id) ON DELETE CASCADE,
  option_id TEXT NOT NULL,
  value INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (ballot_id, option_id)
);
CREATE TABLE poll_voters (
  poll_id TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL,
  PRIMARY KEY (poll_id, user_id)
);
CREATE TABLE events (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  location TEXT NOT NULL DEFAULT '',
  url TEXT NOT NULL DEFAULT '',
  starts_at BIGINT NOT NULL,
  ends_at BIGINT NOT NULL,
  all_day INTEGER NOT NULL DEFAULT 0,
  uid TEXT NOT NULL DEFAULT '',
  meeting_id TEXT NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
CREATE INDEX events_time ON events(starts_at);
CREATE TABLE meetings (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  location TEXT NOT NULL DEFAULT '',
  url TEXT NOT NULL DEFAULT '',
  starts_at BIGINT NOT NULL,
  ends_at BIGINT NOT NULL,
  status TEXT NOT NULL DEFAULT 'planned',
  attendees TEXT NOT NULL DEFAULT '',
  chair TEXT NOT NULL DEFAULT '',
  secretary TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '',
  finalized_at BIGINT NOT NULL DEFAULT 0,
  finalized_by TEXT NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
CREATE TABLE agenda_items (
  id TEXT PRIMARY KEY,
  meeting_id TEXT NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  presenter TEXT NOT NULL DEFAULT '',
  duration_min INTEGER NOT NULL DEFAULT 0,
  notes TEXT NOT NULL DEFAULT '',
  decision_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX agenda_meeting ON agenda_items(meeting_id);
CREATE TABLE decisions (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL,
  number TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  body TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'decided',
  decided_at BIGINT NOT NULL,
  meeting_id TEXT NOT NULL DEFAULT '',
  poll_id TEXT NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
CREATE TABLE decision_revisions (
  id TEXT PRIMARY KEY,
  decision_id TEXT NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  status TEXT NOT NULL,
  comment TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL,
  created_at BIGINT NOT NULL
);
CREATE INDEX decision_revisions_d ON decision_revisions(decision_id);
CREATE TABLE boards (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
CREATE TABLE board_columns (
  id TEXT PRIMARY KEY,
  board_id TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  position INTEGER NOT NULL
);
CREATE TABLE cards (
  id TEXT PRIMARY KEY,
  board_id TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  column_id TEXT NOT NULL REFERENCES board_columns(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  assignee_id TEXT NOT NULL DEFAULT '',
  due_at BIGINT NOT NULL DEFAULT 0,
  position INTEGER NOT NULL DEFAULT 0,
  created_by TEXT NOT NULL,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
CREATE INDEX cards_board ON cards(board_id);
CREATE TABLE activity (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  res_type TEXT NOT NULL DEFAULT '',
  res_id TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  ip TEXT NOT NULL DEFAULT '',
  audit INTEGER NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL
);
CREATE INDEX activity_user ON activity(user_id, created_at);
CREATE INDEX activity_audit ON activity(audit, created_at);
CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
)`,
	// 2: display name key (uniqueness), MFA recovery codes, passkeys, and a
	// guard against more than one ballot per voter in non-anonymous polls.
	`
ALTER TABLE users ADD COLUMN display_key TEXT NOT NULL DEFAULT '';
CREATE TABLE recovery_codes (
  code_hash TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  used_at BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX recovery_codes_user ON recovery_codes(user_id);
CREATE TABLE passkeys (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  cred_id TEXT NOT NULL UNIQUE,
  data TEXT NOT NULL,
  created_at BIGINT NOT NULL,
  last_used_at BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX passkeys_user ON passkeys(user_id);
CREATE UNIQUE INDEX ballots_one_per_voter ON ballots(poll_id, user_id) WHERE user_id <> ''`,
	// 3: profile fields and avatar, task owners in lists, colours on boards
	// and cards, replies and reactions in chat, blocking direct messages,
	// attachments (photos) on lists, events, polls, decisions and cards.
	`
ALTER TABLE users ADD COLUMN profile_enc TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN profile_shared TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN avatar TEXT NOT NULL DEFAULT '';
ALTER TABLE list_items ADD COLUMN assignee_id TEXT NOT NULL DEFAULT '';
ALTER TABLE boards ADD COLUMN color TEXT NOT NULL DEFAULT '';
ALTER TABLE cards ADD COLUMN color TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN parent_id TEXT NOT NULL DEFAULT '';
CREATE TABLE reactions (
  message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL,
  emoji TEXT NOT NULL,
  PRIMARY KEY (message_id, user_id, emoji)
);
CREATE TABLE dm_blocks (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  blocked_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at BIGINT NOT NULL,
  PRIMARY KEY (user_id, blocked_id)
);
CREATE TABLE attachments (
  res_type TEXT NOT NULL,
  res_id TEXT NOT NULL,
  file_id TEXT NOT NULL REFERENCES files(id),
  user_id TEXT NOT NULL,
  created_at BIGINT NOT NULL,
  PRIMARY KEY (res_type, res_id, file_id)
);
CREATE INDEX attachments_file ON attachments(file_id)`,
}
