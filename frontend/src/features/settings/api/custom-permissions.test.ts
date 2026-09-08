import { expect, test } from "vitest";
import { toCustomPermission } from "./custom-permissions";

/*
 * Право приезжает строкой «Yes»/«No» — так его хранит колонка с CHECK
 * (`000046_create_custom_permission.up.sql:16`). Любая другая строка
 * (и отсутствие поля) — это «нет»: галка по истинности непустой строки
 * поставила бы права там, где их не давали.
 */
test("права читаются как «Yes», а не как непустая строка", () => {
  const permission = toCustomPermission({
    custom_permission_id: "p1",
    title: "Утверждать счета",
    read: "Yes",
    write: "No",
    update: "",
    delete: "yes",
  });

  expect(permission.read).toBe(true);
  expect(permission.write).toBe(false);
  expect(permission.update).toBe(false);
  // Регистр не наш: колонка допускает ровно «Yes» и «No».
  expect(permission.delete).toBe(false);
});

test("пояснение берётся из attributes, а мусор в нём не рисуется", () => {
  expect(toCustomPermission({ custom_permission_id: "p1", attributes: { description: " Склад " } }).description).toBe(
    "Склад",
  );
  expect(toCustomPermission({ custom_permission_id: "p1", attributes: null }).description).toBe("");
  expect(toCustomPermission({ custom_permission_id: "p1", attributes: { description: 42 } }).description).toBe("");
});
