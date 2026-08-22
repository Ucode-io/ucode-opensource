import { expect, test } from "vitest";
import { entryUrl, remoteName } from "./remote";

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
