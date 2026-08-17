import { useState } from "react";
import {
  IconAdjustments,
  IconFilter,
  IconSortAscending,
  IconSortDescending,
  IconTrash,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, type Field } from "@/features/table";
import { Anchored } from "@/shared/ui/anchored";
import { Icon } from "@/shared/ui/icon";
import { filterKind } from "../model/filter-kind";
import type { SortDirection } from "../model/query";
import { fieldIcon } from "./field-icon";

/**
 * Меню колонки: то, что делают с полем, не уходя из таблицы.
 *
 * Переименование прямо здесь, отдельным полем ввода: это самая частая
 * правка, и гонять ради неё диалог с типом и вариантами — лишний шаг.
 * Всё остальное, что меняет схему, живёт в диалоге настроек.
 *
 * Чего здесь нет и почему: «заморозить», «скрыть» и «вставить слева» —
 * это правка списка колонок VIEW, а PUT view перезаписывает его целиком
 * вместе с чужими настройками. «Группировать» и «посчитать» — фичи,
 * которых в таблице пока нет; пункт меню, который ничего не делает,
 * хуже отсутствующего. «Переносить текст» не сделан сознательно:
 * высота строки фиксирована, на ней держится виртуализация.
 */
export type ColumnActions = {
  rename: (field: Field, label: string) => void;
  /** Панель настроек всплывает там же, где меню: ей нужен тот же якорь. */
  settings: (field: Field, anchor: DOMRect) => void;
  filter: (field: Field) => void;
  remove: (field: Field) => void;
};

export function ColumnMenu({
  field,
  language,
  anchor,
  actions,
  onSort,
  onClose,
}: {
  field: Field;
  language: string;
  anchor: DOMRect;
  actions: ColumnActions;
  onSort: (slug: string, direction: SortDirection) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const label = localized(field.labels, language, field.label);
  const [name, setName] = useState(label);

  /*
   * У колонки-связи подпись лежит не в `label`, а в attributes, и
   * уезжает своей ручкой — переименования на месте у неё нет.
   *
   * «Настройки» и «Удалить» есть у всех. У связи оба пункта ведут
   * в её собственные ручки: настройки открывают форму связи, а удаление
   * сносит связь целиком — вместе с колонкой-ссылкой. Различие
   * вызывающий и разбирает, здесь оно не видно.
   */
  const renamable = !field.relationId;
  const filterable = filterKind(field) !== null;

  const run = (action: () => void) => {
    action();
    onClose();
  };

  /** Переименование применяется при закрытии — как правка ячейки. */
  const commitName = () => {
    const next = name.trim();
    if (renamable && next && next !== label) actions.rename(field, next);
  };

  return (
    <Anchored
      anchor={anchor}
      onClose={() => {
        commitName();
        onClose();
      }}
      onCancel={onClose}
    >
      <div className="w-64 rounded-lg border border-border bg-surface p-1 shadow-popover">
        {renamable ? (
        <div className="flex items-center gap-1.5 p-1">
          <span className="grid size-7 shrink-0 place-items-center rounded-md border border-border text-fg-muted">
            <Icon as={fieldIcon(field.type)} size={14} />
          </span>

          <input
            autoFocus
            value={name}
            onChange={(event) => setName(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                commitName();
                onClose();
              }
            }}
            aria-label={t("fieldForm.label")}
            className="h-8 min-w-0 flex-1 rounded-md border border-border-strong bg-surface px-2 text-sm text-fg outline-none focus:border-accent"
          />
        </div>
        ) : (
          <p className="flex items-center gap-1.5 p-2 text-sm text-fg-muted">
            <Icon as={fieldIcon(field.type)} size={14} />
            <span className="truncate">{label}</span>
          </p>
        )}

        <MenuItem
          icon={IconAdjustments}
          onClick={() => run(() => actions.settings(field, anchor))}
          label={t("column.settings")}
        />

        <div className="my-1 h-px bg-border" />

        <MenuItem
          icon={IconSortAscending}
          onClick={() => run(() => onSort(field.slug, "asc"))}
          label={t("table.sortAsc")}
        />
        <MenuItem
          icon={IconSortDescending}
          onClick={() => run(() => onSort(field.slug, "desc"))}
          label={t("table.sortDesc")}
        />
        {filterable && (
          <MenuItem
            icon={IconFilter}
            onClick={() => run(() => actions.filter(field))}
            label={t("table.addFilter")}
          />
        )}

        <div className="my-1 h-px bg-border" />

        <MenuItem
          icon={IconTrash}
          danger
          onClick={() => run(() => actions.remove(field))}
          label={t("column.delete")}
        />
      </div>
    </Anchored>
  );
}

function MenuItem({
  icon,
  label,
  danger,
  onClick,
}: {
  icon: typeof IconTrash;
  label: string;
  danger?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors ${
        danger ? "text-danger hover:bg-danger-subtle" : "text-fg hover:bg-surface-hover"
      }`}
    >
      <Icon as={icon} size={16} className={danger ? "" : "text-fg-muted"} />
      <span className="truncate">{label}</span>
    </button>
  );
}
