import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { keys } from "@/shared/lib/query-keys";
import { reportError } from "@/shared/lib/toast";
import type { Item } from "../model/types";
import { type ItemsQuery, toRequestBody } from "../model/query";

/**
 * Строки таблицы.
 *
 * Метод POST, хотя это чтение: фильтры уезжают телом, а не строкой
 * запроса — их длина не ограничена, и в них бывают персональные данные.
 *
 * Ответ вложен дважды: клиент снимает общий конверт {status, data},
 * а под ним лежит ещё {data: {count, response}}. Разворачиваем здесь
 * один раз, чтобы слово data не тянулось через все вызовы.
 */
export type ItemsResponseDto = {
  data?: { count?: number; response?: Item[] | null };
};

export type ItemsPage = {
  rows: Item[];
  /** Сколько строк всего — для счётчика и пагинации. */
  count: number;
};

const EMPTY_PAGE: ItemsPage = { rows: [], count: 0 };

export function useItems(tableSlug: string | undefined, query: ItemsQuery) {
  const slug = tableSlug ?? "";
  /*
   * Тело первой страницы и есть ключ кэша: любая правка сортировки,
   * отбора или поиска обязана дать новый кэш, а перечислять поля
   * по одному — способ однажды забыть новое.
   *
   * Смещение в ключ не входит: страницы лежат под одним ключом, одна
   * за другой, — иначе прокрутка вниз плодила бы по кэшу на страницу
   * и вверх было бы нечего показать.
   */
  const body = toRequestBody({ ...query, page: 1 });

  const result = useInfiniteQuery({
    queryKey: keys.items.list(slug, body),
    queryFn: ({ pageParam }) =>
      api.post<ItemsResponseDto>(`/v2/object/get-list/${slug}`, {
        data: { ...body, offset: pageParam * query.limit },
      }),
    enabled: Boolean(slug),
    staleTime: 60_000,
    initialPageParam: 0,
    /*
     * Следующая страница есть, пока загруженных строк меньше, чем
     * сказал `count`. Курсора у ручки нет — только limit и offset, —
     * поэтому «следующая» это просто следующий номер.
     *
     * Пустой ответ тоже останавливает: у таблицы, из которой строки
     * удаляют прямо сейчас, `count` бывает больше, чем реально есть,
     * и без этой проверки грид крутил бы запросы до конца страницы.
     */
    getNextPageParam: (_last, all) => nextPage(all),
    select: toPages,
  });

  return {
    page: result.data ?? EMPTY_PAGE,
    isLoading: result.isLoading,
    /** Обновление поверх уже показанных строк — для индикатора, не для скелетона. */
    isFetching: result.isFetching,
    /** Есть ли что грузить дальше и не грузится ли уже. */
    hasMore: result.hasNextPage && !result.isFetchingNextPage,
    loadingMore: result.isFetchingNextPage,
    loadMore: () => {
      if (result.hasNextPage && !result.isFetchingNextPage) void result.fetchNextPage();
    },
    error: result.error,
  };
}

/**
 * Одна строка по guid.
 *
 * Тем же get-list, а не отдельной ручкой за одной записью, и это
 * не лень: список приносит вместе со строкой её связанные записи
 * (`<слаг>_data`), а ячейка-связь показывает именно их. Через
 * `GET /v2/items/{slug}/{id}` пришла бы та же строка, но с голыми
 * идентификаторами вместо названий — и карточка, открытая по ссылке,
 * выглядела бы иначе, чем открытая из таблицы.
 *
 * `enabled` — потому что нужна она редко: строка почти всегда уже лежит
 * в загруженной странице, и запрашивать её второй раз незачем. Нужна,
 * когда ссылку на запись прислали, а страница открылась другая: своя
 * страница, свой отбор, своя сортировка.
 */
export function useItem(
  tableSlug: string | undefined,
  guid: string | undefined,
  enabled: boolean,
) {
  const query = useItems(enabled && guid ? tableSlug : undefined, {
    limit: 1,
    page: 1,
    // `contains` — форма записи, а не поиск подстроки: в теле get-list
    // это голое значение рядом со слагом. См. toCondition.
    filters: { guid: { op: "contains", values: [guid ?? ""] } },
  });

  return { item: query.page.rows[0], isLoading: query.isLoading };
}

/** response приходит null, когда строк нет, — это не ошибка. */
export function toPage(dto: ItemsResponseDto): ItemsPage {
  return {
    rows: dto.data?.response ?? [],
    count: dto.data?.count ?? 0,
  };
}

/**
 * Номер следующего куска. `undefined` — грузить больше нечего.
 *
 * Условий два, и второе не для красоты: у таблицы, из которой строки
 * удаляют прямо сейчас, `count` бывает больше, чем реально есть,
 * и без проверки «последний кусок что-то принёс» грид крутил бы
 * запросы до конца страницы.
 */
export function nextPage(pages: ItemsResponseDto[]): number | undefined {
  const last = pages[pages.length - 1];
  if (!last) return undefined;

  const loaded = pages.reduce((sum, page) => sum + (page.data?.response?.length ?? 0), 0);
  const grew = (last.data?.response?.length ?? 0) > 0;

  return grew && loaded < (last.data?.count ?? 0) ? pages.length : undefined;
}

/**
 * Страницы бесконечной прокрутки → один список.
 *
 * `count` берётся у последней: пока человек листает, строки добавляют
 * и удаляют, и свежее число честнее того, что приехало со стартовой
 * страницей.
 */
export function toPages(data: { pages: ItemsResponseDto[] }): ItemsPage {
  const rows = data.pages.flatMap((page) => page.data?.response ?? []);
  const last = data.pages[data.pages.length - 1];

  return { rows, count: last?.data?.count ?? rows.length };
}

/** Правка одной ячейки. Больше в теле ничего и не должно быть. */
export type CellEdit = { guid: string; slug: string; value: unknown };

/**
 * Правка ячейки.
 *
 * Уходит ровно одно поле и guid: бэкенд собирает UPDATE из тех ключей,
 * что пришли (items.go: `if ok { query += ... }`), и остального не
 * трогает. Слать строку целиком нельзя — вернёшь на место чужие правки,
 * сделанные, пока таблица была открыта.
 *
 * Значение подставляется в кэш до ответа: ячейка обязана меняться
 * мгновенно, иначе таблица ощущается как форма. При ошибке снимок
 * возвращается на место, а в конце список всё равно перезапрашивается —
 * вместе с полем правки бэкенд пересчитывает формулы и updated_at,
 * и их значения знает только он.
 */
export function useUpdateItem(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: ({ guid, slug: field, value }: CellEdit) =>
      api.put<unknown>(`/v2/items/${slug}`, { data: { guid, [field]: value } }),

    onMutate: async (edit) => {
      // Летящий запрос списка перезапишет наш патч своим старым ответом.
      await queryClient.cancelQueries({ queryKey: keys.items.all });

      const snapshot = queryClient.getQueriesData({ queryKey: keys.items.all });
      queryClient.setQueriesData({ queryKey: keys.items.all }, (page: unknown) =>
        patchRow(page, edit),
      );

      return snapshot;
    },

    onError: (error, _edit, snapshot) => {
      // Значение возвращается на место, и без объяснения это выглядит
      // как «ячейка не нажимается».
      snapshot?.forEach(([key, data]) => queryClient.setQueryData(key, data));
      reportError(error, "common.saveFailed");
    },

    onSettled: () => queryClient.invalidateQueries({ queryKey: keys.items.all }),
  });
}

/**
 * Тот же ответ с изменённой строкой. Кэш хранит сырой ответ бэкенда
 * (select применяется на выходе), поэтому и патчится он в этой форме.
 *
 * Чужая форма проходит насквозь: под ключом items лежат и одиночные
 * записи, и ответы других ручек.
 */
export function patchRow(page: unknown, { guid, slug, value }: CellEdit): unknown {
  const rows = (page as ItemsResponseDto | undefined)?.data?.response;
  if (!Array.isArray(rows)) return page;

  const index = rows.findIndex((row) => row.guid === guid);
  if (index === -1) return page;

  const next = rows.slice();
  next[index] = { ...rows[index], [slug]: value };

  const dto = page as ItemsResponseDto;
  return { ...dto, data: { ...dto.data, response: next } };
}

/**
 * Удаление отмеченных строк — одним запросом, а не циклом по одному:
 * бэкенд удаляет их в одной транзакции, и половина удалённых строк
 * при сбое в середине не остаётся.
 *
 * Метод DELETE с телом: так объявлена ручка (DELETE /v1/object/{slug}).
 */
export function useDeleteItems(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (ids: string[]) => api.delete<unknown>(`/v1/object/${slug}`, { data: { ids } }),
    onError: (error) => reportError(error, "common.deleteFailed"),
    // Инвалидируем весь раздел: удаление сдвигает страницы, и соседние
    // страницы в кэше после него неверны.
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.items.all }),
  });
}
