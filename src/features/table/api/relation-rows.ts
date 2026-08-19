import { useQuery } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { keys } from "@/shared/lib/query-keys";
import { relationLabel } from "@/shared/lib/relation-label";

/**
 * Строки ЧУЖОЙ таблицы для выбора руками: условие агрегата отбирает
 * строки по связи, а связь хранит guid'ы.
 *
 * Своя маленькая ручка, а не список строк из features/item, и это
 * не дублирование ради дублирования: features/item уже зависит
 * от features/table (ему нужны типы полей), и обратный импорт замкнул бы
 * кольцо между фичами. Нужно здесь ровно одно — десять строк по поиску
 * с подписью, а не страница, отбор, сортировка и виртуализация.
 *
 * `view_fields` уходит в тело, как это делает старая админка: подпись
 * собирается из тех же полей показа, что и в ячейке-связи.
 */
type RowsDto = { data?: { response?: Record<string, unknown>[] | null } };

export type RelationRow = { guid: string; label: string };

/** Десяти хватает: это подсказка при вводе, а не список записей. */
const LIMIT = 10;

export function useRelationRows({
  tableSlug,
  viewFieldSlugs,
  search,
}: {
  /** Таблица, из которой выбирают. Пусто — запроса нет. */
  tableSlug: string;
  /** Поля показа связи: из них собирается подпись строки. */
  viewFieldSlugs: string[];
  search: string;
}) {
  const text = search.trim();

  const query = useQuery({
    queryKey: keys.tables.relationRows(tableSlug, text),
    queryFn: () =>
      api.post<RowsDto>(`/v2/object/get-list/${tableSlug}`, {
        data: {
          limit: LIMIT,
          offset: 0,
          view_fields: viewFieldSlugs,
          ...(text ? { search: text } : {}),
        },
      }),
    enabled: Boolean(tableSlug),
    // Строки чужой таблицы для настройки: меняются они не в этот момент.
    staleTime: 30_000,
    select: (dto): RelationRow[] =>
      (dto.data?.response ?? []).map((row) => ({
        guid: String(row["guid"] ?? ""),
        label: relationLabel(row, viewFieldSlugs),
      })),
  });

  return { rows: query.data ?? [], isFetching: query.isFetching };
}
