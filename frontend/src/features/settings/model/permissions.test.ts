import { expect, test } from "vitest";
import {
  toRolePermissions,
  toggleColumn,
  toggleFieldRight,
  toggleGlobalRight,
  toggleRight,
} from "./permissions";

/*
 * Права приходят строками 'Yes'/'No'. Ошибка здесь тихая: строка "No"
 * в булевом контексте истинна, и роль без единого права выглядела бы
 * ролью со всеми.
 */
/*
 * Форма ответа взята с живого бэкенда: под общим конвертом лежит ещё
 * `{project_id, data: {...}}`, имя и id роли — прямо в этом `data`,
 * а не в отдельном `role`. Пока читали `role.guid`, экран прав показывал
 * пустой список на роли со ста пятьюдесятью таблицами.
 */
const body = () => ({
  project_id: "p1",
  data: {
  guid: "r1",
  name: " Менеджер ",
  global_permission: { id: "g1", billing: true, sms_button: false },
  tables: [
    {
      id: "t2",
      slug: "orders",
      label: "Заказы",
      record_permissions: { read: "Yes", write: "No", update: "Yes", delete: "No" },
      custom_permission: { settings: "No", columns: "Yes" },
      field_permissions: [
        { field_id: "f1", label: "Номер", view_permission: true, edit_permission: false },
        { field_id: "f2", label: "Клиент", view_permission: true, edit_permission: true },
      ],
    },
    {
      id: "t1",
      slug: "clients",
      label: "Клиенты",
      record_permissions: {},
      custom_permission: {},
    },
  ],
  },
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
  expect(after.raw["global_permission"]).toMatchObject({ id: "g1" });
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

test("глобальные права булевы и живут в своём мешке", () => {
  const before = toRolePermissions(body());

  expect(before.global.billing).toBe(true);
  expect(before.global.sms_button).toBe(false);
  // Ключа нет — права нет: `false` бэкенд режет omitempty, и так же
  // читает эти права features/auth (галка не должна врать про кнопку).
  expect(before.global.menu_button).toBe(false);
  // Право на помощника — `chat`, его читает features/copilot.
  expect(before.global.chat).toBe(false);

  const after = toggleGlobalRight(before, "billing", false);

  expect(after.global.billing).toBe(false);
  // Булево, а не "No": у global_permission своя таблица и своя форма.
  expect(after.raw["global_permission"]).toMatchObject({ id: "g1", billing: false });
  // Права на таблицы не тронуты.
  expect(after.tables).toBe(before.tables);
});

test("право раздаётся всем показанным таблицам разом", () => {
  const before = toRolePermissions(body());
  const after = toggleColumn(before, ["orders", "clients"], "delete", true);

  expect(after.tables.map((table) => table.record.delete)).toEqual([true, true]);

  const tables = after.raw["tables"] as { record_permissions: Record<string, string> }[];
  expect(tables.map((table) => table.record_permissions["delete"])).toEqual(["Yes", "Yes"]);

  // Не показанной таблицы правка не касается: у «клиентов» право
  // остаётся тем, чем было (пустые права читаются как разрешение).
  const narrow = toggleColumn(before, ["orders"], "read", false);
  expect(narrow.tables.find((table) => table.slug === "orders")?.record.read).toBe(false);
  expect(narrow.tables.find((table) => table.slug === "clients")?.record.read).toBe(true);
});

test("права, которых нет в матрице, правятся тем же способом", () => {
  const after = toggleRight(toRolePermissions(body()), "orders", "pdf_action", false);

  expect(after.tables[0]?.other.pdf_action).toBe(false);

  const tables = after.raw["tables"] as { custom_permission: Record<string, string> }[];
  expect(tables[0]?.custom_permission).toMatchObject({ pdf_action: "No", settings: "No" });
});

test("право на поле правится и в списке, и в теле запроса", () => {
  const before = toRolePermissions(body());
  const field = before.tables[0]?.fields[0];

  expect(field).toMatchObject({ id: "f1", view: true, edit: false });

  const after = toggleFieldRight(before, "orders", "f1", "edit", true);
  expect(after.tables[0]?.fields[0]?.edit).toBe(true);

  const tables = after.raw["tables"] as {
    field_permissions: { field_id: string; edit_permission: boolean }[];
  }[];
  expect(tables[0]?.field_permissions[0]).toMatchObject({ field_id: "f1", edit_permission: true });

  // Второе поле не тронуто: PUT перезапишет права целиком, и потерянное
  // ограничение никто не заметит.
  expect(tables[0]?.field_permissions[1]).toMatchObject({ field_id: "f2" });
});
