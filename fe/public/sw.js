// 서비스 워커 — 오프라인일 때 화면 이동만 offline.html로 받는다.
// 그 밖의 요청(JS·CSS·API·RSC·시세 스트림)은 손대지 않는다: 페이지나 번들을 캐시하면
// 배포가 사용자에게 늦게 닿는다. offline.html을 고치면 CACHE 판을 올린다.
const CACHE = "offline-v1";
const OFFLINE = "/offline.html";

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE).then((c) => c.add(OFFLINE)));
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      // 워커가 깨어나는 동안 내비게이션 요청을 먼저 보내 둔다 — 첫 화면 지연을 없앤다.
      if (self.registration.navigationPreload) await self.registration.navigationPreload.enable();
      for (const key of await caches.keys()) if (key !== CACHE) await caches.delete(key);
      await self.clients.claim();
    })(),
  );
});

self.addEventListener("fetch", (event) => {
  if (event.request.mode !== "navigate") return;
  event.respondWith(
    (async () => {
      try {
        return (await event.preloadResponse) || (await fetch(event.request));
      } catch {
        return (await caches.match(OFFLINE)) || Response.error();
      }
    })(),
  );
});
