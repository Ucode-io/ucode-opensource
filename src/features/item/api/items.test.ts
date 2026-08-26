import { expect, test } from "vitest";
import { nextPage, patchRow, toPage, toPages } from "./items";

const page = {
  data: { count: 2, response: [{ guid: "a", title: "раз" }, { guid: "b", title: "два" }] },
};

test("правка меняет одну строку и только присланные поля", () => {
  const next = patchRow(page, { guid: "b", values: { title: "три" } });

  expect(toPage(next as typeof page).rows).toEqual([
    { guid: "a", title: "раз" },
    { guid: "b", title: "три" },
  ]);
  // Исходный ответ не тронут: он же лежит в снимке для отката.
  expect(page.data.response[1]!.title).toBe("два");
});

test("перенос карточки правит два поля разом", () => {
  // Колонка доски и порядок в ней уезжают одним PUT — см. RowEdit.
  const next = patchRow(page, { guid: "a", values: { title: "раз", board_order: 3 } });

  expect(toPage(next as typeof page).rows[0]).toEqual({
    guid: "a",
    title: "раз",
    board_order: 3,
  });
});

test("чужая форма ответа проходит насквозь", () => {
  // Под ключом items лежат и одиночные записи, и ответы других ручек.
  const single = { data: { guid: "a" } };

  expect(patchRow(single, { guid: "a", values: { title: "x" } })).toBe(single);
  expect(patchRow(undefined, { guid: "a", values: { title: "x" } })).toBeUndefined();
});

test("строки нет на этой странице — ответ не меняется", () => {
  expect(patchRow(page, { guid: "нет такой", values: { title: "x" } })).toBe(page);
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

test("строка, приехавшая в двух кусках, в списке одна", () => {
  // Куски приходят внахлёст: см. toPages и docs/backend-notes.md.
  const first = { data: { count: 3, response: [{ guid: "a" }, { guid: "b" }] } };
  const second = { data: { count: 3, response: [{ guid: "b" }, { guid: "c" }] } };

  expect(toPages({ pages: [first, second] }).rows).toEqual([
    { guid: "a" },
    { guid: "b" },
    { guid: "c" },
  ]);
});

/*
 * В кэше лежат куски бесконечного запроса, а не один ответ: useItems —
 * всегда useInfiniteQuery. Правка, не умеющая их разбирать, просто
 * не находит строку, и оптимистичного обновления нет вовсе.
 */
test("правка доходит до строки внутри кусков бесконечного запроса", () => {
  const cached = {
    pageParams: [0, 1],
    pages: [
      { data: { count: 3, response: [{ guid: "a", title: "раз" }] } },
      { data: { count: 3, response: [{ guid: "b", title: "два" }] } },
    ],
  };

  const next = patchRow(cached, { guid: "b", values: { title: "три" } }) as typeof cached;

  expect(next.pages[1]?.data.response[0]).toEqual({ guid: "b", title: "три" });
  // Кусок без правки — та же ссылка: перерисовывать его незачем.
  expect(next.pages[0]).toBe(cached.pages[0]);
  expect(next.pageParams).toEqual([0, 1]);
});

test("строки нет ни в одном куске — кэш не трогается вовсе", () => {
  const cached = { pageParams: [0], pages: [{ data: { count: 1, response: [{ guid: "a" }] } }] };

  expect(patchRow(cached, { guid: "нет такой", values: { title: "x" } })).toBe(cached);
});
