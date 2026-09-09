import { beforeEach, expect, test, vi } from "vitest";
import { entryUrl, loadRemotePage, remoteName, shareWithRemotes } from "./remote";

/*
 * Подменяем рантайм федерации целиком: проверяем не его, а то, ЧТО
 * и в каком порядке мы у него просим. Строка `имя/модуль` — самое
 * хрупкое место загрузчика: ошибка в ней выглядит как «ремоут не
 * отдаёт страницу», а не как опечатка.
 */
const requested: string[] = [];
let exposes: Record<string, unknown> = {};

vi.mock("@module-federation/runtime", () => ({
  init: () => undefined,
  registerRemotes: () => undefined,
  loadRemote: (id: string) => {
    requested.push(id);
    const expose = id.slice(id.indexOf("/") + 1);

    // Ровно так отвечает @originjs: неизвестный модуль — исключение.
    return expose in exposes
      ? Promise.resolve(exposes[expose])
      : Promise.reject(new Error(`Can not find remote module ./${expose}`));
  },
}));

vi.mock("react18/react", () => ({ default: { createElement: () => undefined, version: "18.3.1" } }));
vi.mock("react18/dom", () => ({ default: {} }));
vi.mock("react18/client", () => ({ default: { createRoot: () => ({}) } }));

vi.stubGlobal("localStorage", { getItem: () => null, setItem: () => {}, removeItem: () => {} });

beforeEach(() => {
  requested.length = 0;
  exposes = {};
});

/** Так общую область читает сам ремоут — см. комментарий в тесте ниже. */
function readShared(name: string) {
  const shared = globalThis as {
    __federation_shared__?: {
      default?: Record<string, Record<string, { get: () => () => unknown }>>;
    };
  };
  return shared.__federation_shared__?.default?.[name] ?? {};
}

test("общая область федерации читается ровно так, как её читает ремоут", async () => {
  const react = { useEffect: () => {} };
  shareWithRemotes("react", react, "18.3.1");

  /*
   * Это и есть код потребителя @originjs, слово в слово
   * (`vite-plugin-federation/dist/index.js:1848`): первая версия из
   * объекта, `get()`, и вызов того, что вернулось. Разойдётся форма —
   * ремоут молча возьмёт свою копию React, а мы получим пустой
   * диспетчер хуков вместо экрана.
   */
  const versions = readShared("react");
  expect(Object.keys(versions)).toEqual(["18.3.1"]);

  const factory = await Object.values(versions)[0]?.get();
  expect(factory?.()).toBe(react);
});

test("два поколения лежат рядом, и 18-я всегда первая", async () => {
  /*
   * Область одна на всех, но версий в ней может быть несколько.
   * Старый ремоут не проставляет `requiredVersion` и берёт
   * `Object.keys(versionObj)[0]`; новый ищет подходящую перебором
   * (`@originjs 1.4.1/dist/index.js:956`). Значит первой обязана
   * лежать 18-я — иначе старому достанется React 19, и он упадёт
   * на элементах чужого поколения.
   *
   * Порядок не должен зависеть от того, какой микрофронтенд открыли
   * первым, — поэтому 19-ю кладём раньше 18-й именно здесь.
   */
  const react18 = { version: "18.3.1" };
  const react19 = { version: "19.2.8" };

  shareWithRemotes("react", react19, "19.2.8");
  shareWithRemotes("react", react18, "18.3.1");

  const versions = readShared("react");
  expect(Object.keys(versions)).toEqual(["18.3.1", "19.2.8"]);

  const forLegacy = await Object.values(versions)[0]?.get();
  expect(forLegacy?.()).toBe(react18);
});

test("адрес сборки собирается из голого хоста", () => {
  // В базе лежит хост без схемы — старая админка всюду клеит https://
  // (views/Microfrontend/index.jsx:30).
  expect(entryUrl("my-app.example.com")).toBe("https://my-app.example.com/assets/remoteEntry.js");
});

test("уже записанная схема не удваивается", () => {
  expect(entryUrl("https://my-app.example.com")).toBe(
    "https://my-app.example.com/assets/remoteEntry.js",
  );
  // http оставляем как есть: локальную сборку смотрят и без сертификата.
  expect(entryUrl("http://localhost:5001")).toBe("http://localhost:5001/assets/remoteEntry.js");
});

test("хвостовая косая черта не даёт двойного слэша", () => {
  expect(entryUrl("https://my-app.example.com/")).toBe(
    "https://my-app.example.com/assets/remoteEntry.js",
  );
});

test("пустой адрес остаётся пустым, а не превращается в https://", () => {
  // Иначе экран пошёл бы грузить https:///assets/remoteEntry.js
  // вместо того, чтобы сказать «адрес не задан».
  expect(entryUrl("")).toBe("");
  expect(entryUrl("   ")).toBe("");
});

test("имя ремоута годится для строки «имя/Page»", () => {
  // Косая черта в имени разорвала бы путь модуля, точка — сломала бы
  // разбор; от идентификатора остаются только буквы и цифры.
  expect(remoteName("3f2a1b4c-9d8e-4f10-a1b2-c3d4e5f60718")).toBe(
    "mf_3f2a1b4c9d8e4f10a1b2c3d4e5f60718",
  );
  expect(remoteName("a/b.c")).toBe("mf_abc");
});

test("у старого ремоута спрашиваем ./ucode, потом ./Page, потом ./App", async () => {
  // Нынешний шаблон ucode экспонирует ./App — три из четырёх живых
  // микрофронтендов собраны именно так, и без перебора они не
  // открываются вовсе.
  const Page = () => null;
  exposes = { App: Page };

  const loaded = await loadRemotePage("cd45af5a-7974", "app.example.com");

  expect(requested).toEqual(["mf_cd45af5a7974/ucode", "mf_cd45af5a7974/Page", "mf_cd45af5a7974/App"]);
  expect(loaded.page).toBe(Page);
  // Самоописания нет — значит старое поколение, свой корень React 18.
  expect(loaded.meta).toBeNull();
});

test("ремоут с самоописанием: ./App у него уже не спрашиваем", async () => {
  const Page = () => null;
  exposes = { ucode: { meta: { contract: 1, react: "19" } }, Page };

  const loaded = await loadRemotePage("cd45af5a-7974", "app.example.com");

  expect(requested).toEqual(["mf_cd45af5a7974/ucode", "mf_cd45af5a7974/Page"]);
  expect(loaded.meta).toEqual({ contract: 1, react: "19" });
});

test("договор новее известного — отказ, а не белый экран", async () => {
  exposes = { ucode: { meta: { contract: 99, react: "19" } }, Page: () => null };

  await expect(loadRemotePage("cd45af5a-7974", "app.example.com")).rejects.toThrow(
    /contract 99/,
  );
});
