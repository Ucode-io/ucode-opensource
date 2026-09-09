/**
 * Разбор полей журнала изменений.
 *
 * Бэкенд кладёт в `request`, `response`, `previous` и `current` СТРОКУ
 * с JSON, и каждый раз завёрнутую: `{"data": …}` (шлюз, handler.go:192
 * — он оборачивает всё, что пишет в журнал). Внутри лежит либо объект,
 * либо строка с текстом ошибки, либо пустая карта.
 *
 * Поэтому разбор один и здесь: экран показывает содержимое, а не
 * конверт, и не должен решать, какой из четырёх видов ему достался.
 */

/** Содержимое поля журнала: конверт снят, JSON разложен по строкам. */
export function unwrapEntry(raw: string): string {
  const value = raw?.trim();
  if (!value) return "";

  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch {
    // Не JSON — значит текст. Показываем как есть: чаще всего это
    // сообщение об ошибке, ради которого запись и открыли.
    return value;
  }

  const inner =
    typeof parsed === "object" && parsed !== null && "data" in parsed
      ? (parsed as { data: unknown }).data
      : parsed;

  if (inner === null || inner === undefined) return "";
  if (typeof inner === "string") return inner;

  // Пустая карта — это «поля не было», а не «объект без ключей»:
  // шлюз подставляет её вместо отсутствующего значения.
  if (isEmptyObject(inner)) return "";

  return JSON.stringify(inner, null, 2);
}

function isEmptyObject(value: unknown): boolean {
  return (
    typeof value === "object" &&
    value !== null &&
    !Array.isArray(value) &&
    Object.keys(value).length === 0
  );
}
