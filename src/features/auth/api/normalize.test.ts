import { expect, test } from "vitest";
import { toConnection, toSession } from "./normalize";

test("сессия собирается из вложенных полей ответа", () => {
  const session = toSession({
    user_id: "u1",
    environment_id: "e1",
    login_table_slug: "person",
    role: { id: "r1" },
    client_type: { id: "ct1" },
    permissions: [{ table_slug: "orders", read: "Yes", write: "No", settings: "No" }],
  });

  expect(session).toEqual({
    userId: "u1",
    projectId: "",
    environmentId: "e1",
    roleId: "r1",
    clientTypeId: "ct1",
    loginTableSlug: "person",
    permissions: [
      {
        tableSlug: "orders",
        read: true,
        // Строка "No" — это запрет; в булевом контексте она истинна,
        // и без перевода запрет читался бы как разрешение.
        write: false,
        settings: false,
        // Колонки, которых в ответе нет: настройку не трогали, а не «нельзя».
        update: true,
        delete: true,
        columns: true,
        fixColumn: true,
        excelMenu: true,
        viewCreate: true,
        addField: true,
        group: true,
        tabGroup: true,
      },
    ],
  });
});

test("пустой ответ не роняет нормализацию", () => {
  const session = toSession({});

  expect(session.userId).toBe("");
  expect(session.permissions).toEqual([]);
});

test("подпись опции берётся из поля, названного в view_slug", () => {
  const connection = toConnection({
    guid: "c1",
    table_slug: "branch",
    view_slug: "name",
    options: [
      { guid: "o1", name: "Чиланзар" },
      { guid: "o2", name: "Юнусабад" },
    ],
  });

  expect(connection).toEqual({
    id: "c1",
    tableSlug: "branch",
    options: [
      { id: "o1", label: "Чиланзар" },
      { id: "o2", label: "Юнусабад" },
    ],
  });
});

test("без подписи опция показывает свой id, а не пустоту", () => {
  const connection = toConnection({
    guid: "c1",
    view_slug: "title",
    options: [{ guid: "o1" }, { guid: "o2", title: "" }],
  });

  expect(connection.options).toEqual([
    { id: "o1", label: "o1" },
    { id: "o2", label: "o2" },
  ]);
});
