import { expect, test } from "vitest";
import { withLiveToken } from "./dev-token";

/** JWT без подписи: разбирается только `exp`, проверять нечего. */
function jwt(secondsFromNow: number): string {
  const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + secondsFromNow }));
  return `header.${payload}.signature`;
}

/*
 * Подмена обязана трогать ТОЛЬКО протухшее. Начни она чинить всё
 * подряд — и настоящая ошибка доступа выглядела бы как успех,
 * а на боевом стенде это же место молчало бы о просроченной сессии.
 */

test("протухший чужой токен заменяется живым", () => {
  // Ровно этот случай: ремоут зашил токен в сборку, тот протух.
  const expired = jwt(-3600);
  expect(withLiveToken(`Bearer ${expired}`, "ЖИВОЙ")).toBe("Bearer ЖИВОЙ");
});

test("живой чужой токен не трогаем", () => {
  const fresh = `Bearer ${jwt(3600)}`;
  expect(withLiveToken(fresh, "ЖИВОЙ")).toBe(fresh);
});

test("не-JWT проходит как есть", () => {
  // Ключи других сервисов, Basic-авторизация, что угодно чужое.
  expect(withLiveToken("Bearer P-bToNkSoxb3YuY5R5ldVmdgFfQQiB1OqA", "ЖИВОЙ")).toBe(
    "Bearer P-bToNkSoxb3YuY5R5ldVmdgFfQQiB1OqA",
  );
  expect(withLiveToken("Basic abc", "ЖИВОЙ")).toBe("Basic abc");
});

test("без своей сессии не подменяем ничего", () => {
  const expired = `Bearer ${jwt(-1)}`;
  expect(withLiveToken(expired, null)).toBe(expired);
});
