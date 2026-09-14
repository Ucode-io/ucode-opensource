import type { ReactNode } from "react";
import { IconChevronLeft, IconChevronRight } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
/* Размеры страницы — те же, что у таблицы данных: это один и тот же
   вопрос «сколько строк показывать», и два разных набора значений
   разошлись бы на первой же правке. */
import { PAGE_SIZES } from "@/features/item";
import { Chip, type ChipColor } from "@/shared/ui/chip";
import { Dropdown } from "@/shared/ui/dropdown";
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
 *
 * Слева — счётчик и размер страницы, справа — номера. Счётчик тут
 * не украшение: без него «20 строк» на экране не отличить от «20 строк
 * всего», а в журнале это разные ответы. Поэтому полоса рисуется
 * и тогда, когда страница одна: листать нечего, а знать сколько всего
 * — есть зачем. Пусто — полосы нет вовсе: «Показано 0 из 0» под пустой
 * таблицей повторяет то, что она уже сказала словами.
 *
 * Размер страницы появляется только у того, кто передал `onLimit`:
 * у списка, где число строк задано жёстко, выбор из четырёх значений
 * был бы кнопкой, которая ничего не меняет.
 */
export function Pager({
  page,
  total,
  limit,
  onPage,
  onLimit,
}: {
  page: number;
  total: number;
  limit: number;
  onPage: (page: number) => void;
  onLimit?: ((limit: number) => void) | undefined;
}) {
  const { t } = useTranslation();
  const pages = pageCount(total, limit);

  if (!total) return null;

  const button =
    "inline-flex size-7 items-center justify-center rounded-md text-sm transition-colors";

  return (
    <div className="flex h-11 shrink-0 items-center justify-between gap-3 border-t border-border px-3">
      <div className="flex min-w-0 items-center gap-2">
        <span className="text-xs text-fg-muted tabular-nums">
          {/* Диапазон, а не число загруженного, как в таблице данных:
              там строки копятся прокруткой, здесь страница ровно одна
              и важно, какая именно. Ключ тот же. */}
          {t("table.shownOf", {
            shown: `${(page - 1) * limit + 1}–${Math.min(page * limit, total)}`,
            total,
          })}
        </span>

        {onLimit && (
          <>
            <div className="w-20">
              <Dropdown
                value={String(limit)}
                items={PAGE_SIZES.map((size) => ({ value: String(size), label: String(size) }))}
                ariaLabel={t("table.customLimit")}
                size="sm"
                onChange={(value) => onLimit(Number(value))}
              />
            </div>
            <span className="text-xs text-fg-muted">{t("table.perPage")}</span>
          </>
        )}
      </div>

      {/* Номера прячутся, а счётчик остаётся: на одной странице листать
          нечего, но «сколько всего» спрашивают и там. */}
      {pages > 1 && (
        <nav className="flex shrink-0 items-center gap-1" aria-label={t("table.pages")}>
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
        </nav>
      )}
    </div>
  );
}

/**
 * Метод запроса плашкой, а не словом в общей строке.
 *
 * В журнале полсотни строк на экран, и глазами по ним ищут не «что
 * случилось», а «где мой DELETE». Слово одного цвета среди других слов
 * для этого перечитывают целиком, цветная плашка — узнаётся формой
 * и оттенком, не читаясь.
 *
 * Оттенки — из палитры чипов (`shared/ui/chip`): она уже проверена
 * в обеих темах, и это тот же набор, которым красятся значения Status
 * в таблицах. Свои цвета здесь означали бы десятый набор в системе.
 * Соответствие привычное (зелёный — создание, красный — удаление),
 * и незнакомый метод не выдумывает себе цвет, а остаётся серым.
 */
const METHOD_COLOR: Record<string, ChipColor> = {
  GET: "blue",
  POST: "green",
  PUT: "orange",
  PATCH: "orange",
  DELETE: "red",
};

export function MethodBadge({ method }: { method: string }) {
  const name = method.trim().toUpperCase();
  if (!name) return null;

  return <Chip color={METHOD_COLOR[name] ?? "gray"}>{name}</Chip>;
}

/**
 * Тип действия плашкой: «CREATE VIEW», «DELETE ITEM», «GET_LIST».
 *
 * Цвет берётся по ГЛАГОЛУ — первому слову. Оно и есть то, что ищут
 * глазами в журнале: «кто удалил» спрашивают куда чаще, чем «что было
 * с view». Сущность после глагола цвета не меняет: иначе пришлось бы
 * держать таблицу из четырёх десятков значений, которая растёт вместе
 * с ручками бэкенда и устареет молча — ровно то, из-за чего отбор
 * по типу здесь поиск, а не список.
 *
 * Оттенки те же, что у методов запроса, и по тому же смыслу: создание
 * зелёное, удаление красное, правка оранжевая, чтение синее. Глагол
 * не из списка остаётся серым, а не выдумывает себе цвет.
 *
 * `GET_LIST` и `GET_ITEM` подводятся под `GET`: подчёркивание здесь
 * — не другой глагол, а уточнение.
 */
const ACTION_COLOR: Record<string, ChipColor> = {
  CREATE: "green",
  UPDATE: "orange",
  UPSERT: "orange",
  DELETE: "red",
  GET: "blue",
};

export function ActionBadge({ action }: { action: string }) {
  const name = action.trim().toUpperCase();
  if (!name) return null;

  const verb = name.split(/[\s_]/)[0] ?? "";

  return <Chip color={ACTION_COLOR[verb] ?? "gray"}>{name}</Chip>;
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
