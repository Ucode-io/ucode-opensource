/** Как показать ячейку: своим цветом, выключкой, лишь бы не «текст как текст». */
export type SqlCellKind = "null" | "boolean" | "number" | "uuid" | "date" | "text";

/**
 * Типы колонки (`types[column]`, `object_builder.go:3395-3400`), которые
 * стоит показывать не как обычный текст, а приглушённым — техническое
 * значение, не то, ради чего запрос писали.
 *
 * uuid отдельно от даты: это не «оба приглушённые», а два разных повода.
 * uuid мутится, потому что длинный и обычно не читают целиком — нужен
 * факт совпадения, не текст. Дата мутится по другой причине — она
 * второстепенна почти всегда (created_at рядом с настоящими данными
 * строки), и тот же приём уже стоит на датах в `ActivityLog`/`FunctionLogs`
 * (`Td className="text-fg-muted"`) — здесь та же дата, тот же смысл.
 */
const UUID_TYPES = new Set(["uuid"]);
const DATE_TYPES = new Set(["date", "time", "timestamp", "timestamptz", "interval"]);

/**
 * Значение ячейки ответа SQL — в строку плюс то, как её показать.
 *
 * Типов у postgres больше, чем у JSON, и по дороге они уже сжались:
 * бэкенд переводит время в RFC3339, uuid и bytea — в строки
 * (`object_builder.go:3432-3444`), остальное отдаёт как есть. Так что
 * сюда приезжают только числа, строки, булевы, null и вложенный JSON
 * (`json`/`jsonb` и массивы).
 *
 * `kind` для null/boolean/number берётся из типа значения в JS, а не
 * из колонки `types`: тип колонки — это то, что ХРАНИТ база («bool»),
 * а `typeof` — то, что реально ПРИЕХАЛО в этой строке. NULL из числовой
 * колонки — JS-значение `null`, и красить его как число значило бы
 * врать глазами раньше слов.
 *
 * uuid и дата — наоборот, только по `pgType`: у обоих значение в JS —
 * обычная строка, отличить которую от текста можно только по тому, что
 * САМА база назвала колонку. Регулярка на форму строки (тридцать шесть
 * символов и тире) угадывала бы через раз — `text`-колонка с похожим
 * значением получила бы чужую отметку.
 *
 * `null` возвращается отдельно от текста, а не строкой «null»: в колонке
 * с текстом настоящее слово «null» не отличить от пустоты, а разница
 * между ними — обычная причина лезть в консоль.
 */
export function sqlCell(value: unknown, pgType = ""): { text: string; kind: SqlCellKind } {
  if (value === null || value === undefined) return { text: "null", kind: "null" };
  if (typeof value === "boolean") return { text: String(value), kind: "boolean" };
  if (typeof value === "number") return { text: String(value), kind: "number" };
  if (typeof value === "object") return { text: JSON.stringify(value), kind: "text" };

  if (UUID_TYPES.has(pgType)) return { text: String(value), kind: "uuid" };
  if (DATE_TYPES.has(pgType)) return { text: String(value), kind: "date" };

  return { text: String(value), kind: "text" };
}
