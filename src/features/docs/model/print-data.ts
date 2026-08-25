/**
 * Что из строки уезжает в печатную форму.
 *
 * Почти всё — но не ключи, на которых печать падает целиком. Шлюз
 * берёт из присланных данных КАЖДЫЙ ключ, в котором есть «_id»,
 * отрезает у него этот суффикс и лезет за строкой в таблицу с таким
 * именем: `additional_fields` (`docx_template.go:830`) →
 * `GetAllForDocx` → `GetItem(ctx, conn, TrimSuffix(key, "_id"), value)`
 * (`storage/postgres/docx.go:474`). Промах — это не пустая переменная
 * в документе, а ошибка SQL, из которой получается 500 на весь запрос:
 *
 *   clients_ids  Many2Many: таблицы «clients_ids» не существует
 *   client_id ∅  незаполненная связь: `WHERE guid = ''` — invalid uuid
 *   passport_id  обычное текстовое поле: таблицы «passport» нет
 *
 * Первые два случая — не экзотика: Many2Many есть у половины таблиц,
 * а незаполненная связь бывает в любой строке. То есть печать сегодня
 * ломается чаще, чем работает.
 *
 * Поэтому ключ с «_id» уезжает, только если это настоящая связь
 * Many2One (поле LOOKUP) и в ней лежит uuid. Значения остальных таких
 * полей в документ не попадут — зато попадёт сам документ. Ошибка
 * записана в docs/backend-notes.md.
 */
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function printableRow(
  row: Record<string, unknown>,
  /** Слаги полей LOOKUP: по ним шлюз дочитает связанную строку. */
  lookups: ReadonlySet<string>,
): Record<string, unknown> {
  const data: Record<string, unknown> = {};

  for (const [key, value] of Object.entries(row)) {
    if (!key.includes("_id")) {
      data[key] = value;
      continue;
    }

    // Связь, которая заполнена, — единственный ключ с «_id», за которым
    // шлюз найдёт таблицу и строку. Она же и полезна: связанная запись
    // приезжает в шаблон целиком, как `{<слаг>_id_data.<поле>}`.
    if (lookups.has(key) && typeof value === "string" && UUID.test(value)) {
      data[key] = value;
    }
  }

  return data;
}
