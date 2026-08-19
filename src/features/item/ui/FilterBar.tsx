import { useState } from "react";
import { IconArrowNarrowDown, IconArrowNarrowUp, IconPlus, IconSearch } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, type Field, type Relation } from "@/features/table";
import { Checkbox } from "@/shared/ui/checkbox";
import { Icon } from "@/shared/ui/icon";
import { Popover } from "@/shared/ui/popover";
import { emptyFilter, filterKind } from "../model/filter-kind";
import { activeFilterCount, type Filters, type Sort } from "../model/query";
import { fieldIcon } from "./field-icon";
import { FilterChip } from "./FilterChip";
import { SortPanel } from "./SortPanel";

/**
 * Подшапка: чипы сортировки, затем чипы фильтров. Поля добавляются по
 * одному, как в референсе: сначала выбираешь, по чему фильтровать,
 * потом задаёшь значение.
 *
 * Набор выбранных полей живёт в адресе вместе со значениями — ключ есть
 * в filters, значит чип показан. Старая версия писала этот набор
 * в настройки view (attributes.quick_filters) отдельным запросом на
 * каждое переключение; мы её набор ЧИТАЕМ как начальный, но обратно
 * не пишем: PUT перезаписывает view целиком, и промах в одном поле
 * стоит чужих настроек. Ссылка с фильтрами при этом пересылается
 * целиком — чего у старой версии как раз не было.
 */
export function FilterBar({
  columns,
  relations,
  language,
  filters,
  sorts,
  locked,
  onFilters,
  onSorts,
}: {
  columns: Field[];
  /**
   * Связи таблицы. Нет их — нет и отбора по связи: выбирать строку
   * не из чего, пока неизвестно, в какой таблице искать и чем подписать.
   */
  relations?: Relation[];
  language: string;
  filters: Filters;
  sorts: Sort[];
  /**
   * Слаги, по которым фильтр задан настройкой view и снят быть не может
   * (область видимости — см. `default_filters`). Такое поле не предлагается
   * к добавлению: чип по нему выглядел бы рабочим, а условие всё равно
   * перекрывалось бы настройкой. Сортировать по нему при этом можно,
   * поэтому из `columns` оно не убирается.
   */
  locked?: Set<string>;
  onFilters: (filters: Filters) => void;
  onSorts: (sorts: Sort[]) => void;
}) {
  const { t } = useTranslation();

  const byId = new Map((relations ?? []).map((relation) => [relation.id, relation]));
  const relationOf = (column: Field) =>
    column.relationId ? byId.get(column.relationId) : undefined;

  const filterable = columns.filter((column) => {
    const kind = filterKind(column);
    if (kind === null || locked?.has(column.slug)) return false;

    // Связь без настроек — это ввод, в котором нечего показать и не из
    // чего выбрать. Такое поле не предлагаем вовсе.
    return kind !== "relation" || Boolean(relationOf(column)?.toSlug);
  });
  const byslug = new Map(filterable.map((column) => [column.slug, column]));
  const count = activeFilterCount(filters);

  const add = (column: Field) => {
    const kind = filterKind(column);
    if (kind) onFilters({ ...filters, [column.slug]: emptyFilter(kind) });
  };

  const remove = (slug: string) => {
    const next = { ...filters };
    delete next[slug];
    onFilters(next);
  };

  return (
    <div className="flex min-h-11 shrink-0 flex-wrap items-center gap-2 border-b border-border px-3 py-1.5">
      {sorts.length > 0 && (
        <>
          <SortChips columns={columns} language={language} sorts={sorts} onChange={onSorts} />
          <span className="h-4 w-px shrink-0 bg-border" />
        </>
      )}

      {/* Порядок чипов — порядок добавления: он же порядок ключей. */}
      {Object.keys(filters).map((slug) => {
        const column = byslug.get(slug);
        if (!column) return null;

        return (
          <FilterChip
            key={slug}
            field={column}
            relation={relationOf(column)}
            language={language}
            filter={filters[slug]!}
            onChange={(value) => onFilters({ ...filters, [slug]: value })}
            onRemove={() => remove(slug)}
          />
        );
      })}

      {filterable.length > 0 && (
        <Popover
          trigger={({ open, toggle }) => (
            <button
              type="button"
              onClick={toggle}
              aria-expanded={open}
              aria-label={t("table.addFilter")}
              title={t("table.addFilter")}
              className="grid size-7 shrink-0 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
            >
              <Icon as={IconPlus} size={16} />
            </button>
          )}
        >
          {() => (
            <FieldPicker
              columns={filterable}
              language={language}
              chosen={filters}
              onAdd={add}
              onRemove={remove}
            />
          )}
        </Popover>
      )}

      {count > 0 && (
        <button
          type="button"
          onClick={() => onFilters({})}
          className="h-7 shrink-0 rounded-md px-2 text-xs text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
        >
          {t("table.clearFilters")}
        </button>
      )}
    </div>
  );
}

/**
 * Список полей с поиском по названию. Уже добавленные — сверху и
 * отмечены: так видно, что фильтр уже есть, и не появляется второй.
 */
function FieldPicker({
  columns,
  language,
  chosen,
  onAdd,
  onRemove,
}: {
  columns: Field[];
  language: string;
  chosen: Filters;
  onAdd: (column: Field) => void;
  onRemove: (slug: string) => void;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");

  const label = (column: Field) => localized(column.labels, language, column.label);

  const visible = columns
    .filter((column) => label(column).toLowerCase().includes(query.trim().toLowerCase()))
    .sort((a, b) => Number(b.slug in chosen) - Number(a.slug in chosen));

  return (
    <div className="flex w-64 flex-col gap-1">
      <label className="flex h-7 items-center gap-1.5 rounded-md border border-border-strong px-2 text-sm">
        <Icon as={IconSearch} size={14} className="text-fg-subtle" />
        <input
          autoFocus
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t("table.searchField")}
          className="min-w-0 flex-1 bg-transparent text-fg outline-none placeholder:text-fg-subtle"
        />
      </label>

      <div className="max-h-72 overflow-y-auto">
        {visible.map((column) => {
          const added = column.slug in chosen;

          return (
            <label
              key={column.id}
              className="flex h-8 cursor-pointer items-center gap-2 rounded-md px-2 text-sm transition-colors hover:bg-surface-hover"
            >
              <Checkbox
                checked={added}
                onChange={() => (added ? onRemove(column.slug) : onAdd(column))}
              />
              <Icon as={fieldIcon(column.type)} size={14} className="text-fg-muted" />
              <span className="truncate">{label(column)}</span>
            </label>
          );
        })}

        {!visible.length && (
          <p className="px-2 py-2 text-xs text-fg-subtle">{t("table.noFields")}</p>
        )}
      </div>
    </div>
  );
}

/**
 * Чипы сортировки слева от фильтров, как в референсе. Все открывают
 * одну панель: у сортировок общий порядок, и править их по одной,
 * не видя остальных, значит вслепую менять приоритет.
 */
function SortChips({
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
  const bySlug = new Map(columns.map((column) => [column.slug, column]));

  return (
    <Popover
      trigger={({ toggle }) => (
        <span className="flex items-center gap-1">
          {sorts.map((sort) => {
            const column = bySlug.get(sort.field);

            return (
              <button
                key={sort.field}
                type="button"
                onClick={toggle}
                className="flex h-7 max-w-56 items-center gap-1 rounded-full border border-accent bg-accent-subtle px-2 text-xs text-accent-text"
              >
                <Icon
                  as={sort.direction === "asc" ? IconArrowNarrowUp : IconArrowNarrowDown}
                  size={14}
                />
                <span className="truncate">
                  {column ? localized(column.labels, language, column.label) : sort.field}
                </span>
              </button>
            );
          })}
        </span>
      )}
    >
      {() => <SortPanel columns={columns} language={language} sorts={sorts} onChange={onChange} />}
    </Popover>
  );
}
