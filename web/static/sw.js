// Minimal service worker: makes Samling installable as an app (PWA).
// Nothing is cached on purpose – the data is access controlled and may be
// sensitive, so every request goes to the server.
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (e) => e.waitUntil(self.clients.claim()));
self.addEventListener('fetch', () => {});
