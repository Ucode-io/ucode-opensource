import { expect, test } from "vitest";
import { toActors, toUsage } from "./usage";

test("таблица подставляется в шаблон маршрута", () => {
  const usage = toUsage({
    limit: 100,
    used: 10,
    top: [
      { source: "client", method: "GET", route: "/v2/items/:collection", collection: "deal", count: 5, percent: 50 },
      { source: "admin", method: "GET", route: "/v3/menus", collection: "", count: 5, percent: 50 },
    ],
  });

  expect(usage.top.map((row) => row.label)).toEqual(["GET /v2/items/deal", "GET /v3/menus"]);
  expect(usage.percentUsed).toBe(10);
});

/* Ручка отдаёт маршрут по строке на тип авторизации — человеку это одна строка. */
test("один маршрут с разными типами авторизации складывается в одну строку", () => {
  const usage = toUsage({
    limit: 100000,
    used: 245,
    top: [
      { source: "admin", auth_type: "bearer", method: "GET", route: "/v3/menus", count: 28, percent: 11.43 },
      { source: "admin", auth_type: "", method: "GET", route: "/v3/menus", count: 15, percent: 6.12 },
      { source: "admin", auth_type: "api_key", method: "GET", route: "/v1/table/:table_id", count: 14, percent: 5.71 },
    ],
  });

  expect(usage.top).toHaveLength(2);
  expect(usage.top.map((row) => row.count)).toEqual([43, 14]);
  expect(usage.top.map((row) => row.percent)).toEqual([17.55, 5.71]);
  // Сырые части маршрута переживают склейку: ими строка раскрывается.
  expect(usage.top.map((row) => row.route)).toEqual(["/v3/menus", "/v1/table/:table_id"]);
});

/* Безлимит приходит двумя способами: флагом и нулевым лимитом. */
test("без лимита процент не считается", () => {
  expect(toUsage({ limit: 0, used: 5000, remaining: null, unlimited: true }).percentUsed).toBeNull();
  expect(toUsage({ limit: 0, used: 5000 }).percentUsed).toBeNull();
});

/* Флаг блокировки и счётчик — разные снимки: перерасход возможен. */
test("процент не выходит за сотню", () => {
  expect(toUsage({ limit: 100, used: 250 }).percentUsed).toBe(100);
});

/* actor_name есть только у ключей; людям достаётся короткий id. */
test("отправитель: имя ключа важнее идентификатора, длинный id режется", () => {
  const actors = toActors({
    top: [
      { auth_type: "api_key", actor_name: "Function", actor_id: "ignored", count: 5, percent: 9.43 },
      { auth_type: "bearer", actor_id: "eb4675e9-03d8-400e-aad7-e8c76af95480", count: 31, percent: 58.49 },
      { auth_type: "", count: 11, percent: 20.75 },
    ],
  });

  expect(actors.map((actor) => actor.name)).toEqual(["Function", "eb4675e9…", ""]);
});
