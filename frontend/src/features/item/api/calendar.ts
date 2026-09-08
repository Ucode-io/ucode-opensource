import { useMemo } from "react";
import { dayKey, toWallClock } from "../model/calendar";
import { useItems } from "./items";

/**
 * Нерабочие дни календаря (`view.disable_dates`).
 *
 * Второй запрос по тому же диапазону, но в ЧУЖУЮ таблицу: расписание
 * выходных проекта лежит отдельным справочником, а не флагом в строке.
 * Ручка та же самая — обычный get-list, — поэтому своей здесь нет.
 */

/**
 * Сколько строк спрашивать. Диапазон календаря — не больше шести недель,
 * то есть 42 дня; запас на несколько записей в день. ponytail: потолок,
 * а не страницы — справочник выходных длиннее сотни строк на месяц
 * означает, что это не справочник выходных.
 */
const LIMIT = 100;

export function useDisabledDays({
  tableSlug,
  daySlug,
  from,
  to,
}: {
  /** Таблица расписания. Пусто — запроса нет, нерабочих дней нет. */
  tableSlug: string;
  daySlug: string;
  /** Границы видимого периода — те же, что и у строк календаря. */
  from: string;
  to: string;
}): ReadonlySet<string> {
  const enabled = Boolean(tableSlug && daySlug);

  const { page } = useItems(enabled ? tableSlug : undefined, {
    limit: LIMIT,
    page: 1,
    filters: { [daySlug]: { op: "between", values: [from, to] } },
  });

  return useMemo(() => {
    const days = new Set<string>();

    for (const row of page.rows) {
      /*
       * Читаем как календарную дату: схемы чужой таблицы у нас нет,
       * а колонка дня — это день. Значение с временем и поясом тоже
       * разберётся — от него возьмётся записанная дата.
       */
      const date = toWallClock(row[daySlug], "date");
      if (date) days.add(dayKey(date));
    }

    return days;
  }, [page.rows, daySlug]);
}
