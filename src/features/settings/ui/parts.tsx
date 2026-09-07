import type { ReactNode } from "react";
import { IconChevronLeft, IconChevronRight } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/shared/ui/icon";
import { GAP, pageCount, pageItems } from "@/shared/ui/pagination";

/**
 * Общие части списочных разделов настроек: шапка, таблица, страницы.
 *
 * Разделов таких пять, и все они устроены одинаково — заголовок,
 * кнопка «добавить», таблица, страницы. Пять копий одних и тех же
 * классов разъезжаются на первой же правке отступа: в старой админке
 * ровно так и вышло — свой `CTable` со своими стилями почти в каждом
 * модуле.
 *
 * В shared/ это не уезжает намеренно: за пределами настроек списков
 * такого вида нет, а таблица данных у нас своя и совсем другая.
 */

export function SectionHeader({
  title,
  hint,
  children,
}: {
  title: string;
  /** Одна строка о том, чего это касается. Не инструкция. */
  hint?: string;
  /** Поиск и кнопки — у правого края. */
  children?: ReactNode;
}) {
  return (
    <header className="flex shrink-0 items-start gap-3 border-b border-border px-4 py-3">
      <div className="min-w-0 flex-1">
        <h3 className="text-sm font-medium text-fg">{title}</h3>
        {hint && <p className="mt-0.5 text-xs text-fg-subtle">{hint}</p>}
      </div>

      {children && <div className="flex shrink-0 items-center gap-2">{children}</div>}
    </header>
  );
}

/** Заголовок колонки. Липкий: у списков своя прокрутка. */
export function Th({ children, className = "" }: { children?: ReactNode; className?: string }) {
  return (
    <th
      className={`sticky top-0 z-10 h-9 border-b border-border bg-surface px-3 text-left text-xs font-normal text-fg-muted ${className}`}
    >
      {children}
    </th>
  );
}

export function Td({
  children,
  className = "",
  colSpan,
}: {
  children?: ReactNode;
  className?: string;
  colSpan?: number;
}) {
  return (
    <td colSpan={colSpan} className={`h-10 border-b border-border px-3 text-sm text-fg ${className}`}>
      {children}
    </td>
  );
}

/** Пусто — это ответ, а не ошибка: так и пишем словами. */
export function Empty({ text, colSpan }: { text: string; colSpan: number }) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-3 py-6 text-center text-sm text-fg-subtle">
        {text}
      </td>
    </tr>
  );
}

/**
 * Страницы. Логика номеров — общая (`shared/ui/pagination`), та же,
 * что у таблицы данных: полоса постоянной ширины, чтобы кнопки
 * не разъезжались под курсором.
 */
export function Pager({
  page,
  total,
  limit,
  onPage,
}: {
  page: number;
  total: number;
  limit: number;
  onPage: (page: number) => void;
}) {
  const { t } = useTranslation();
  const pages = pageCount(total, limit);

  if (pages <= 1) return null;

  const button =
    "inline-flex size-7 items-center justify-center rounded-md text-sm transition-colors";

  return (
    <div className="flex h-11 shrink-0 items-center justify-end gap-1 border-t border-border px-3">
      <button
        type="button"
        onClick={() => onPage(page - 1)}
        disabled={page <= 1}
        aria-label={t("table.prevPage")}
        className={`${button} text-fg-muted hover:bg-surface-hover hover:text-fg disabled:opacity-30`}
      >
        <Icon as={IconChevronLeft} size={14} />
      </button>

      {pageItems(page, pages).map((item, index) =>
        item === GAP ? (
          <span key={`${GAP}${index}`} className={`${button} text-fg-subtle`} aria-hidden>
            …
          </span>
        ) : (
          <button
            key={item}
            type="button"
            onClick={() => onPage(item)}
            aria-current={item === page ? "page" : undefined}
            className={`${button} ${
              item === page
                ? "bg-accent-subtle text-accent-text"
                : "text-fg-muted hover:bg-surface-hover hover:text-fg"
            }`}
          >
            {item}
          </button>
        ),
      )}

      <button
        type="button"
        onClick={() => onPage(page + 1)}
        disabled={page >= pages}
        aria-label={t("table.nextPage")}
        className={`${button} text-fg-muted hover:bg-surface-hover hover:text-fg disabled:opacity-30`}
      >
        <Icon as={IconChevronRight} size={14} />
      </button>
    </div>
  );
}

/**
 * Дата так, как её понимает человек. Мусор не показываем вовсе —
 * пустая ячейка честнее, чем «Invalid Date».
 */
export function formatDateTime(value: string, locale: string): string {
  if (!value) return "";

  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toLocaleString(locale);
}
