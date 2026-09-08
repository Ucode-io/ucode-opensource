import { useState } from "react";
import {
  IconCheck,
  IconChevronLeft,
  IconChevronRight,
  IconLoader2,
  IconTrash,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Icon } from "@/shared/ui/icon";
import { GAP, pageCount, pageItems } from "@/shared/ui/pagination";
import { Popover } from "@/shared/ui/popover";

/**
 * Подвал таблицы: размер порции, действия над отмеченными и — смотря
 * как настроен view — либо номера страниц, либо счётчик загруженного.
 *
 * Два режима, потому что списки читают по-разному: справочник листают
 * страницами и возвращаются на седьмую, журнал событий крутят вниз.
 * Выбирает режим тот, кто настроил экран (см. View.infiniteScroll),
 * а не тот, кто на него зашёл.
 *
 * У прокрутки счётчик обязателен: без него непонятно, всё ли уже видно,
 * а «показано 60 из 4321» отвечает на это без единого щелчка.
 *
 * Размер порции виден всегда: он же — настройка view (default_limit),
 * и от него зависит, сколько строк приезжает за раз.
 */
export const PAGE_SIZES = [20, 50, 100, 200] as const;

/** Границы своего значения. Верхняя — чтобы «99999» не уронил вкладку. */
export const MIN_LIMIT = 1;
export const MAX_LIMIT = 1000;

const pageButton =
  "inline-flex size-7 items-center justify-center rounded-md text-sm transition-colors";

export function GridFooter({
  page,
  shown,
  limit,
  total,
  loadingMore,
  selectedCount,
  deleting,
  onPage,
  onLimit,
  onDeleteSelected,
}: {
  /** Открытая страница. Не задана — строки догружаются прокруткой. */
  page?: number | undefined;
  /** Сколько строк уже загружено и лежит в таблице. */
  shown: number;
  limit: number;
  total: number;
  /** Едет следующая порция: счётчик показывает это, а не таблица. */
  loadingMore?: boolean;
  selectedCount: number;
  deleting: boolean;
  /** Переход на страницу. Не задан — режим прокрутки. */
  onPage?: ((page: number) => void) | undefined;
  onLimit: (limit: number) => void;
  /**
   * Удалить отмеченные. Не задан — кнопки нет: у роли без права
   * на удаление она отвечала бы 403, а во вкладке связи удалять
   * чужие строки не предлагают вовсе.
   */
  onDeleteSelected?: (() => void) | undefined;
}) {
  const { t } = useTranslation();
  const pages = pageCount(total, limit);

  return (
    <div className="flex h-11 shrink-0 items-center justify-between gap-4 border-t border-border px-3">
      <div className="flex min-w-0 items-center gap-3">
        <LimitPicker limit={limit} onLimit={onLimit} />

        {/* Действия появляются вместе с выделением и занимают место
            только тогда: пустая панель действий сбивает с толку. */}
        {selectedCount > 0 && onDeleteSelected && (
          <Button variant="danger" size="sm" disabled={deleting} onClick={onDeleteSelected}>
            <Icon as={IconTrash} size={14} />
            {t("table.deleteSelected", { count: selectedCount })}
          </Button>
        )}
      </div>

      {page === undefined || !onPage ? (
        <p className="flex shrink-0 items-center gap-1.5 text-xs text-fg-muted tabular-nums">
          {loadingMore && <Icon as={IconLoader2} size={12} className="animate-spin" />}
          {t("table.shownOf", { shown, total })}
        </p>
      ) : (
        <nav className="flex items-center gap-0.5" aria-label={t("table.pages")}>
          <Step
            label={t("table.prevPage")}
            icon={IconChevronLeft}
            disabled={page <= 1}
            onClick={() => onPage(page - 1)}
          />

          {pageItems(page, pages).map((item, index) =>
            item === GAP ? (
              // Многоточие — не кнопка: нажимать там нечего.
              <span key={`${GAP}${index}`} className={`${pageButton} text-fg-subtle`} aria-hidden>
                …
              </span>
            ) : (
              <button
                key={item}
                type="button"
                aria-current={item === page ? "page" : undefined}
                onClick={() => onPage(item)}
                className={`${pageButton} ${
                  item === page
                    ? "bg-accent-solid text-accent-fg"
                    : "text-fg-muted hover:bg-surface-hover hover:text-fg"
                }`}
              >
                {item}
              </button>
            ),
          )}

          <Step
            label={t("table.nextPage")}
            icon={IconChevronRight}
            disabled={page >= pages}
            onClick={() => onPage(page + 1)}
          />
        </nav>
      )}
    </div>
  );
}

function Step({
  label,
  icon,
  disabled,
  onClick,
}: {
  label: string;
  icon: typeof IconChevronLeft;
  disabled: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      disabled={disabled}
      onClick={onClick}
      className={`${pageButton} text-fg-muted hover:bg-surface-hover hover:text-fg disabled:pointer-events-none disabled:opacity-40`}
    >
      <Icon as={icon} size={16} />
    </button>
  );
}

/**
 * Размер страницы: готовые значения и своё.
 *
 * Своё нужно не ради красоты: у view есть настройка default_limit,
 * куда админ вписывает любое число, и список из четырёх кнопок не смог
 * бы показать текущее значение, если оно не из списка.
 */
function LimitPicker({ limit, onLimit }: { limit: number; onLimit: (limit: number) => void }) {
  const { t } = useTranslation();

  return (
    <Popover
      trigger={({ open, toggle }) => (
        <button
          type="button"
          onClick={toggle}
          aria-expanded={open}
          className="flex h-7 items-center gap-1.5 rounded-md border border-border-strong px-2 text-sm text-fg transition-colors hover:bg-surface-hover"
        >
          <span className="tabular-nums">{limit}</span>
          <span className="text-xs text-fg-muted">{t("table.perPage")}</span>
        </button>
      )}
    >
      {(close) => <LimitMenu limit={limit} onLimit={onLimit} close={close} />}
    </Popover>
  );
}

function LimitMenu({
  limit,
  onLimit,
  close,
}: {
  limit: number;
  onLimit: (limit: number) => void;
  close: () => void;
}) {
  const { t } = useTranslation();
  const [custom, setCustom] = useState(String(limit));

  const apply = () => {
    const value = Number(custom);
    // Мусор и выход за границы просто не применяем: тихо подставить
    // другое число хуже, чем ничего не сделать, — человек не заметит.
    if (!Number.isInteger(value) || value < MIN_LIMIT || value > MAX_LIMIT) return;
    onLimit(value);
    close();
  };

  return (
    <div className="flex w-44 flex-col">
      {PAGE_SIZES.map((size) => (
        <button
          key={size}
          type="button"
          role="menuitem"
          onClick={() => {
            onLimit(size);
            close();
          }}
          className="flex h-8 items-center justify-between rounded-md px-2 text-sm text-fg transition-colors hover:bg-surface-hover"
        >
          <span className="tabular-nums">{size}</span>
          {size === limit && <Icon as={IconCheck} size={14} className="text-accent-text" />}
        </button>
      ))}

      <div className="my-1 h-px bg-border" />

      <form
        className="flex items-center gap-1 p-1"
        onSubmit={(event) => {
          event.preventDefault();
          apply();
        }}
      >
        <input
          type="number"
          min={MIN_LIMIT}
          max={MAX_LIMIT}
          value={custom}
          onChange={(event) => setCustom(event.target.value)}
          aria-label={t("table.customLimit")}
          className="h-7 w-full min-w-0 rounded-md border border-border-strong bg-surface px-2 text-sm tabular-nums text-fg"
        />
        <Button type="submit" size="sm">
          {t("action.apply")}
        </Button>
      </form>
    </div>
  );
}
