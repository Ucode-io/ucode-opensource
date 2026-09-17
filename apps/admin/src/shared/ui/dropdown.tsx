import type { ReactNode, UIEvent } from "react";
import { IconCheck, IconChevronDown, IconSearch } from "@tabler/icons-react";
import { Icon } from "./icon";
import { Popover, PopoverItem } from "./popover";

/**
 * Значок необязателен и нужен там, где у варианта есть силуэт: форма
 * графика узнаётся по нему быстрее, чем по названию. Список без значков
 * от этого не меняется — отступа под пустое место не появляется.
 */
export type DropdownItem = { value: string; label: string; icon?: ReactNode };

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
 * Выбор одиночный. Множественный — у `SelectMenu`: там флажки и список
 * не закрывается после каждой строки.
 */
export function Dropdown({
  value,
  items,
  placeholder = "",
  ariaLabel,
  className = "",
  size = "md",
  disabled = false,
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
  /**
   * Высота кнопки. `sm` — для панели настроек поля и строк отбора, где
   * поля стоят в столбик по десятку и в полный рост не помещаются.
   * `control` — вровень с кнопкой и полоской вкладок (`--spacing-control`,
   * те же 32px), для строки, где дропдаун стоит рядом с `Tabs`: `sm`
   * там читается тесно, а `md` (36px, высота `Input`) выше вкладок.
   * Отдельным свойством, а не классом снаружи: у конфликтующих утилит
   * Tailwind побеждает не та, что стоит позже в атрибуте, а та, что
   * позже в собранном CSS, — и высота получалась бы через раз.
   */
  size?: "sm" | "md" | "control";
  disabled?: boolean;
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
      /* Ширина уезжает на обёртку, а не на кнопку. Две причины, и обе
         одинаково ломают вид: у `<button>` ширина `auto` считается
         по содержимому даже когда он блочный (кнопка сжималась
         в горошину), а `w-40` снаружи всё равно не победил бы `w-full`
         в базовом классе — у Tailwind выигрывает не тот класс, что
         стоит позже в атрибуте, а тот, что позже в собранном CSS. */
      className={className}
      trigger={({ open, toggle }) => (
        <button
          type="button"
          onClick={toggle}
          disabled={disabled}
          aria-expanded={open}
          aria-haspopup="menu"
          {...(ariaLabel ? { "aria-label": ariaLabel } : {})}
          className={`flex w-full items-center gap-2 rounded-md border bg-surface text-left transition-colors disabled:opacity-50 ${
            size === "sm"
              ? "h-7 px-1.5 text-xs"
              : size === "control"
                ? "h-(--spacing-control) px-2.5 text-sm"
                : "h-(--spacing-input) px-2.5 text-sm"
          } ${open ? "border-accent" : "border-border-strong hover:border-fg-subtle"}`}
        >
          {/* Значок выбранного — на кнопке, а не только в раскрытом
              списке: иначе он виден ровно в тот момент, когда уже
              не нужен. */}
          {chosen?.icon}
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
            <label className="mb-1 flex h-7 shrink-0 items-center gap-1.5 rounded-md border border-border px-2 text-sm">
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
              Высоту задаёт Popover по месту на экране; здесь берём остаток
              после поиска, чтобы тот остался на виду. */}
          <div className="max-h-64 min-h-0 flex-1 overflow-y-auto" onScroll={onScroll}>
            {items.map((item) => (
              <PopoverItem
                key={item.value}
                active={item.value === value}
                {...(item.icon ? { icon: item.icon } : {})}
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
