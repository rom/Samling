# Phones and tablets

## Today: responsive web app, installable (PWA)

The client adapts to phone, tablet and desktop widths. It ships a web app
manifest, icons and a minimal service worker, so it can be installed and
then opens full-screen like an app:

* **iPhone / iPad (Safari)**: Share → *Add to Home Screen*.
* **Android (Chrome)**: menu → *Install app* / *Add to Home screen*.

Requirements: the server must be reached over HTTPS with a certificate the
device trusts (use Let's Encrypt; a self-signed certificate must first be
installed and trusted on the device).

The service worker does **not** cache data or pages — everything is access
controlled and fetched live — so there is no offline mode and no push
notifications in the MVP.

## Native iOS and Android apps

**Status: not included in the MVP.** The server side is prepared:

* Every function of the web client goes through the documented REST API
  ([api.md](api.md)).
* **Bearer token login** for apps: `POST /api/auth/login` with
  `"token": true` returns a token to send as `Authorization: Bearer …`.
  No cookies and no CSRF token are involved.
* Live updates use standard Server-Sent Events.

### Option A — wrapper around the web client (fastest)

Use [Capacitor](https://capacitorjs.com/) (or a plain `WKWebView` /
Android `WebView` shell) pointing at the server's URL:

```json
{
  "appId": "org.example.samling",
  "appName": "Samling",
  "server": { "url": "https://club.example.org", "cleartext": false }
}
```

Because the web view loads the client *from the server*, requests are
same-origin: the normal cookie session works and **no CORS configuration is
needed** (the server sends no CORS headers, by design). The app must let the
user enter the server address on first start if it is to work for more than
one organisation.

### Option B — fully native clients

Swift/Kotlin (or Flutter/React Native) clients call the API directly with
bearer tokens. Store the token in the **iOS Keychain** / **Android
Keystore** (EncryptedSharedPreferences), never in plain preferences.

### What would need to be added

| Area | Work |
|------|------|
| Push notifications | A notification service on the server (APNs / FCM) and device registration endpoints; today updates only arrive while the app is open |
| Server selection | A first-run screen for the server URL; optionally pinning the server certificate |
| File handling | Native share/open-in for downloads and uploads from the device (camera, files) |
| Session management | Listing and revoking app sessions per device (the session table already records kind, address and user agent) |
| Biometric unlock | Local protection of the stored token |
| Offline | A local cache with careful handling of access changes — deliberately not done in the web client |
| Store publishing | Signing, privacy declarations, review |

Bundling the web client *inside* the app (instead of loading it from the
server) would require CORS support and token based authentication in the
JavaScript client; loading from the server avoids both.
