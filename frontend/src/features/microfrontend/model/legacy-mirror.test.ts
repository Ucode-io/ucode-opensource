import { beforeEach, expect, test, vi } from "vitest";

/*
 * Сессия читает localStorage при загрузке модуля — подмена идёт ДО
 * импорта, как в shared/api/client.test.
 */
const store = new Map<string, string>();
vi.stubGlobal("localStorage", {
  getItem: (key: string) => store.get(key) ?? null,
  setItem: (key: string, value: string) => void store.set(key, value),
  removeItem: (key: string) => void store.delete(key),
  clear: () => store.clear(),
});
// Зеркало вешает слушателя pagehide — в node-окружении window нет.
vi.stubGlobal("window", { addEventListener: () => {}, removeEventListener: () => {} });

const { clearLegacyMirror, startLegacyMirror, writeLegacyMirror } = await import("./legacy-mirror");
const { session } = await import("@/shared/api/session");

/**
 * Проверяется ровно то, как зеркало читает РЕМОУТ: два `JSON.parse`
 * подряд. Формат redux-persist 6 — значение ключа JSON, и каждое поле
 * внутри тоже JSON. Разойдётся форма — ремоут молча возьмёт свой
 * протухший токен, и по экрану будет видно только «Failed to load data».
 */
function fromMirror(key: string, field: string): unknown {
  const raw = localStorage.getItem(key);
  if (!raw) return undefined;

  const fields = JSON.parse(raw) as Record<string, string>;
  const value = fields[field];
  return value === undefined ? undefined : JSON.parse(value);
}

beforeEach(() => {
  store.clear();
  session.clear();
});

test("ремоут достаёт из зеркала живой токен, окружение и проект", () => {
  session.set({ access: "TOKEN", refresh: "REFRESH", environmentId: "ENV", projectId: "PRJ" });
  writeLegacyMirror("Production");

  expect(fromMirror("persist:auth", "token")).toBe("TOKEN");
  expect(fromMirror("persist:auth", "refreshToken")).toBe("REFRESH");
  expect(fromMirror("persist:auth", "isAuth")).toBe(true);

  // Заголовок environment-id ремоут берёт из company, а не из auth
  // (httpsRequest.js:82-88 старой админки) — значит в обоих ключах.
  expect(fromMirror("persist:company", "environmentId")).toBe("ENV");
  expect(fromMirror("persist:company", "projectId")).toBe("PRJ");
  // По имени окружения ремоут отличает production от staging.
  expect(fromMirror("persist:company", "environmentItem")).toEqual({
    id: "ENV",
    name: "Production",
  });

  // Без _persist redux-persist считает запись чужой и не рехидратирует.
  expect(fromMirror("persist:auth", "_persist")).toEqual({ version: -1, rehydrated: true });
});

test("обновление токена посреди работы ремоута доезжает до зеркала", () => {
  session.set({ access: "OLD", refresh: "R", environmentId: "ENV", projectId: "PRJ" });
  writeLegacyMirror("Staging");

  const stop = startLegacyMirror();
  session.set({ access: "NEW", refresh: "R2" });

  expect(fromMirror("persist:auth", "token")).toBe("NEW");
  expect(fromMirror("persist:auth", "refreshToken")).toBe("R2");
  // Имя окружения переписывается тем же, что было: сессия его не знает.
  expect(fromMirror("persist:company", "environmentItem")).toEqual({ id: "ENV", name: "Staging" });

  stop();
});

test("новому ремоуту зеркало не достаётся вовсе", () => {
  // Загрузчик пишет зеркало только для старого поколения. Подписка
  // не имеет права создать его задним числом — иначе ремоут на SDK
  // получил бы ровно то, чего мы ему не даём: токен в хранилище.
  session.set({ access: "TOKEN", refresh: "R" });

  const stop = startLegacyMirror();
  session.set({ access: "ANOTHER", refresh: "R2" });

  expect(localStorage.getItem("persist:auth")).toBeNull();
  stop();
});

test("уход с экрана и выход из системы уносят зеркало", () => {
  session.set({ access: "TOKEN", refresh: "R" });
  writeLegacyMirror("Staging");

  const stop = startLegacyMirror();
  stop();
  expect(localStorage.getItem("persist:auth")).toBeNull();
  expect(localStorage.getItem("persist:company")).toBeNull();

  // Выход при ОТКРЫТОМ экране: подписка обязана стереть зеркало сама,
  // иначе токен остался бы в хранилище после логаута.
  writeLegacyMirror("Staging");
  const stopAgain = startLegacyMirror();
  session.clear();
  expect(localStorage.getItem("persist:auth")).toBeNull();

  stopAgain();
});

test("подчистка на старте уносит и хвост старой админки", () => {
  // Ключи те же, а её дев-сервер слушает тот же порт — именно отсюда
  // ремоут однажды подобрал восьмидневный токен.
  localStorage.setItem("persist:auth", '{"token":"\\"stale\\""}');
  clearLegacyMirror();
  expect(localStorage.getItem("persist:auth")).toBeNull();
});
