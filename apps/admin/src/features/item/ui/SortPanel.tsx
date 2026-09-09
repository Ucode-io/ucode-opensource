import { IconChevronDown, IconPlus, IconTrash, IconX } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, type Field } from "@/features/table";
import { Icon } from "@/shared/ui/icon";
import { Popover, PopoverItem } from "@/shared/ui/popover";
import type { Sort, SortDirection } from "../model/query";
import { fieldIcon } from "./field-icon";

/**
 * Панель сортировки. Условий может быть несколько: бэкенд принимает
 * `order` объектом, и порядок ключей в нём и есть приоритет.
 *
 * Поле, уже участвующее в сортировке, из списка выбора убирается —
 * два условия по одной колонке не имеют смысла, и второе всё равно
 * потерялось бы при сборке объекта.
 */
export function SortPanel({
  columns,
  language,
  sorts,
  onChange,
}: {
  columns: Field[];
  language: string;
  sorts: Sort[];
  onChange: (sorts: Sort[]) => void;
}) {
  const { t } = useTranslation();

  const bySlug = new Map(columns.map((column) => [column.slug, column]));
  const label = (slug: string) => {
    const column = bySlug.get(slug);
    return column ? localized(column.labels, language, column.label) : slug;
  };

  const free = columns.filter((column) => !sorts.some((sort) => sort.field === column.slug));

  const replace = (index: number, sort: Sort) =>
    onChange(sorts.map((item, position) => (position === index ? sort : item)));

  return (
    <div className="flex w-80 flex-col gap-1">
      {sorts.map((sort, index) => (
        <div key={sort.field} className="flex items-center gap-1">
          <FieldPicker
            columns={[bySlug.get(sort.field), ...free].filter(Boolean) as Field[]}
            language={language}
            value={label(sort.field)}
            icon={bySlug.get(sort.field)?.type ?? ""}
            onChange={(field) => replace(index, { ...sort, field })}
          />

          <DirectionPicker
            value={sort.direction}
            onChange={(direction) => replace(index, { ...sort, direction })}
          />

          <button
            type="button"
            aria-label={t("table.removeSort")}
            onClick={() => onChange(sorts.filter((_, position) => position !== index))}
            className="grid size-6 shrink-0 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
          >
            <Icon as={IconX} size={14} />
          </button>
        </div>
      ))}

      {sorts.length > 0 && <div className="my-1 h-px bg-border" />}

      {free.length > 0 && (
        <Popover
          trigger={({ toggle }) => (
            <button
              type="button"
              onClick={toggle}
              className="flex h-8 items-center gap-2 rounded-md px-2 text-left text-sm text-fg transition-colors hover:bg-surface-hover"
            >
              <Icon as={IconPlus} size={14} />
              {t("table.addSort")}
            </button>
          )}
        >
          {(close) => (
            <FieldList
              columns={free}
              language={language}
              onPick={(field) => {
                onChange([...sorts, { field, direction: "asc" }]);
                close();
              }}
            />
          )}
        </Popover>
      )}

      {sorts.length > 0 && (
        <button
          type="button"
          onClick={() => onChange([])}
          className="flex h-8 items-center gap-2 rounded-md px-2 text-left text-sm text-danger transition-colors hover:bg-danger-subtle"
        >
          <Icon as={IconTrash} size={14} />
          {t("table.clearSort")}
        </button>
      )}
    </div>
  );
}

function FieldPicker({
  columns,
  language,
  value,
  icon,
  onChange,
}: {
  columns: Field[];
  language: string;
  value: string;
  icon: string;
  onChange: (slug: string) => void;
}) {
  return (
    <Popover
      trigger={({ toggle }) => (
        <button
          type="button"
          onClick={toggle}
          className="flex h-7 min-w-0 flex-1 items-center gap-1.5 rounded-md border border-border-strong px-2 text-sm text-fg transition-colors hover:bg-surface-hover"
        >
          <Icon as={fieldIcon(icon)} size={14} className="text-fg-muted" />
          <span className="truncate">{value}</span>
          <Icon as={IconChevronDown} size={12} className="ml-auto shrink-0 opacity-60" />
        </button>
      )}
    >
      {(close) => (
        <FieldList
          columns={columns}
          language={language}
          onPick={(slug) => {
            onChange(slug);
            close();
          }}
        />
      )}
    </Popover>
  );
}

function FieldList({
  columns,
  language,
  onPick,
}: {
  columns: Field[];
  language: string;
  onPick: (slug: string) => void;
}) {
  return (
    <div className="max-h-72 w-56 overflow-y-auto">
      {columns.map((column) => (
        <PopoverItem
          key={column.id}
          icon={<Icon as={fieldIcon(column.type)} size={14} className="text-fg-muted" />}
          onClick={() => onPick(column.slug)}
        >
          {localized(column.labels, language, column.label)}
        </PopoverItem>
      ))}
    </div>
  );
}

function DirectionPicker({
  value,
  onChange,
}: {
  value: SortDirection;
  onChange: (direction: SortDirection) => void;
}) {
  const { t } = useTranslation();

  return (
    <Popover
      align="end"
      trigger={({ toggle }) => (
        <button
          type="button"
          onClick={toggle}
          className="flex h-7 shrink-0 items-center gap-1 rounded-md border border-border-strong px-2 text-sm text-fg transition-colors hover:bg-surface-hover"
        >
          {t(value === "asc" ? "table.sortAsc" : "table.sortDesc")}
          <Icon as={IconChevronDown} size={12} className="opacity-60" />
        </button>
      )}
    >
      {(close) => (
        <div className="w-40">
          {(["asc", "desc"] as const).map((direction) => (
            <PopoverItem
              key={direction}
              onClick={() => {
                onChange(direction);
                close();
              }}
            >
              {t(direction === "asc" ? "table.sortAsc" : "table.sortDesc")}
            </PopoverItem>
          ))}
        </div>
      )}
    </Popover>
  );
}
