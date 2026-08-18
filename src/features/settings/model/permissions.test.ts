import { expect, test } from "vitest";
import { toRolePermissions, toggleRight } from "./permissions";

/*
 * Права приходят строками 'Yes'/'No'. Ошибка здесь тихая: строка "No"
 * в булевом контексте истинна, и роль без единого права выглядела бы
 * ролью со всеми.
 */
const body = () => ({
  role: { guid: "r1", name: " Менеджер " },
  global_permission: { guid: "g1" },
  tables: [
    {
      id: "t2",
      slug: "orders",
      label: "Заказы",
      record_permissions: { read: "Yes", write: "No", update: "Yes", delete: "No" },
      custom_permission: { settings: "No", columns: "Yes" },
      field_permissions: [{ guid: "fp1" }],
    },
    {
      id: "t1",
      slug: "clients",
      label: "Клиенты",
      record_permissions: {},
      custom_permission: {},
    },
  ],
});

test("права разбираются в булевы, а таблицы идут по имени", () => {
  const permissions = toRolePermissions(body());

  expect(permissions.roleName).toBe("Менеджер");
  expect(permissions.tables.map((table) => table.slug)).toEqual(["orders", "clients"]);

  expect(permissions.tables[0]?.record).toEqual({
    read: true,
    write: false,
    update: true,
    delete: false,
  });
  expect(permissions.tables[0]?.screen).toMatchObject({ settings: false, columns: true });

  // Ключа нет вовсе — право есть: бэкенд подставляет 'Yes' у всего,
  // чего в record_permission не оказалось.
  expect(permissions.tables[1]?.record.read).toBe(true);
});

test("переключение правит и показанное, и то, что уедет на сервер", () => {
  const before = toRolePermissions(body());
  const after = toggleRight(before, "orders", "write", true);

  expect(after.tables[0]?.record.write).toBe(true);

  const tables = after.raw["tables"] as { slug: string; record_permissions: unknown }[];
  expect(tables[0]?.record_permissions).toMatchObject({ write: "Yes", read: "Yes" });

  /*
   * Всё, чего экран не показывает, обязано уехать обратно нетронутым:
   * PUT перезаписывает права роли целиком и без global_permission
   * отвечает отказом.
   */
  expect(after.raw["global_permission"]).toEqual({ guid: "g1" });
  expect(tables[0]).toHaveProperty("field_permissions");

  // Соседняя таблица не тронута.
  expect(after.tables[1]).toBe(before.tables[1]);
});

test("экранное право пишется в свой мешок, а не в права на строки", () => {
  const after = toggleRight(toRolePermissions(body()), "orders", "settings", true);
  const tables = after.raw["tables"] as {
    record_permissions: Record<string, unknown>;
    custom_permission: Record<string, unknown>;
  }[];

  expect(tables[0]?.custom_permission).toMatchObject({ settings: "Yes" });
  expect(tables[0]?.record_permissions).not.toHaveProperty("settings");
});
