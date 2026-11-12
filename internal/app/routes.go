package app

import (
	"net/http"
	"regexp"
)

var identRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

var httpAbort = http.ErrAbortHandler

func (a *App) routes() {
	pub := routeOpt{public: true}
	lim := routeOpt{limited: true} // usable while password change / MFA setup is pending
	std := routeOpt{}
	perm := func(p string) routeOpt { return routeOpt{perm: p} }

	// Authentication and own account
	a.handle("POST /api/auth/login", pub, a.hLogin)
	a.handle("POST /api/auth/logout", lim, a.hLogout)
	a.handle("GET /api/me", lim, a.hMe)
	a.handle("PATCH /api/me", lim, a.hUpdateMe)
	a.handle("POST /api/me/password", lim, a.hChangePassword)
	a.handle("POST /api/me/mfa/setup", lim, a.hMFASetup)
	a.handle("POST /api/me/mfa/enable", lim, a.hMFAEnable)
	a.handle("POST /api/me/mfa/disable", std, a.hMFADisable)
	a.handle("GET /api/me/history", std, a.hMyHistory)
	a.handle("GET /api/me/export", std, a.hMyExport)
	a.handle("GET /api/events", std, a.hEvents)

	// People, groups, roles
	a.handle("GET /api/users", std, a.hUsers)
	a.handle("GET /api/groups", std, a.hGroups)
	a.handle("POST /api/groups", std, a.hCreateGroup)
	a.handle("PUT /api/groups/{id}", std, a.hUpdateGroup)
	a.handle("DELETE /api/groups/{id}", std, a.hDeleteGroup)
	a.handle("GET /api/roles", std, a.hRoles)
	a.handle("POST /api/roles", perm(PermRoles), a.hCreateRole)
	a.handle("PUT /api/roles/{id}", perm(PermRoles), a.hUpdateRole)
	a.handle("DELETE /api/roles/{id}", perm(PermRoles), a.hDeleteRole)

	// Administration
	a.handle("GET /api/admin/users", perm(PermUsers), a.hAdminUsers)
	a.handle("POST /api/admin/users", perm(PermUsers), a.hAdminCreateUser)
	a.handle("PATCH /api/admin/users/{id}", perm(PermUsers), a.hAdminUpdateUser)
	a.handle("POST /api/admin/users/{id}/password", perm(PermUsers), a.hAdminSetPassword)
	a.handle("DELETE /api/admin/users/{id}", perm(PermUsers), a.hAdminDeleteUser)
	a.handle("GET /api/admin/logs", perm(PermLogs), a.hAdminLogs)
	a.handle("POST /api/admin/backup", perm(PermBackup), a.hBackup)
	a.handle("POST /api/admin/export", perm(PermBackup), a.hAdminExport)

	// Sharing
	a.handle("GET /api/acl/{type}/{id}", std, a.hGetACL)
	a.handle("PUT /api/acl/{type}/{id}", std, a.hSetACL)

	// Chat
	a.handle("GET /api/channels", std, a.hChannels)
	a.handle("POST /api/channels", perm(PermChannels), a.hCreateChannel)
	a.handle("PATCH /api/channels/{id}", std, a.hUpdateChannel)
	a.handle("DELETE /api/channels/{id}", std, a.hDeleteChannel)
	a.handle("POST /api/dm", std, a.hOpenDM)
	a.handle("GET /api/channels/{id}/messages", std, a.hMessages)
	a.handle("POST /api/channels/{id}/messages", std, a.hPostMessage)
	a.handle("GET /api/channels/{id}/pins", std, a.hPins)
	a.handle("POST /api/channels/{id}/read", std, a.hMarkRead)
	a.handle("PATCH /api/messages/{id}", std, a.hEditMessage)
	a.handle("DELETE /api/messages/{id}", std, a.hDeleteMessage)
	a.handle("POST /api/messages/{id}/pin", std, a.hPin)
	a.handle("DELETE /api/messages/{id}/pin", std, a.hPin)

	// Files and documents
	a.handle("POST /api/files", std, a.hUpload)
	a.handle("GET /api/files/{id}", std, a.hDownload)
	a.handle("GET /api/documents", std, a.hDocuments)
	a.handle("POST /api/documents", std, a.hCreateDocument)
	a.handle("GET /api/documents/{id}", std, a.hDocument)
	a.handle("PATCH /api/documents/{id}", std, a.hUpdateDocument)
	a.handle("DELETE /api/documents/{id}", std, a.hDeleteDocument)
	a.handle("POST /api/documents/{id}/versions", std, a.hAddVersion)
	a.handle("POST /api/documents/{id}/copy", std, a.hCopyDocument)
	a.handle("GET /api/documents/{id}/download", std, a.hDownloadDocument)

	// Lists
	a.handle("GET /api/lists", std, a.hLists)
	a.handle("POST /api/lists", std, a.hCreateList)
	a.handle("GET /api/lists/{id}", std, a.hList)
	a.handle("PATCH /api/lists/{id}", std, a.hUpdateList)
	a.handle("DELETE /api/lists/{id}", std, a.hDeleteList)
	a.handle("POST /api/lists/{id}/items", std, a.hAddItem)
	a.handle("PATCH /api/lists/{id}/items/{item}", std, a.hUpdateItem)
	a.handle("DELETE /api/lists/{id}/items/{item}", std, a.hDeleteItem)

	// Polls
	a.handle("GET /api/polls", std, a.hPolls)
	a.handle("POST /api/polls", std, a.hCreatePoll)
	a.handle("GET /api/polls/{id}", std, a.hPoll)
	a.handle("DELETE /api/polls/{id}", std, a.hDeletePoll)
	a.handle("POST /api/polls/{id}/vote", std, a.hVote)
	a.handle("POST /api/polls/{id}/close", std, a.hClosePoll)

	// Calendar
	a.handle("GET /api/calendar", std, a.hCalendar)
	a.handle("POST /api/calendar", std, a.hCreateEvent)
	a.handle("PATCH /api/calendar/{id}", std, a.hUpdateEvent)
	a.handle("DELETE /api/calendar/{id}", std, a.hDeleteEvent)
	a.handle("GET /api/calendar/export.ics", std, a.hExportICS)
	a.handle("POST /api/calendar/import", std, a.hImportICS)

	// Meetings and decisions
	a.handle("GET /api/meetings", std, a.hMeetings)
	a.handle("POST /api/meetings", perm(PermMeetings), a.hCreateMeeting)
	a.handle("GET /api/meetings/{id}", std, a.hMeeting)
	a.handle("PATCH /api/meetings/{id}", std, a.hUpdateMeeting)
	a.handle("DELETE /api/meetings/{id}", std, a.hDeleteMeeting)
	a.handle("PUT /api/meetings/{id}/agenda", std, a.hSetAgenda)
	a.handle("POST /api/meetings/{id}/finalize", std, a.hFinalizeMeeting)
	a.handle("POST /api/meetings/{id}/reopen", std, a.hFinalizeMeeting)
	a.handle("GET /api/meetings/{id}/protocol.md", std, a.hProtocol)
	a.handle("GET /api/decisions", std, a.hDecisions)
	a.handle("POST /api/decisions", perm(PermDecisions), a.hCreateDecision)
	a.handle("GET /api/decisions/export.csv", std, a.hExportDecisions)
	a.handle("GET /api/decisions/{id}", std, a.hDecision)
	a.handle("PATCH /api/decisions/{id}", std, a.hReviseDecision)

	// Kanban boards
	a.handle("GET /api/boards", std, a.hBoards)
	a.handle("POST /api/boards", std, a.hCreateBoard)
	a.handle("GET /api/boards/{id}", std, a.hBoard)
	a.handle("PATCH /api/boards/{id}", std, a.hUpdateBoard)
	a.handle("DELETE /api/boards/{id}", std, a.hDeleteBoard)
	a.handle("POST /api/boards/{id}/columns", std, a.hAddColumn)
	a.handle("PATCH /api/boards/{id}/columns/{col}", std, a.hUpdateColumn)
	a.handle("DELETE /api/boards/{id}/columns/{col}", std, a.hDeleteColumn)
	a.handle("POST /api/boards/{id}/cards", std, a.hAddCard)
	a.handle("PATCH /api/boards/{id}/cards/{card}", std, a.hUpdateCard)
	a.handle("DELETE /api/boards/{id}/cards/{card}", std, a.hDeleteCard)

	a.handle("GET /api/search", std, a.hSearch)
}
