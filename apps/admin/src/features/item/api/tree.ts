import { useQueries } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { keys } from "@/shared/lib/query-keys";
import { errorMessage } from "@/shared/lib/toast";
import type { Item } from "../model/types";
import type { ItemsResponseDto } from "./items";

/**
 * Дети узлов TREE view: POST /v2/items/{slug}/tree.
 *
 * Ручка отдаёт прямых детей ОДНОГО родителя (`{слаг}_id: [guid]`, null —
 * корни) с флагом `has_child`, поэтому запрос — по одному на раскрытый
 * узел. useQueries, а не цикл useQuery: набор раскрытых меняется, а
 * количество хуков в компоненте меняться не может.
 *
 * Фильтры и поиск сюда не передаются сознательно: ручка их не читает
 * (ag_grid_tree.go, parseAndValidateRequest — только fields, limit,
 * offset и родитель), и передавать их значило бы рисовать рабочий
 * фильтр над списком, который на него не отвечает.
 */

/*
 * ponytail: одна страница детей на узел — шлюз всё равно режет по 100
 * (items.go, AgTree). Докрутка внутри узла — когда встретится живое
 * дерево шире сотни детей на уровень.
 */
const LIMIT = 100;

export function useTreeChildren(
  tableSlug: string | undefined,
  fieldSlugs: string[],
  expanded: ReadonlySet<string>,
) {
  const slug = tableSlug ?? "";
  // guid обязателен: по нему ключуются строки и раскрытие.
  const fields = [...new Set([...fieldSlugs, "guid"])];
  const fieldsKey = fields.join(",");
  /** Пустая строка — корни дерева. */
  const parents = ["", ...expanded];

  const queries = useQueries({
    queries: parents.map((parent) => ({
      queryKey: keys.items.tree(slug, parent, fieldsKey),
      queryFn: () =>
        api.post<ItemsResponseDto>(`/v2/items/${slug}/tree`, {
          data: {
            fields,
            [`${slug}_id`]: [parent || null],
            limit: LIMIT,
            offset: 0,
          },
        }),
      enabled: Boolean(slug),
      staleTime: 60_000,
    })),
  });

  const childrenOf = new Map<string, Item[]>();
  parents.forEach((parent, index) => {
    const rows = queries[index]?.data?.data?.response;
    if (rows) childrenOf.set(parent, rows);
  });

  const root = queries[0];

  return {
    childrenOf,
    isLoading: root?.isLoading ?? false,
    /** Причина отказа корневого запроса словами. null — всё в порядке. */
    error: errorMessage(root?.error ?? null, "table.loadFailed"),
    refetch: () => queries.forEach((query) => void query.refetch()),
  };
}
