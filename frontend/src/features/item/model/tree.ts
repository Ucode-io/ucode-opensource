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
      /*
       * `has_child` присылает ручка дерева. У дерева, собранного здесь же
       * из плоского списка (groupByParent), его нет — там дети известны
       * сразу, и признак виден по самой карте.
       */
      const hasChild = row["has_child"] === true || (childrenOf.get(guid)?.length ?? 0) > 0;
      meta.set(guid, { depth, hasChild });

      if (guid && expanded.has(guid)) visit(guid, depth + 1);
    }
  };

  visit("", 0);
  return { rows, meta };
}

/**
 * Плоский список строк → дети по guid родителя, как их отдала бы ручка.
 *
 * Нужно там, где строки уже загружены обычным get-list и ходить за ними
 * во второй раз незачем, — во вкладке связи. Ручка дерева туда не годится
 * принципиально: она читает из тела только родителя, а вкладке нужен
 * отбор по колонке-ссылке, и дерево показало бы всю чужую таблицу
 * (docs/backend-notes.md, «Дерево»).
 *
 * СИРОТЫ ВСТАЮТ В КОРЕНЬ. Строка, чей родитель в набор не попал —
 * не связан с открытой записью, отсеян фильтром или остался на другой
 * странице, — иначе не показалась бы вовсе: её не к чему подвесить.
 * Спрятать связанную строку хуже, чем показать её без предка.
 *
 * @param parentSlug колонка-ссылка на родителя: `<слаг таблицы>_id`
 */
export function groupByParent(rows: Item[], parentSlug: string): Map<string, Item[]> {
  const known = new Set(rows.map((row) => (typeof row.guid === "string" ? row.guid : "")));
  const childrenOf = new Map<string, Item[]>();

  for (const row of rows) {
    const parent = row[parentSlug];
    const at = typeof parent === "string" && known.has(parent) ? parent : "";

    childrenOf.set(at, [...(childrenOf.get(at) ?? []), row]);
  }

  return childrenOf;
}
