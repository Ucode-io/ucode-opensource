import { beforeEach, expect, test, vi } from "vitest";

/*
 * Клиент читает адреса и localStorage прямо при загрузке модуля,
 * поэтому подмена окружения идёт ДО импорта — как в user-id.test.
 */
vi.stubEnv("VITE_API_URL", "https://api.test");
vi.stubEnv("VITE_AUTH_URL", "https://auth.test");

const store = new Map<string, string>();
vi.stubGlobal("localStorage", {
  getItem: (key: string) => store.get(key) ?? null,
  setItem: (key: string, value: string) => void store.set(key, value),
  removeItem: (key: string) => void store.delete(key),
  clear: () => store.clear(),
});

const { ApiError, ensureAccessToken, errorText, refreshOnce, setRefreshHandler, unwrap } =
  await import("./client");
const { session } = await import("./session");

/*
 * Здесь проверяется то, ради чего этот модуль вообще один на всё
 * приложение: конверт снимается в одном месте, обновление токена
 * происходит одним запросом на всех, а причина отказа доходит до
 * человека. В старом ucode было восемь инстансов axios и четыре копии
 * логики refresh — отсюда и случайные разлогины.
 */

beforeEach(() => {
  store.clear();
  session.clear();
  setRefreshHandler(async () => false);
});

test("конверт снимается только с настоящего конверта", () => {
  expect(unwrap({ status: "OK", data: { id: 1 } })).toEqual({ id: 1 });

  // Без status это не конверт: такой ответ не должен стать undefined.
  expect(unwrap({ data: { id: 1 } })).toEqual({ data: { id: 1 } });
  expect(unwrap({ id: 1 })).toEqual({ id: 1 });
  expect(unwrap(null)).toBeNull();
  expect(unwrap("plain")).toBe("plain");
});

test("вложенный конверт разворачивается ровно один раз", () => {
  // Список строк приходит как {status, data: {count, response}} —
  // второй data принадлежит ручке, а не транспорту.
  const body = { status: "OK", data: { data: { count: 2, response: [] } } };

  expect(unwrap(body)).toEqual({ data: { count: 2, response: [] } });
});

test("причина ошибки достаётся из тела ответа", () => {
  // Так её отдаёт шлюз: строкой в data.
  expect(errorText(new ApiError(500, "this table is auth table"))).toBe("this table is auth table");
  expect(errorText(new ApiError(400, { message: " slug is required " }))).toBe("slug is required");
  expect(errorText(new ApiError(500, { description: "not found" }))).toBe("not found");
});

test("говорить нечего — говорим null, а не пустую строку", () => {
  expect(errorText(new ApiError(500, ""))).toBeNull();
  expect(errorText(new ApiError(500, "   "))).toBeNull();
  expect(errorText(new ApiError(500, { code: 7 }))).toBeNull();
  // Сетевой сбой — не ApiError: там нет ответа сервера.
  expect(errorText(new Error("Network Error"))).toBeNull();
});

test("параллельные запросы ждут один refresh", async () => {
  let calls = 0;
  setRefreshHandler(async () => {
    calls += 1;
    await Promise.resolve();
    return true;
  });

  const results = await Promise.all([refreshOnce(), refreshOnce(), refreshOnce()]);

  expect(calls).toBe(1);
  expect(results).toEqual([true, true, true]);

  // Следующая волна — уже новый запрос, а не тот же промис.
  await refreshOnce();
  expect(calls).toBe(2);
});

test("неудачный refresh не залипает навсегда", async () => {
  // Промис снимается в finally: иначе один отказ навсегда закрывал бы
  // возможность обновиться, и приложение висело бы разлогиненным.
  setRefreshHandler(async () => {
    throw new Error("network");
  });
  await expect(refreshOnce()).rejects.toThrow("network");

  setRefreshHandler(async () => true);
  await expect(refreshOnce()).resolves.toBe(true);
});

test("токен восстанавливается до первого запроса, а не после отказа", async () => {
  const refresh = vi.fn(async () => {
    session.set({ access: "a", refresh: "r" });
    return true;
  });
  setRefreshHandler(refresh);

  // Ни access, ни refresh — восстанавливаться нечем, и запрос обновления
  // не нужен: шлюз ответит на такой запрос 403, а не 401.
  expect(await ensureAccessToken()).toBe(false);
  expect(refresh).not.toHaveBeenCalled();

  store.set("ucode.refresh", "r");
  expect(await ensureAccessToken()).toBe(true);
  expect(refresh).toHaveBeenCalledTimes(1);

  // Access уже в памяти — второй раз обновлять нечего.
  expect(await ensureAccessToken()).toBe(true);
  expect(refresh).toHaveBeenCalledTimes(1);
});
