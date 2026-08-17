import { useState, type UIEvent } from "react";
import { IconCheck, IconChevronDown, IconSearch, IconTable } from "@tabler/icons-react";
import { Checkbox } from "./checkbox";
import { DynamicIcon } from "./dynamic-icon";
import { Icon } from "./icon";

export type SelectItem = {
  value: string;
  label: string;
  /**
   * Значок в том формате, в каком его отдаёт бэкенд, — «mdi:home»,
   * ссылка или файл в нашем CDN (см. DynamicIcon). Пусто — рисуется
   * запасной значок таблицы, а не пустое место: строка со значком
   * и строка без него встали бы с разным отступом.
   */
  icon?: string;
};

/**
 * Выпадающий список с поиском и догрузкой по прокрутке.
 *
 * Раскрывается НА МЕСТЕ, а не всплывающим слоем: живёт он внутри
 * панели поля, которая сама всплывашка со своим Escape и своим
 * закрытием по клику мимо. Второй такой слой поверх первого означал бы
 * два обработчика Escape на одно нажатие — закрылось бы и меню,
 * и панель под ним.
 *
 * Загрузку и поиск ведёт вызывающий: `shared/` не знает ни про запросы,
 * ни про то, откуда берутся строки.
 */
export function SelectMenu({
  label,
  placeholder,
  searchPlaceholder,
  emptyText,
  items,
  selected,
  multiple = false,
  search,
  loading = false,
  hasMore = false,
  onSearch,
  onLoadMore,
  onPick,
}: {
  /** Подпись над кнопкой. */
  label: string;
  /** Что показывать, когда ничего не выбрано. */
  placeholder: string;
  searchPlaceholder: string;
  emptyText: string;
  items: SelectItem[];
  /** Выбранные значения. У одиночного списка их ноль или одно. */
  selected: ReadonlySet<string>;
  multiple?: boolean;
  search: string;
  loading?: boolean;
  /** Есть ли ещё страница. Пусто — прокрутка ничего не догружает. */
  hasMore?: boolean;
  onSearch: (query: string) => void;
  onLoadMore: () => void;
  onPick: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);

  /*
   * Догрузка по прокрутке: следующая страница просится, когда до низа
   * осталось меньше экрана списка. Порог, а не «доскроллил до конца»,
   * — иначе список на мгновение упирается в дно и дёргается.
   *
   * Проверка на loading обязательна: событие прокрутки приходит десятками
   * подряд, и без неё одна и та же страница запрашивалась бы пачкой.
   */
  const onScroll = (event: UIEvent<HTMLDivElement>) => {
    if (!hasMore || loading) return;

    const { scrollTop, scrollHeight, clientHeight } = event.currentTarget;
    if (scrollHeight - scrollTop - clientHeight < clientHeight) onLoadMore();
  };

  const chosen = items.filter((item) => selected.has(item.value));
  /*
   * Подпись кнопки. У множественного — сколько выбрано, а не перечисление:
   * пять полей в строку не помещаются, а обрезанное «Логин, Роль, Ава…»
   * не говорит ни сколько их, ни какие.
   */
  const summary = multiple
    ? selected.size
      ? `${selected.size}`
      : ""
    : (chosen[0]?.label ?? [...selected][0] ?? "");

  return (
    <div>
      <p className="px-1 pb-1 text-2xs text-fg-subtle">{label}</p>

      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        className="mx-1 flex h-8 w-[calc(100%-0.5rem)] items-center gap-2 rounded-md border border-border-strong px-2 text-left text-sm transition-colors hover:border-fg-subtle"
      >
        {/* Значок выбранного — на кнопке тоже: иначе выбор со значком
            превращается в строку текста, как только список закрыли. */}
        {!multiple && chosen[0] && <OptionIcon icon={chosen[0].icon} />}

        <span className={`flex-1 truncate ${summary ? "text-fg" : "text-fg-subtle"}`}>
          {summary || placeholder}
        </span>
        <Icon
          as={IconChevronDown}
          size={14}
          className={`shrink-0 text-fg-subtle transition-transform ${open ? "rotate-180" : ""}`}
        />
      </button>

      {open && (
        <div className="mx-1 mt-1 rounded-md border border-border">
          <label className="flex h-7 items-center gap-1.5 border-b border-border px-2 text-sm">
            <Icon as={IconSearch} size={14} className="shrink-0 text-fg-subtle" />
            <input
              autoFocus
              value={search}
              onChange={(event) => onSearch(event.target.value)}
              placeholder={searchPlaceholder}
              className="min-w-0 flex-1 bg-transparent text-fg outline-none placeholder:text-fg-subtle"
            />
          </label>

          <div className="max-h-48 overflow-y-auto p-1" onScroll={onScroll}>
            {items.map((item) => {
              const picked = selected.has(item.value);

              return (
                <button
                  key={item.value}
                  type="button"
                  onClick={() => {
                    onPick(item.value);
                    // Одиночный выбор закрывает список: выбирать больше нечего.
                    if (!multiple) setOpen(false);
                  }}
                  className={`flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors ${
                    picked && !multiple
                      ? "bg-accent-subtle text-accent-text"
                      : "text-fg hover:bg-surface-hover"
                  }`}
                >
                  {multiple && <Checkbox checked={picked} onChange={() => onPick(item.value)} />}
                  {!multiple && <OptionIcon icon={item.icon} />}
                  <span className="flex-1 truncate">{item.label}</span>
                  {picked && !multiple && <Icon as={IconCheck} size={14} className="shrink-0" />}
                </button>
              );
            })}

            {/* «Загружаю» ниже строк, а не вместо них: догрузка не должна
                смахивать с экрана то, что уже прочитали. */}
            {loading && <p className="px-2 py-2 text-2xs text-fg-subtle">…</p>}

            {!items.length && !loading && (
              <p className="px-2 py-2 text-2xs text-fg-subtle">{emptyText}</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * Значок строки списка. Запасной — таблица: списки со значками у нас
 * пока только табличные, а пустое место на месте значка сбивает
 * выравнивание соседних строк.
 */
function OptionIcon({ icon }: { icon?: string | undefined }) {
  return (
    <span className="grid size-4 shrink-0 place-items-center text-fg-muted">
      <DynamicIcon
        name={icon ?? ""}
        size={14}
        fallback={<Icon as={IconTable} size={14} />}
      />
    </span>
  );
}
