import type { Item } from "./types";

/**
 * Группировка строк по одному полю — v1.1 из плана.
 *
 * Группирует КЛИЕНТ, но по уже отсортированному СЕРВЕРОМ списку: запрос
 * ставит поле группы первой сортировкой, и одинаковые значения приходят
 * подряд. Заголовок вставляется на каждой смене значения.
 *
 * Старая админка ходила в отдельную ветку get-list (builder_service_view_id
 * → GroupByColumns, object_builder.go:1977), и та собирала ВСЮ таблицу
 * одним запросом, игнорируя фильтры, поиск, сортировку и границы страницы
 * — включая отбор по умолчанию, то есть область видимости view. Строки,
 * которые админ спрятал, в сгруппированной таблице были видны. Поэтому
 * группировка здесь честная: тот же запрос, тот же отбор, та же прокрутка.
 *
 * Цена: группа, не поместившаяся в страницу, продолжается на следующей
 * со своим заголовком заново — счёт в заголовке считает загруженное,
 * а не всю группу.
 */
export type GroupEntry =
  /** Заголовок группы. `row` — первая строка группы: из неё берётся значение. */
  | { kind: "header"; key: string; row: number; count: number }
  | { kind: "row"; key: string; row: number };

/**
 * Ключ группы — значение поля целиком. JSON, а не String(): у MULTISELECT
 * значение — список, и String() склеивает ["a","b"] с ["a,b"] в одну группу.
 */
export function groupKey(value: unknown): string {
  return JSON.stringify(value ?? null);
}

export function groupEntries(rows: Item[], slug: string): GroupEntry[] {
  const entries: GroupEntry[] = [];
  let header: { kind: "header"; key: string; row: number; count: number } | null = null;

  rows.forEach((row, index) => {
    const key = groupKey(row[slug]);

    if (!header || header.key !== key) {
      header = { kind: "header", key, row: index, count: 0 };
      entries.push(header);
    }

    header.count += 1;
    entries.push({ kind: "row", key, row: index });
  });

  return entries;
}

/** Видимые записи: строки свёрнутых групп спрятаны, заголовки — всегда. */
export function visibleEntries(
  entries: GroupEntry[],
  collapsed: ReadonlySet<string>,
): GroupEntry[] {
  if (!collapsed.size) return entries;
  return entries.filter((entry) => entry.kind === "header" || !collapsed.has(entry.key));
}
