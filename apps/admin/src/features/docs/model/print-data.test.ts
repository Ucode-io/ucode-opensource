import { expect, test } from "vitest";
import { printableRow } from "./print-data";

const GUID = "3f1c9b0e-2a4d-4c6b-9f1a-2e5d7c8b9a01";

test("в шаблон уезжает всё, кроме ключей, на которых печать падает", () => {
  const data = printableRow(
    {
      guid: GUID,
      name: "Договор",
      total: 1200,
      // Заполненная связь: шлюз дочитает по ней строку таблицы «client».
      client_id: GUID,
      // Незаполненная: `WHERE guid = ''` — invalid uuid, 500 на весь запрос.
      manager_id: null,
      // Many2Many: таблицы «orders_ids» не существует.
      orders_ids: [GUID],
      // Обычное текстовое поле, в слаге которого просто есть «_id».
      passport_id: "AA1234567",
    },
    new Set(["client_id", "manager_id"]),
  );

  expect(data).toEqual({
    guid: GUID,
    name: "Договор",
    total: 1200,
    client_id: GUID,
  });
});

test("поле не-связь с uuid внутри тоже не уезжает", () => {
  // Таблицы «external» может не быть, а промах стоит всего документа —
  // поэтому решает не вид значения, а тип поля.
  expect(printableRow({ external_id: GUID }, new Set())).toEqual({});
});
