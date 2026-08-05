// Probakgo service worker
//
// This is the only piece of JavaScript that runs in the background outside
// the page lifecycle. Its job is small on purpose:
//   1. Receive Web Push messages from the server and show them as native
//      OS notifications.
//   2. Focus or open the relevant page when the user clicks the toast.
//   3. Keep the offline experience reasonable: never block the user, never
//      intercept fetches, just provide the absolute minimum so the page
//      itself still works when the network is flaky.
//
// The SW is intentionally NOT a network proxy. The UI is a thin SPA over
// server-rendered HTML, so any aggressive caching would mask server-side
// alerts. Push and notification click handling are the only responsibilities.

self.addEventListener('install', (event) => {
  // Take over from any previous SW immediately so the first page load
  // after a deploy does not keep the old push handler.
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  // Claim all open clients without a reload; the UI is small enough that
  // the next XHR will pick up the new SW through the natural page life.
  event.waitUntil(self.clients.claim());
});

self.addEventListener('push', (event) => {
  let payload = {};
  try {
    payload = event.data ? event.data.json() : {};
  } catch (err) {
    payload = { title: 'Probakgo', body: event.data ? event.data.text() : '' };
  }
  const title = payload.title || 'Probakgo';
  const options = {
    body: payload.body || 'Nueva alerta',
    tag: payload.tag || undefined,
    icon: payload.icon || '/static/icons/icon-192.png',
    badge: '/static/icons/icon-192.png',
    data: { url: payload.url || '/alerts' },
    requireInteraction: payload.severity === 'critical',
    vibrate: payload.severity === 'critical' ? [250, 100, 250, 100, 250] : [120, 60, 120],
    renotify: Boolean(payload.tag),
  };
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const target = (event.notification.data && event.notification.data.url) || '/alerts';
  event.waitUntil((async () => {
    let absolute;
    try {
      const requested = new URL(target, self.location.origin);
      absolute = requested.origin === self.location.origin ? requested.toString() : new URL('/alerts', self.location.origin).toString();
    } catch (_) {
      absolute = new URL('/alerts', self.location.origin).toString();
    }
    const all = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
    for (const client of all) {
      // If the dashboard is already open, focus it and navigate.
      try {
        const url = new URL(client.url);
        if (url.origin === self.location.origin) {
          await client.focus();
          if ('navigate' in client) {
            return client.navigate(absolute);
          }
          return client.postMessage({ type: 'pbk-navigate', url: absolute });
        }
      } catch (_) { /* ignore malformed client URLs */ }
    }
    if (self.clients.openWindow) {
      return self.clients.openWindow(absolute);
    }
  })());
});
