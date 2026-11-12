// Applies the saved appearance before first paint (avoids a flash of the
// default theme) and registers the service worker. Kept as a tiny classic
// script because the Content-Security-Policy forbids inline scripts.
(function () {
  try {
    var p = JSON.parse(localStorage.getItem('samling.prefs') || '{}');
    var d = document.documentElement.dataset;
    if (p.theme) d.theme = p.theme;
    else if (window.matchMedia && matchMedia('(prefers-color-scheme: dark)').matches) d.theme = 'dark';
    if (p.size) d.size = p.size;
    if (p.motion) d.motion = 'reduce';
    if (p.underline) d.underline = '1';
    if (p.readable) d.readable = '1';
    if (p.focus) d.focus = '1';
  } catch (e) { /* storage unavailable */ }
  if ('serviceWorker' in navigator) {
    window.addEventListener('load', function () { navigator.serviceWorker.register('/sw.js').catch(function () {}); });
  }
})();
