import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { Field, Relation } from "@/features/table";
import { useTreeChildren } from "../api/tree";
import { flattenTree } from "../model/tree";
import type { Item } from "../model/types";
import type { ColumnActions } from "./ColumnMenu";
import { DataGrid, GridSkeleton } from "./DataGrid";

/**
 * TREE view: тот же грид, но строки разложены деревом по рекурсивной
 * связи таблицы на саму себя (колонка `<слаг>_id`, см. model/tree).
 *
 * Дети подгружаются по мере раскрытия — по запросу на узел, как это
 * делает и ручка. Раскрытие живёт здесь, а не в адресе: это состояние
 * взгляда, как прокрутка, а не экран, который пересылают ссылкой.
 *
 * Сортировки и фильтров у дерева нет намеренно: ручка /tree их
 * не читает вовсе, а рабочий на вид фильтр над списком, который на него
 * не отвечает, хуже отсутствующего.
 */
export function TreeGrid({
  tableSlug,
  columns,
  pinned,
  widths,
  onWidth,
  relations,
  locale,
  language,
  selected,
  onSelect,
  onOpenRow,
  onEdit,
  onAddChild,
  onAddField,
  columnActions,
}: {
  tableSlug: string;
  columns: Field[];
  pinned?: ReadonlySet<string>;
  widths?: Record<string, number> | undefined;
  onWidth?: ((fieldId: string, width: number) => void) | undefined;
  relations: Relation[];
  locale: string;
  language: string;
  selected: ReadonlySet<string>;
  onSelect: (next: Set<string>) => void;
  onOpenRow?: (guid: string) => void;
  onEdit?: (guid: string, slug: string, value: unknown) => void;
  /** Завести дочернюю запись под строкой. Нет — кнопки у строк нет. */
  onAddChild?: (parent: Item) => void;
  onAddField?: (anchor: DOMRect) => void;
  columnActions?: ColumnActions;
}) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());

  const { childrenOf, isLoading, error, refetch } = useTreeChildren(
    tableSlug,
    columns.map((column) => column.slug),
    expanded,
  );
  const { rows, meta } = flattenTree(childrenOf, expanded);

  const toggle = (guid: string) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (!next.delete(guid)) next.add(guid);
      return next;
    });

  /*
   * «Дочерняя запись» сразу раскрывает родителя — так делал и старый
   * код (createChildTree): созданный ребёнок должен появиться на глазах,
   * а не спрятаться под свёрнутым узлом.
   */
  const addChild = (guid: string) => {
    const parent = rows.find((row) => row.guid === guid);
    if (!parent) return;

    setExpanded((prev) => new Set(prev).add(guid));
    onAddChild?.(parent);
  };

  if (isLoading) return <GridSkeleton columns={columns.length} />;

  if (error) {
    return (
      <div className="grid flex-1 place-items-center p-8 text-center">
        <div className="flex max-w-sm flex-col items-center gap-3">
          <p className="text-sm text-fg-muted">{error}</p>
          <button
            type="button"
            onClick={refetch}
            className="h-8 rounded-md border border-border-strong px-3 text-sm text-fg transition-colors hover:bg-surface-hover"
          >
            {t("action.retry")}
          </button>
        </div>
      </div>
    );
  }

  return (
    <DataGrid
      tableSlug={tableSlug}
      columns={columns}
      {...(pinned ? { pinned } : {})}
      widths={widths}
      onWidth={onWidth}
      rows={rows}
      tree={{ meta, expanded, onToggle: toggle, ...(onAddChild ? { onAddChild: addChild } : {}) }}
      relations={relations}
      locale={locale}
      language={language}
      selected={selected}
      onSelect={onSelect}
      /* У дерева свой порядок — обход иерархии, сортировать его нечем. */
      sorts={[]}
      onSort={() => {}}
      {...(onOpenRow ? { onOpenRow } : {})}
      {...(onEdit ? { onEdit } : {})}
      {...(onAddField ? { onAddField } : {})}
      {...(columnActions ? { columnActions } : {})}
    />
  );
}
