import { expect, test } from "vitest";
import { toTemplates } from "./templates";

test("ответ читается и списком, и объектом со списком", () => {
  // Две ветки шлюза отвечают по-разному, а экран у них один.
  expect(toTemplates([{ id: "1", name: "CRM" }])).toHaveLength(1);
  expect(toTemplates({ templates: [{ id: "1", name: "CRM" }] })).toHaveLength(1);
  expect(toTemplates(undefined)).toEqual([]);
});

test("шаблон без имени подписан своим id, а не пустотой", () => {
  expect(toTemplates([{ id: "7f3" }])[0]?.name).toBe("7f3");
});

test("содержимое шаблона не разбирается, а переносится как есть", () => {
  // Разбирать его — значит держать у себя копию модели всего
  // конструктора; она устареет на первой же правке бэкенда.
  const tables = [{ slug: "orders", fields: [{ slug: "guid" }] }];

  expect(toTemplates([{ id: "1", tables }])[0]?.tables).toBe(tables);
});

test("шаблон без содержимого не роняет экран", () => {
  expect(toTemplates([{ id: "1" }])[0]?.tables).toEqual([]);
});
