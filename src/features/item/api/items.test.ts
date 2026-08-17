import { expect, test } from "vitest";
import { patchRow, toPage } from "./items";

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
