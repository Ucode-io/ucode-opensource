import { useState } from "react";
import { IconCheck, IconLoader2, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Icon } from "@/shared/ui/icon";
import { Popover } from "@/shared/ui/popover";

/**
 * Подвал таблицы: сколько строк показано, каким куском они грузятся
 * и действия над отмеченными.
 *
 * Номеров страниц нет: строки догружаются прокруткой. Счётчик вместо
 * них обязателен — иначе непонятно, всё ли уже видно, а «показано 60
 * из 4321» отвечает на это без единого щелчка.
 *
 * Размер куска виден и переключается: он же — настройка view
 * (default_limit), и от него зависит, сколько строк приезжает за раз.
 */
export const PAGE_SIZES = [20, 50, 100, 200] as const;

/** Границы своего значения. Верхняя — чтобы «99999» не уронил вкладку. */
export const MIN_LIMIT = 1;
export const MAX_LIMIT = 1000;

export function GridFooter({
  shown,
  limit,
  total,
  loadingMore,
  selectedCount,
  deleting,
  onLimit,
  onDeleteSelected,
}: {
  /** Сколько строк уже загружено и лежит в таблице. */
  shown: number;
  limit: number;
  total: number;
  /** Едет следующий кусок: счётчик показывает это, а не таблица. */
  loadingMore?: boolean;
  selectedCount: number;
  deleting: boolean;
  onLimit: (limit: number) => void;
  onDeleteSelected: () => void;
}) {
  const { t } = useTranslation();

  return (
    <div className="flex h-11 shrink-0 items-center justify-between gap-4 border-t border-border px-3">
      <div className="flex min-w-0 items-center gap-3">
        <LimitPicker limit={limit} onLimit={onLimit} />

        {/* Действия появляются вместе с выделением и занимают место
            только тогда: пустая панель действий сбивает с толку. */}
        {selectedCount > 0 && (
          <Button variant="danger" size="sm" disabled={deleting} onClick={onDeleteSelected}>
            <Icon as={IconTrash} size={14} />
            {t("table.deleteSelected", { count: selectedCount })}
          </Button>
        )}
      </div>

      <p className="flex shrink-0 items-center gap-1.5 text-xs text-fg-muted tabular-nums">
        {loadingMore && <Icon as={IconLoader2} size={12} className="animate-spin" />}
        {t("table.shownOf", { shown, total })}
      </p>
    </div>
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
