import { expect, test } from "vitest";
import { toUsage } from "./usage";

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

/* Безлимит приходит двумя способами: флагом и нулевым лимитом. */
test("без лимита процент не считается", () => {
  expect(toUsage({ limit: 0, used: 5000, remaining: null, unlimited: true }).percentUsed).toBeNull();
  expect(toUsage({ limit: 0, used: 5000 }).percentUsed).toBeNull();
});

/* Флаг блокировки и счётчик — разные снимки: перерасход возможен. */
test("процент не выходит за сотню", () => {
  expect(toUsage({ limit: 100, used: 250 }).percentUsed).toBe(100);
});
