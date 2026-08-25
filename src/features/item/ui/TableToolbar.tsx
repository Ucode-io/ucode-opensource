import { useEffect, useRef, useState } from "react";
import {
  IconArrowsSort,
  IconDotsVertical,
  IconLoader2,
  IconFilter,
  IconSearch,
  IconX,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import {
  localized,
  SEARCH_TYPES,
  useTableDetails,
  useUpdateSearchFields,
  type Field,
} from "@/features/table";
import { Icon } from "@/shared/ui/icon";
import { Popover, PopoverItem } from "@/shared/ui/popover";
import { ToolButton } from "@/shared/ui/tool-button";
import type { Sort } from "../model/query";
import { fieldIcon } from "./field-icon";
import { SortPanel } from "./SortPanel";

/**
 * Кнопки над таблицей: фильтр, сортировка, поиск.
 *
 * Иконки, а не подписи: строка стоит рядом с вкладками view, и три
 * слова вытеснили бы сами вкладки на узком экране. Смысл каждой кнопки
 * подсказывается всплывающей подписью, а активное состояние — цветом.
 *
 * Поиск раскрывается на месте, а не занимает ширину постоянно: он нужен
 * реже, чем виден.
 */
export function TableToolbar({
  tableSlug,
  columns,
  language,
  sorts,
  onSorts,
  filtersOpen,
  filterCount,
  onToggleFilters,
  search,
  onSearch,
}: {
  tableSlug: string;
  columns: Field[];
  language: string;
  sorts: Sort[];
  /**
   * Своя сортировка. Не задана — кнопки нет: доска расставлена руками
   * (`board_order`), и любая другая сортировка эту расстановку просто
   * спрятала бы.
   */
  onSorts?: ((sorts: Sort[]) => void) | undefined;
  filtersOpen: boolean;
  filterCount: number;
  onToggleFilters: () => void;
  search: string;
  /**
   * Свой поиск. Не задан — поля нет: право на поиск у роли отдельное
   * (`search_button`), и отнимают его именно у поля, а не у кнопки.
   */
  onSearch?: ((search: string) => void) | undefined;
}) {
  const { t } = useTranslation();

  return (
    <div className="flex items-center gap-0.5">
      {onSearch && (
        <SearchBox
          value={search}
          onChange={onSearch}
          tableSlug={tableSlug}
          language={language}
        />
      )}

      <ToolButton
        icon={IconFilter}
        label={t("table.filter")}
        open={filtersOpen}
        on={filterCount > 0}
        onClick={onToggleFilters}
      />

      {onSorts && (
        <Popover
          align="end"
          trigger={({ open, toggle }) => (
            <ToolButton
              icon={IconArrowsSort}
              label={t("table.sort")}
              open={open}
              on={sorts.length > 0}
              onClick={toggle}
            />
          )}
        >
          {() => (
            <SortPanel columns={columns} language={language} sorts={sorts} onChange={onSorts} />
          )}
        </Popover>
      )}
    </div>
  );
}

/**
 * Поиск: кнопка, которая на месте разворачивается в поле ввода.
 *
 * Свёрнутым остаётся только пустой и неактивный: заданный запрос обязан
 * быть виден, иначе таблица показывает неполный список без объяснения.
 *
 * Значение уходит с задержкой — иначе каждая буква становится запросом
 * к серверу и новой страницей в кэше.
 */
function SearchBox({
  value,
  onChange,
  tableSlug,
  language,
}: {
  value: string;
  onChange: (value: string) => void;
  tableSlug: string;
  language: string;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(Boolean(value));
  const [text, setText] = useState(value);
  const input = useRef<HTMLInputElement>(null);

  /*
   * onChange новый на каждом рендере — это замыкание на навигацию.
   * В зависимостях эффекта он сбрасывал бы таймер бесконечно, поэтому
   * держим его в ref: эффект зависит только от текста, а вызывается
   * всегда свежая версия.
   */
  const latest = useRef(onChange);
  latest.current = onChange;

  useEffect(() => {
    setText(value);
    if (value) setOpen(true);
  }, [value]);

  useEffect(() => {
    if (text === value) return;
    const timer = setTimeout(() => latest.current(text), 350);
    return () => clearTimeout(timer);
  }, [text, value]);

  const close = () => {
    setText("");
    setOpen(false);
    latest.current("");
  };

  if (!open) {
    return (
      <ToolButton
        icon={IconSearch}
        label={t("table.search")}
        onClick={() => {
          setOpen(true);
          // Фокус после отрисовки поля: до неё фокусировать нечего.
          requestAnimationFrame(() => input.current?.focus());
        }}
      />
    );
  }

  return (
    /*
     * Пустое поле схлопывается, как только фокус ушёл из него совсем:
     * пустая рамка посреди панели ничего не сообщает. Крестик очистки
     * лежит внутри, поэтому проверяется relatedTarget, а не сам факт
     * потери фокуса.
     */
    <div
      className="mr-1 flex items-center"
      onBlur={(event) => {
        if (!text && !event.currentTarget.contains(event.relatedTarget)) setOpen(false);
      }}
    >
      <label className="flex h-7 w-52 items-center gap-1.5 rounded-md px-1.5 text-sm">
        <Icon as={IconSearch} size={16} className="shrink-0 text-fg-muted" />
        <input
          ref={input}
          value={text}
          onChange={(event) => setText(event.target.value)}
          onKeyDown={(event) => event.key === "Escape" && close()}
          placeholder={t("table.searchPlaceholder")}
          className="min-w-0 flex-1 bg-transparent text-fg outline-none placeholder:text-fg-subtle"
        />
        {text && (
          <button
            type="button"
            onClick={close}
            aria-label={t("table.clearSearch")}
            className="shrink-0 text-fg-subtle transition-colors hover:text-fg"
          >
            <Icon as={IconX} size={14} />
          </button>
        )}
      </label>

      <SearchFieldsMenu tableSlug={tableSlug} language={language} />
    </div>
  );
}

/**
 * По каким полям искать.
 *
 * Без этого списка поиск выглядит сломанным: бэкенд ищет ТОЛЬКО по
 * колонкам, отмеченным флагом is_search, и, пока не отмечена ни одна,
 * любой запрос возвращает таблицу целиком. Флаг живёт на поле, а не
 * во view — отметка общая для всех view таблицы.
 */
function SearchFieldsMenu({ tableSlug, language }: { tableSlug: string; language: string }) {
  const { t } = useTranslation();
  const update = useUpdateSearchFields(tableSlug);
  const { fields, enabled, isFetching, error } = useTableDetails(tableSlug);
  // Отметка едет на сервер и возвращается оттуда же: без признака
  // работы список выглядит так, будто щелчок не сработал.
  const busy = isFetching || update.isPending;
  /*
   * Берутся поля ТАБЛИЦЫ, а не колонки view: флаг живёт на поле, общий
   * для всех view, и искать по колонке, скрытой в текущем, — обычное
   * дело. Числа и даты бэкенд в поиск не берёт, обещать их нельзя.
   */
  const searchable = fields.filter((field) => SEARCH_TYPES.has(field.type));

  return (
    <Popover
      align="end"
      trigger={({ open, toggle }) => (
        <ToolButton
          icon={IconDotsVertical}
          label={t("table.searchFields")}
          open={open}
          on={enabled.size > 0}
          onClick={toggle}
        />
      )}
    >
      {() => (
        <div className="max-h-80 w-64 overflow-y-auto">
          <p className="flex items-center gap-1.5 px-2 py-1 text-2xs text-fg-subtle">
            <span className="flex-1">
              {/* Список не приехал — говорим об этом, а не «искать не по чему». */}
              {error ?? t(searchable.length ? "table.searchFieldsHint" : "table.searchFieldsNone")}
            </span>
            {busy && <Icon as={IconLoader2} size={12} className="shrink-0 animate-spin" />}
          </p>

          {searchable.map((field) => (
            <PopoverItem
              key={field.id}
              active={enabled.has(field.id)}
              icon={<Icon as={fieldIcon(field.type)} size={16} className="shrink-0" />}
              onClick={() =>
                update.mutate([{ id: field.id, searchable: !enabled.has(field.id) }])
              }
            >
              {localized(field.labels, language, field.label)}
            </PopoverItem>
          ))}
        </div>
      )}
    </Popover>
  );
}
