import { useQuery } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { keys } from "@/shared/lib/query-keys";
import { relationLabel, type ViewField } from "@/shared/lib/relation-label";

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

const NO_ROWS: RelationRow[] = [];

/** Десяти хватает: это подсказка при вводе, а не список записей. */
const LIMIT = 10;

export function useRelationRows({
  tableSlug,
  viewFields,
  search,
  limit = LIMIT,
}: {
  /** Таблица, из которой выбирают. Пусто — запроса нет. */
  tableSlug: string;
  /** Поля показа связи: из них собирается подпись строки. */
  viewFields: ViewField[];
  search: string;
  /**
   * Сколько строк спрашивать. По умолчанию десять — столько влезает
   * в подсказку при вводе. Вкладкам группировки нужно больше: там
   * список не подсказывает, а перечисляет.
   */
  limit?: number;
}) {
  const text = search.trim();

  const query = useQuery({
    // Лимит в ключе: под одним ключом не должны лежать десять строк
    // подсказки и полсотни вкладок — вторые прочитали бы первые.
    queryKey: keys.tables.relationRows(tableSlug, text, limit),
    queryFn: () =>
      api.post<RowsDto>(`/v2/object/get-list/${tableSlug}`, {
        data: {
          limit,
          offset: 0,
          view_fields: viewFields.map((field) => field.slug),
          ...(text ? { search: text } : {}),
        },
      }),
    enabled: Boolean(tableSlug),
    // Строки чужой таблицы для настройки: меняются они не в этот момент.
    staleTime: 30_000,
    select: (dto): RelationRow[] =>
      (dto.data?.response ?? []).map((row) => ({
        guid: String(row["guid"] ?? ""),
        label: relationLabel(row, viewFields),
      })),
  });

  // Постоянная ссылка на пустоту: `?? []` отдавал бы новый массив
  // на каждый рендер, и useMemo у вызывающего пересчитывался бы всегда.
  return { rows: query.data ?? NO_ROWS, isFetching: query.isFetching, isLoading: query.isLoading };
}
