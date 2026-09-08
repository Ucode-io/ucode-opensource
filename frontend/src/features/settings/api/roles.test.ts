import { expect, test } from "vitest";
import { toRoles } from "./roles";

/*
 * Ответ ручки ролей завёрнут дважды: общий конверт снимает http-клиент,
 * а под ним лежит `{table_slug, data: {count, response}}` — как у строк
 * таблицы. Пока читали несуществующий ключ `roles`, экран прав честно
 * писал «у проекта нет ролей» на проекте с двадцатью ролями.
 */
test("роли читаются из data.response, а не из корня ответа", () => {
  const body = {
    table_slug: "role",
    data: {
      count: 2,
      response: [
        { guid: "r1", name: " Админ ", is_system: false, status: true, client_type_id: "c1" },
        { guid: "r2", name: "", is_system: true },
        // Без guid роль не открыть — такие пропускаем.
        { name: "Без идентификатора" },
      ],
    },
  };

  expect(toRoles(body)).toEqual([
    { id: "r1", name: "Админ", isSystem: false, clientTypeId: "c1" },
    { id: "r2", name: "—", isSystem: true, clientTypeId: "" },
  ]);
});

test("пустой ответ не роняет экран", () => {
  expect(toRoles({})).toEqual([]);
  expect(toRoles({ data: { count: 0 } })).toEqual([]);
});
