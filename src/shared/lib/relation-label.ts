/**
 * Подпись связанной строки: значения полей показа через пробел.
 *
 * Живёт в shared, потому что читают её обе стороны и по одному правилу:
 * ячейка-связь в таблице (features/item) и выбор строк в условии
 * агрегата (features/table). Держать две копии значило бы однажды
 * показать одну и ту же запись по-разному на двух экранах.
 *
 * Поля показа задаёт админ в настройках связи. Не заданы — подписи нет,
 * и подставлять `title` или `name` нельзя: это угадывание, а не чтение
 * (см. CONTEXT, Relation).
 */
export function relationLabel(item: Record<string, unknown>, slugs: string[]): string {
  return slugs
    .map((slug) => item[slug])
    .filter((part) => !isEmpty(part) && typeof part !== "object")
    .join(" ");
}

function isEmpty(value: unknown): boolean {
  return value === null || value === undefined || value === "";
}
