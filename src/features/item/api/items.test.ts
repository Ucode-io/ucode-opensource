import { expect, test } from "vitest";
import { nextPage, patchRow, toPage, toPages } from "./items";

const page = {
  data: { count: 2, response: [{ guid: "a", title: "раз" }, { guid: "b", title: "два" }] },
};

test("правка меняет одну строку и одно поле", () => {
  const next = patchRow(page, { guid: "b", slug: "title", value: "три" });

  expect(toPage(next as typeof page).rows).toEqual([
    { guid: "a", title: "раз" },
    { guid: "b", title: "три" },
  ]);
  // Исходный ответ не тронут: он же лежит в снимке для отката.
  expect(page.data.response[1]!.title).toBe("два");
});

test("чужая форма ответа проходит насквозь", () => {
  // Под ключом items лежат и одиночные записи, и ответы других ручек.
  const single = { data: { guid: "a" } };

  expect(patchRow(single, { guid: "a", slug: "title", value: "x" })).toBe(single);
  expect(patchRow(undefined, { guid: "a", slug: "title", value: "x" })).toBeUndefined();
});

test("строки нет на этой странице — ответ не меняется", () => {
  expect(patchRow(page, { guid: "нет такой", slug: "title", value: "x" })).toBe(page);
});

test("следующий кусок заказывается, пока строк меньше, чем всего", () => {
  const page = (rows: number, count: number) => ({
    data: { count, response: Array.from({ length: rows }, () => ({ guid: "x" })) },
  });

  // Загружено 20 из 45 — есть что грузить, и номер следующего куска
  // равен числу уже загруженных.
  expect(nextPage([page(20, 45)])).toBe(1);
  expect(nextPage([page(20, 45), page(20, 45)])).toBe(2);

  // Загружено всё.
  expect(nextPage([page(20, 45), page(20, 45), page(5, 45)])).toBeUndefined();

  /*
   * Счётчик врёт (строки удаляют прямо сейчас), а кусок приехал пустым —
   * останавливаемся, иначе запросы шли бы до конца страницы.
   */
  expect(nextPage([page(20, 999), page(0, 999)])).toBeUndefined();
  expect(nextPage([])).toBeUndefined();
});

test("страницы склеиваются в один список, счётчик берётся у последней", () => {
  const first = { data: { count: 100, response: [{ guid: "a" }] } };
  // Пока листали, строк стало меньше — верим свежему числу.
  const second = { data: { count: 98, response: [{ guid: "b" }] } };

  expect(toPages({ pages: [first, second] })).toEqual({
    rows: [{ guid: "a" }, { guid: "b" }],
    count: 98,
  });
});
