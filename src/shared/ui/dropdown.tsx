import type { UIEvent } from "react";
import { IconCheck, IconChevronDown, IconSearch } from "@tabler/icons-react";
import { Icon } from "./icon";
import { Popover, PopoverItem } from "./popover";

export type DropdownItem = { value: string; label: string };

/**
 * Выпадающий список в нашем оформлении.
 *
 * Не `<select>`: системное меню браузера рисуется шрифтом и цветами
 * операционной системы, тёмную тему не знает и рядом с нашими кнопками
 * выглядит чужим. Внутри — тот же `Popover`, что у меню строки сайдбара
 * и у панели view, поэтому Escape, закрытие по клику мимо и переворот
 * вверх у него уже правильные.
 *
 * Список бывает и длинным: передан `onSearch` — сверху появляется поиск,
 * передан `onLoadMore` — прокрутка догружает следующую страницу. Ищет
 * и грузит вызывающий: `shared/` не знает ни про запросы, ни про то,
 * откуда берутся строки.
 *
 * Отличие от `SelectMenu`: тот раскрывается НА МЕСТЕ и живёт внутри
 * панели поля — она сама всплывашка со своим Escape, и второго слоя
 * поверх ей не нужно. Здесь наоборот: место под кнопкой занято таблицей
 * или строкой отбора, и раскрываться список должен слоем поверх.
 */
export function Dropdown({
  value,
  items,
  placeholder = "",
  ariaLabel,
  className = "",
  onChange,
  search,
  searchPlaceholder = "",
  emptyText = "",
  loading = false,
  hasMore = false,
  onSearch,
  onLoadMore,
}: {
  value: string;
  items: DropdownItem[];
  /** Что на кнопке, когда ничего не выбрано. */
  placeholder?: string;
  ariaLabel?: string;
  /** Ширина задаётся снаружи: у панели свои размеры. */
  className?: string;
  onChange: (value: string) => void;
  /** Строка поиска. Есть `onSearch` — над списком появляется поле. */
  search?: string;
  searchPlaceholder?: string;
  /** Что показать вместо пустого списка. Нужен только там, где есть поиск. */
  emptyText?: string;
  loading?: boolean;
  /** Есть ли ещё страница. Пусто — прокрутка ничего не догружает. */
  hasMore?: boolean;
  onSearch?: (query: string) => void;
  onLoadMore?: () => void;
}) {
  const chosen = items.find((item) => item.value === value);

  /*
   * Догрузка по прокрутке: следующая страница просится, когда до низа
   * осталось меньше экрана списка. Порог, а не «доскроллил до конца», —
   * иначе список на мгновение упирается в дно и дёргается.
   *
   * Проверка на loading обязательна: событие прокрутки приходит десятками
   * подряд, и без неё одна и та же страница запрашивалась бы пачкой.
   */
  const onScroll = (event: UIEvent<HTMLDivElement>) => {
    if (!hasMore || loading || !onLoadMore) return;

    const { scrollTop, scrollHeight, clientHeight } = event.currentTarget;
    if (scrollHeight - scrollTop - clientHeight < clientHeight) onLoadMore();
  };

  return (
    <Popover
      trigger={({ open, toggle }) => (
        <button
          type="button"
          onClick={toggle}
          aria-expanded={open}
          aria-haspopup="menu"
          {...(ariaLabel ? { "aria-label": ariaLabel } : {})}
          className={`flex h-(--spacing-input) w-full items-center gap-2 rounded-md border bg-surface px-2.5 text-left text-sm transition-colors ${
            open ? "border-accent" : "border-border-strong hover:border-fg-subtle"
          } ${className}`}
        >
          <span className={`flex-1 truncate ${chosen ? "text-fg" : "text-fg-subtle"}`}>
            {chosen?.label ?? placeholder}
          </span>
          <Icon
            as={IconChevronDown}
            size={14}
            className={`shrink-0 text-fg-subtle transition-transform ${open ? "rotate-180" : ""}`}
          />
        </button>
      )}
    >
      {(close) => (
        <>
          {/* Поиск снаружи прокрутки: он ищет по всему списку, а не
              по видимой его части, и уезжать вверх вместе со строками
              ему незачем. */}
          {onSearch && (
            <label className="mb-1 flex h-7 items-center gap-1.5 rounded-md border border-border px-2 text-sm">
              <Icon as={IconSearch} size={14} className="shrink-0 text-fg-subtle" />
              <input
                autoFocus
                value={search ?? ""}
                onChange={(event) => onSearch(event.target.value)}
                placeholder={searchPlaceholder}
                className="min-w-0 flex-1 bg-transparent text-fg outline-none placeholder:text-fg-subtle"
              />
            </label>
          )}

          {/* Список бывает длиннее экрана — прокручиваем его, а не страницу.
              Popover подкручивает себя в видимую часть, но высоту не
              ограничивает: это дело содержимого. */}
          <div className="max-h-64 overflow-y-auto" onScroll={onScroll}>
            {items.map((item) => (
              <PopoverItem
                key={item.value}
                active={item.value === value}
                {...(item.value === value
                  ? { trailing: <Icon as={IconCheck} size={14} className="shrink-0" /> }
                  : {})}
                onClick={() => {
                  onChange(item.value);
                  // Выбор одиночный: после него выбирать больше нечего.
                  close();
                }}
              >
                {item.label}
              </PopoverItem>
            ))}

            {/* «Загружаю» ниже строк, а не вместо них: догрузка не должна
                смахивать с экрана то, что уже прочитали. */}
            {loading && <p className="px-2 py-2 text-2xs text-fg-subtle">…</p>}

            {!items.length && !loading && emptyText && (
              <p className="px-2 py-2 text-2xs text-fg-subtle">{emptyText}</p>
            )}
          </div>
        </>
      )}
    </Popover>
  );
}
