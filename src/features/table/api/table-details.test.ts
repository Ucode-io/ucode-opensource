import { expect, test } from "vitest";
import { toTableDetails } from "./table-details";

/*
 * Ответ вложен дважды: клиент снимает общий конверт, а под ним лежит
 * ещё один `data`. Разбор по плоскому `fields` молча дал бы пустой
 * список — и меню поиска выглядело бы пустым всегда.
 */
test("поля и флаги берутся из вложенного data", () => {
  const { fields, enabled } = toTableDetails({
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
  expect(toTableDetails({}).fields).toEqual([]);
});

test("права роли на view: запрет строгий, разрешение по умолчанию", () => {
  const { viewRights } = toTableDetails({
    data: {
      views: [
        { id: "v1", attributes: { view_permission: { view: false, edit: true, delete: true } } },
        { id: "v2", attributes: { view_permission: { view: true, edit: false, delete: false } } },
        // Записи прав нет: роль завели уже после view — запрещённым
        // это не считается (её и завести-то через настройки нечем).
        { id: "v3", attributes: {} },
      ],
    },
  });

  expect(viewRights.get("v1")).toEqual({ view: false, edit: true, delete: true });
  expect(viewRights.get("v2")).toEqual({ view: true, edit: false, delete: false });
  expect(viewRights.has("v3")).toBe(false);
});
