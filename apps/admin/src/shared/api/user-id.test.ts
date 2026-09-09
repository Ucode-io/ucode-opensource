import { afterEach, expect, test, vi } from "vitest";

vi.stubEnv("VITE_API_URL", "https://api.test");
vi.stubEnv("VITE_AUTH_URL", "https://auth.test");

const store = new Map<string, string>();
vi.stubGlobal("localStorage", {
  getItem: (k: string) => store.get(k) ?? null,
  setItem: (k: string, v: string) => void store.set(k, v),
  removeItem: (k: string) => void store.delete(k),
});

const { session } = await import("./session");

afterEach(() => {
  store.clear();
  session.clear();
});

/** Собирает токен с указанными claim'ами. Подпись здесь не важна. */
function token(claims: Record<string, unknown>): string {
  const payload = btoa(JSON.stringify(claims)).replace(/\+/g, "-").replace(/\//g, "_");
  return `header.${payload}.signature`;
}

test("сохранённый id пользователя возвращается как есть", () => {
  session.set({ access: "a", refresh: "r", userId: "da2aa13a-698b-40ad-9c39-420eb2a92c1c" });

  expect(session.getUserId()).toBe("da2aa13a-698b-40ad-9c39-420eb2a92c1c");
});

test("если id не сохранён, он берётся из токена", () => {
  // Иначе уже открытые сессии остались бы без списка компаний
  // до повторного входа.
  session.set({ access: token({ user_id: "from-token" }), refresh: "r" });

  expect(session.getUserId()).toBe("from-token");
});

test("испорченный токен не роняет чтение", () => {
  session.set({ access: "не-жвт", refresh: "r" });

  expect(session.getUserId()).toBe("");
});

test("без токена и без сохранённого id — пусто, а не исключение", () => {
  expect(session.getUserId()).toBe("");
});
