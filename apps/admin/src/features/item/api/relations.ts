import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { keys } from "@/shared/lib/query-keys";
import { reportError } from "@/shared/lib/toast";
import { toRequestBody } from "../model/query";
import type { RelationEdit } from "../model/relation";
import type { Item } from "../model/types";
import { toPage, type ItemsResponseDto } from "./items";

/**
 * Строки связанной таблицы — то, из чего выбирают в ячейке-связи.
 *
 * Та же ручка списка, что и у самой таблицы, поэтому и ключ кэша тот же:
 * второй раз одни и те же строки не поедут, а правка ячейки, попавшая
 * в этот же кэш, подхватится обоими списками.
 *
 * Двадцать строк без «показать ещё»: выбор делается поиском, а не
 * прокруткой чужой таблицы на тысячу строк.
 */
const LIMIT = 20;

export function useRelationItems(
  tableSlug: string | undefined,
  search: string,
  /**
   * Автофильтр связи: плоские условия, которыми отобраны строки чужой
   * таблицы (см. model/relation, autoFilterValues). Кладутся ПЕРЕД
   * служебными ключами — как обычный отбор, у которого поле со слагом
   * `limit` не должно перебить лимит.
   */
  filters: Record<string, unknown> = {},
) {
  const slug = tableSlug ?? "";
  const body = { ...filters, ...toRequestBody({ limit: LIMIT, page: 1, search }) };

  const query = useQuery({
    queryKey: keys.items.list(slug, body),
    queryFn: () => api.post<ItemsResponseDto>(`/v2/object/get-list/${slug}`, { data: body }),
    enabled: Boolean(slug),
    staleTime: 60_000,
    // Список не должен мигать пустотой на каждую букву поиска.
    placeholderData: (previous) => previous,
    select: toPage,
  });

  return {
    items: query.data?.rows ?? [],
    isLoading: query.isLoading,
  };
}

/**
 * Новая строка в связанной таблице.
 *
 * guid придумывает клиент, а не сервер (items.go: пустой guid он
 * заменяет своим). Это не микрооптимизация: сразу после создания строку
 * надо связать, и без своего guid пришлось бы выуживать его из ответа
 * и надеяться, что он там есть.
 */
export function useCreateItem(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    /*
     * Сопровождающие записи связей (`<слаг>_data`) отбрасываются здесь,
     * один раз на всех вызывающих: это не колонки, а то, что бэкенд
     * дописывает к ответу, и вставка по такому ключу — 500. В черновике
     * они лежат затем, чтобы ячейка-связь показывала подпись, а не uuid.
     */
    mutationFn: (values: Item) =>
      api.post<unknown>(`/v2/items/${slug}`, {
        data: Object.fromEntries(
          Object.entries(values).filter(([key]) => !key.endsWith("_data")),
        ),
      }),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.items.all }),
  });
}

/**
 * Связать строку с другой или снять связь.
 *
 * Обычная правка колонки: ссылка Many2One лежит прямо в строке
 * (см. model/relation — других связей из таблицы не бывает).
 *
 * Оптимистичной подстановки нет: в строке меняется не значение,
 * а сопровождающий его `<слаг>_data` со всей связанной строкой, и
 * собрать его на клиенте — значит повторить логику бэкенда наугад.
 */
export function useLinkRelation(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (edit: RelationEdit) =>
      api.put<unknown>(`/v2/items/${slug}`, {
        data: { guid: edit.rowGuid, [edit.fieldSlug]: edit.itemGuid },
      }),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.items.all }),
  });
}
