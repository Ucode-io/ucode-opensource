import { expect, test } from "vitest";
import { toSearchFields } from "./search-fields";

/*
 * Ответ вложен дважды: клиент снимает общий конверт, а под ним лежит
 * ещё один `data`. Разбор по плоскому `fields` молча дал бы пустой
 * список — и меню поиска выглядело бы пустым всегда.
 */
test("поля и флаги берутся из вложенного data", () => {
  const { fields, enabled } = toSearchFields({
    table_slug: "products",
    data: {
      fields: [
        { id: "1", slug: "name", type: "SINGLE_LINE", is_search: true },
        { id: "2", slug: "note", type: "SINGLE_LINE" },
        { id: "3", slug: "code", type: "SINGLE_LINE", is_search: false },
      ],
    },
  });

  expect(fields.map((field) => field.slug)).toEqual(["name", "note", "code"]);
  expect([...enabled]).toEqual(["1"]);
});

test("пустой ответ — пустой список, а не падение", () => {
  expect(toSearchFields({}).fields).toEqual([]);
});
