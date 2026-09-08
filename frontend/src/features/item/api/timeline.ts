import { useMemo } from "react";
import { isBlank } from "../model/cell-value";
import type { Filters } from "../model/query";
import type { Item } from "../model/types";
import { useItems } from "./items";

/**
 * Строки БЕЗ дат: на оси их нет, но потерять их нельзя — задачу без
 * срока ставят на таймлайн перетаскиванием, и для этого её сначала надо
 * увидеть.
 *
 * Отбором по диапазону они отсекаются вместе со всем остальным, а
 * спросить «где поле пусто» у ручки нечем: get-list понимает `$gt`,
 * `$gte`, `$lt`, `$lte`, `$in` и равенство — условия «пусто» среди них
 * нет (build_query.go, buildComparisonFilters). См. docs/backend-notes.md.
 *
 * Поэтому спрашиваем ОБРАТНЫМ порядком по тому же полю: `ORDER BY
 * a.<поле> DESC` (build_query.go, buildOrderClause) в Postgres ставит
 * NULL первыми, и пустые строки приезжают началом первой же страницы.
 * На веру это не принимается — приехавшее всё равно отбирается по
 * значению, и если порядок однажды изменится, список просто опустеет.
 */

/**
 * Сколько строк спрашивать.
 *
 * ponytail: потолок, а не страницы. Список без дат — это то, что забыли
 * запланировать; сотня забытых задач означает, что чинить надо не
 * прокрутку.
 */
const LIMIT = 50;

export function useUndatedRows({
  tableSlug,
  fromSlug,
  filters,
  search,
}: {
  /** Пусто — запроса нет: без поля начала таймлайна нет вовсе. */
  tableSlug: string;
  fromSlug: string;
  /**
   * Отбор БЕЗ диапазона дат: область видимости view и фильтры человека
   * тут те же, что у оси, — строка, спрятанная настройкой view, не должна
   * всплывать в списке без дат.
   */
  filters: Filters;
  search: string;
}): Item[] {
  const enabled = Boolean(tableSlug && fromSlug);

  const { page } = useItems(enabled ? tableSlug : undefined, {
    limit: LIMIT,
    page: 1,
    sorts: [{ field: fromSlug, direction: "desc" }],
    filters,
    search,
  });

  return useMemo(
    () => page.rows.filter((row) => isBlank(row[fromSlug])),
    [page.rows, fromSlug],
  );
}
