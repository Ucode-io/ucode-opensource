import type { Item } from "./types";

/**
 * TREE view: иерархия строк по рекурсивной связи таблицы на саму себя.
 *
 * Колонка-ссылка на родителя задана бэкендом жёстко: `<слаг таблицы>_id`
 * (ag_grid_tree.go:105, `childField := req.TableSlug + "_id"`). Ручка
 * отдаёт ПРЯМЫХ детей одного узла с флагом `has_child` — дерево
 * подгружается по мере раскрытия, а не целиком.
 *
 * Здесь — сборка загруженных уровней в плоский список для таблицы:
 * виртуализация и рендер остаются теми же, дерево — это порядок строк
 * и отступ, а не другой грид.
 */
export type TreeMeta = { depth: number; hasChild: boolean };

/**
 * @param childrenOf дети по guid родителя; корни — под пустой строкой
 * @param expanded guid'ы раскрытых узлов
 */
export function flattenTree(
  childrenOf: ReadonlyMap<string, Item[]>,
  expanded: ReadonlySet<string>,
): { rows: Item[]; meta: Map<string, TreeMeta> } {
  const rows: Item[] = [];
  const meta = new Map<string, TreeMeta>();
  /* Испорченные данные бывают циклом a→b→a: без защиты это вечная петля. */
  const seen = new Set<string>();

  const visit = (parent: string, depth: number) => {
    for (const row of childrenOf.get(parent) ?? []) {
      const guid = typeof row.guid === "string" ? row.guid : "";
      if (guid && seen.has(guid)) continue;
      if (guid) seen.add(guid);

      rows.push(row);
      meta.set(guid, { depth, hasChild: row["has_child"] === true });

      if (guid && expanded.has(guid)) visit(guid, depth + 1);
    }
  };

  visit("", 0);
  return { rows, meta };
}
