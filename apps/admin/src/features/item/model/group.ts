import type { Item } from "./types";

/**
 * Группировка строк по полям — v2 §4 плана.
 *
 * Группирует КЛИЕНТ, но по уже отсортированному СЕРВЕРОМ списку: запрос
 * ставит поля группы первыми сортировками, и одинаковые значения приходят
 * подряд. Заголовок вставляется на каждой смене значения.
 *
 * Уровней столько, сколько полей в настройке (`attributes.group_by_columns`
 * — это список, и её панель в старой админке была списком галочек, а не
 * выбором одного поля). Вложенность считается тем же проходом: заголовок
 * уровня N меняется, когда сменилось значение на нём ИЛИ на любом уровне
 * выше.
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
  | {
      /** Заголовок группы. `row` — первая строка группы: из неё берётся значение. */
      kind: "header";
      /** Ключ этой группы: значения ВСЕХ уровней до неё включительно. */
      key: string;
      /** Ключи уровней сверху вниз, последний — свой. Ими считается свёртка. */
      path: string[];
      /** Уровень вложенности, 0 — верхний. Им же задаётся отступ. */
      level: number;
      row: number;
      count: number;
    }
  | { kind: "row"; path: string[]; row: number };

/**
 * Ключ группы — значения полей целиком. JSON, а не String(): у MULTISELECT
 * значение — список, и String() склеивает ["a","b"] с ["a,b"] в одну группу.
 */
export function groupKey(values: unknown[]): string {
  return JSON.stringify(values.map((value) => value ?? null));
}

export function groupEntries(rows: Item[], slugs: string[]): GroupEntry[] {
  if (!slugs.length) return [];

  const entries: GroupEntry[] = [];
  /** Открытые заголовки сверху вниз. Счёт им дописывается по ссылке. */
  let open: Extract<GroupEntry, { kind: "header" }>[] = [];

  rows.forEach((row, index) => {
    const values = slugs.map((slug) => row[slug]);
    const keys = slugs.map((_, level) => groupKey(values.slice(0, level + 1)));

    /* Сколько верхних уровней совпало с предыдущей строкой — те и остаются
       открытыми. Первый несовпавший закрывает собой всё, что ниже. */
    let same = 0;
    while (same < open.length && open[same]?.key === keys[same]) same += 1;
    open = open.slice(0, same);

    for (let level = same; level < slugs.length; level += 1) {
      const header = {
        kind: "header" as const,
        key: keys[level] ?? "",
        path: keys.slice(0, level + 1),
        level,
        row: index,
        count: 0,
      };

      entries.push(header);
      open.push(header);
    }

    for (const header of open) header.count += 1;
    entries.push({ kind: "row", path: keys, row: index });
  });

  return entries;
}

/**
 * Видимые записи: содержимое свёрнутых групп спрятано, сами заголовки
 * свёрнутых — нет.
 *
 * Свёрнутым считается не только своё значение, но и любое над ним:
 * свернули отдел — вместе с ним ушли и его проекты, и строки в них.
 */
export function visibleEntries(
  entries: GroupEntry[],
  collapsed: ReadonlySet<string>,
): GroupEntry[] {
  if (!collapsed.size) return entries;

  return entries.filter((entry) => {
    const ancestors = entry.kind === "header" ? entry.path.slice(0, -1) : entry.path;
    return !ancestors.some((key) => collapsed.has(key));
  });
}
