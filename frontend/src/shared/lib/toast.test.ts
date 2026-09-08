import { expect, test, vi } from "vitest";

vi.stubEnv("VITE_API_URL", "https://api.test");
vi.stubEnv("VITE_AUTH_URL", "https://auth.test");

const store = new Map<string, string>();
vi.stubGlobal("localStorage", {
  getItem: (key: string) => store.get(key) ?? null,
  setItem: (key: string, value: string) => void store.set(key, value),
  removeItem: (key: string) => void store.delete(key),
  clear: () => store.clear(),
});

// Клиент читает адреса при загрузке модуля — импорт после подмены (как в client.test).
const { ApiError } = await import("../api/client");
const { errorMessage } = await import("./toast");

test("причина отказа для экрана: сначала слова сервера, потом свои", () => {
  // Ошибки нет — и текста нет: экран проверяет именно null.
  expect(errorMessage(null, "table.loadFailed")).toBe(null);
  expect(errorMessage(undefined, "table.loadFailed")).toBe(null);

  // Сервер объяснил — показываем его объяснение, а не свою догадку.
  expect(errorMessage(new ApiError(403, { message: "permission denied" }), "table.loadFailed")).toBe(
    "permission denied",
  );

  // Сервер смолчал (или упало не в нём) — общая формулировка на языке
  // интерфейса. Язык по умолчанию — русский, см. shared/lib/i18n.
  expect(errorMessage(new ApiError(500, {}), "table.loadFailed")).toBe(
    "Не удалось загрузить строки",
  );
  expect(errorMessage(new Error("network"), "table.loadFailed")).toBe(
    "Не удалось загрузить строки",
  );
});
