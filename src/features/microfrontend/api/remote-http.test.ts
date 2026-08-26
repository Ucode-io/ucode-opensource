import { expect, test, vi } from "vitest";

/*
 * Адреса и localStorage читаются при загрузке модуля, поэтому подмена
 * идёт ДО импорта — как в shared/api/client.test.
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

const { remoteHttp, remoteHttpV2 } = await import("./remote-http");
const { session } = await import("@/shared/api/session");

/*
 * Здесь проверяется чужой контракт — тот, под который написаны ремоуты
 * ucode (`utils/httpsRequest.js` старой админки). Разойдётся хоть одно
 * из трёх — ремоут покажет «Failed to load data», и по нему не будет
 * видно, что именно сломалось.
 */

test("V2 — тот же admin-хост с /v2, а не сервис авторизации", () => {
  // Ровно эта ошибка и была: ремоуту отдавали httpAuth, и запросы
  // данных уходили на api.auth.
  expect(remoteHttp.defaults.baseURL).toBe("https://api.test");
  expect(remoteHttpV2.defaults.baseURL).toBe("https://api.test/v2");
});

test("наружу уходит полезная часть, внутрь — токен, окружение и project-id", async () => {
  session.set({ access: "TOKEN", refresh: "R", environmentId: "ENV", projectId: "PRJ" });

  let sent: { params?: Record<string, unknown>; headers?: { get(name: string): unknown } } = {};

  remoteHttpV2.defaults.adapter = (config) => {
    sent = config as typeof sent;
    return Promise.resolve({
      // Конверт бэкенда: полезная часть лежит в data.
      data: { status: "done", description: "", data: [{ id: 1 }] },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    });
  };

  const rows = await remoteHttpV2.get("/items");

  // Ремоут пишет `const rows = await sharedHttpRequestV2.get(...)` —
  // без `.data`, потому что старый перехватчик отдавал уже развёрнутое.
  expect(rows).toEqual([{ id: 1 }]);
  expect(sent.params?.["project-id"]).toBe("PRJ");
  expect(sent.headers?.get("Authorization")).toBe("Bearer TOKEN");
  expect(sent.headers?.get("Environment-Id")).toBe("ENV");
});
