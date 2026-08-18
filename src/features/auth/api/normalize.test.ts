import { expect, test } from "vitest";
import { toConnection, toRecoveryStart, toSession } from "./normalize";

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

/*
 * Восстановление пароля: у первого шага три исхода, и все три приходят
 * со статусом 200. Отличить их можно только по паре полей, поэтому
 * разбор здесь, а не в форме.
 */
test("первый шаг восстановления: код ушёл, почты нет, логина нет", () => {
  expect(
    toRecoveryStart({ user_id: "u1", email_found: true, sms_id: "s1", email: "a@b.c" }),
  ).toEqual({ kind: "sent", userId: "u1", smsId: "s1", email: "a@b.c" });

  // Логин нашли, почты у пользователя нет — её сначала спрашивают.
  expect(toRecoveryStart({ user_id: "u1", email_found: false })).toEqual({
    kind: "noEmail",
    userId: "u1",
  });

  // Пустой user_id — такого логина нет; ошибки бэкенд при этом не отдаёт.
  expect(toRecoveryStart({ email_found: false })).toEqual({ kind: "unknown" });
});
