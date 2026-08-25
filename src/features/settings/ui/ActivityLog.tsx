import { useDeferredValue, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import {
  ACTIVITY_PAGE,
  NO_FILTERS,
  useActivity,
  useActivityEntry,
  type ActivityFilters,
} from "../api/activity";
import { unwrapEntry } from "../model/activity";
import { Empty, Pager, SectionHeader, Td, Th, formatDateTime } from "./parts";

/**
 * Журнал изменений: кто, когда и что поменял.
 *
 * Запись хранит «было» и «стало», поэтому открытая строка отвечает
 * не только «кто трогал таблицу», но и «что именно в ней стало другим»
 * — ради этого журнал и читают.
 *
 * Отбор по подстроке, а не выбором из списка: типов действий в базе
 * четыре десятка (`CREATE ITEM`, `UPDATE FIELD`, `DELETE MENU`…),
 * список растёт вместе с ручками бэкенда, и зашитый в код перечень
 * устарел бы молча — так же, как он устарел в старой админке.
 * Сервер и сам сравнивает их через ILIKE.
 */
export function ActivityLog() {
  const { t, i18n } = useTranslation();

  const [filters, setFilters] = useState<ActivityFilters>(NO_FILTERS);
  const [page, setPage] = useState(1);

  /*
   * Текстовые поля отложены, даты — нет: дату вводят целиком и разом,
   * а буквы по одной. Ключ запроса собирается из отложенного значения,
   * поэтому лишних запросов на каждую букву не уходит.
   */
  const deferred = useDeferredValue(filters);
  const { entries, count, isLoading } = useActivity(deferred, page);

  const [opened, setOpened] = useState("");

  const put = (patch: Partial<ActivityFilters>) => {
    setFilters((current) => ({ ...current, ...patch }));
    setPage(1);
  };

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("activity.title")} hint={t("activity.hint")} />

      {/*
        Ширину задаёт обёртка, а не само поле: у `Input` в базовых
        классах стоит `w-full`, и своя ширина на нём — спор двух
        одинаковых по весу правил, который выигрывает не тот, кто
        написан последним.
      */}
      <div className="flex shrink-0 flex-wrap items-end gap-2 border-b border-border px-4 py-2">
        <div className="w-40">
          <Input
            value={filters.action}
            onChange={(event) => put({ action: event.target.value })}
            placeholder={t("activity.action")}
            aria-label={t("activity.action")}
            className="h-7 text-sm"
          />
        </div>

        <div className="w-40">
          <Input
            value={filters.table}
            onChange={(event) => put({ table: event.target.value })}
            placeholder={t("activity.table")}
            aria-label={t("activity.table")}
            className="h-7 text-sm"
          />
        </div>

        <div className="w-40">
          <Input
            value={filters.user}
            onChange={(event) => put({ user: event.target.value })}
            placeholder={t("activity.user")}
            aria-label={t("activity.user")}
            className="h-7 text-sm"
          />
        </div>

        {/* Нативные поля даты: календарь, локальный формат и ввод
            с клавиатуры — бесплатно, как в фильтрах таблицы. */}
        <div className="w-36">
          <Input
            type="date"
            value={filters.from}
            onChange={(event) => put({ from: event.target.value })}
            aria-label={t("activity.from")}
            className="h-7 text-sm"
          />
        </div>

        <div className="w-36">
          <Input
            type="date"
            value={filters.to}
            onChange={(event) => put({ to: event.target.value })}
            aria-label={t("activity.to")}
            className="h-7 text-sm"
          />
        </div>

        {/* Кнопка появляется по заполненности, а не по «трогали ли»:
            стёртое поле — это тот же пустой отбор. */}
        {Object.values(filters).some(Boolean) && (
          <Button
            size="sm"
            variant="ghost"
            onClick={() => {
              setFilters(NO_FILTERS);
              setPage(1);
            }}
          >
            {t("table.clearFilters")}
          </Button>
        )}
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th className="w-44">{t("activity.date")}</Th>
              <Th>{t("activity.action")}</Th>
              <Th>{t("activity.table")}</Th>
              <Th>{t("activity.user")}</Th>
              <Th className="w-24">{t("activity.status")}</Th>
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={5} />}
            {!isLoading && !entries.length && <Empty text={t("activity.empty")} colSpan={5} />}

            {entries.map((entry) => (
              <tr
                key={entry.id}
                onClick={() => setOpened(entry.id)}
                className="cursor-pointer hover:bg-surface-hover"
              >
                <Td className="text-fg-muted">{formatDateTime(entry.date, i18n.language)}</Td>
                <Td>{entry.action}</Td>
                <Td className="text-fg-muted">{entry.table}</Td>
                <Td className="text-fg-muted">{entry.user}</Td>
                <Td>
                  {/* Код ответа рисуется только тогда, когда он есть:
                      у записей до появления колонки он нулевой, и «0»
                      читалось бы как настоящий ответ. */}
                  {entry.statusCode > 0 && (
                    <span
                      className={
                        entry.statusCode < 400 ? "text-2xs text-fg-muted" : "text-2xs text-danger"
                      }
                    >
                      {entry.method} {entry.statusCode}
                    </span>
                  )}
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Pager page={page} total={count} limit={ACTIVITY_PAGE} onPage={setPage} />

      {opened && <EntryDialog id={opened} onClose={() => setOpened("")} />}
    </div>
  );
}

/**
 * Одна запись: что просили, что ответили, что было и что стало.
 *
 * Показываем всё четыре поля, а не только «стало»: по «было» видно,
 * что именно поменялось, а по запросу с ответом — кто и чем это сделал.
 * Пустые разделы не рисуются вовсе: у записи о чтении нет ни «было»,
 * ни «стало», и четыре подписи с прочерками только мешают.
 */
function EntryDialog({ id, onClose }: { id: string; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { entry, isLoading } = useActivityEntry(id);

  const parts = entry
    ? ([
        ["activity.before", unwrapEntry(entry.before)],
        ["activity.after", unwrapEntry(entry.after)],
        ["activity.request", unwrapEntry(entry.request)],
        ["activity.response", unwrapEntry(entry.response)],
      ] as const).filter(([, value]) => value)
    : [];

  return (
    <Modal onClose={onClose}>
      <div className="flex max-h-[85vh] w-full max-w-3xl flex-col rounded-xl border border-border bg-surface shadow-modal">
        <header className="flex shrink-0 items-start gap-3 border-b border-border px-5 py-3">
          <div className="min-w-0 flex-1">
            <h2 className="text-base font-semibold">{entry?.action ?? t("common.loading")}</h2>
            <p className="mt-0.5 text-xs text-fg-subtle">
              {[
                entry && formatDateTime(entry.date, i18n.language),
                entry?.user,
                entry?.table,
                entry?.duration ? `${entry.duration} ms` : "",
              ]
                .filter(Boolean)
                .join(" · ")}
            </p>
          </div>

          <Button size="sm" variant="ghost" onClick={onClose}>
            {t("action.close")}
          </Button>
        </header>

        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-5">
          {isLoading && <p className="text-sm text-fg-muted">{t("common.loading")}</p>}
          {!isLoading && !parts.length && (
            <p className="text-sm text-fg-subtle">{t("activity.noPayload")}</p>
          )}

          {parts.map(([key, value]) => (
            <section key={key}>
              <h3 className="mb-1 text-xs font-medium text-fg-muted">{t(key)}</h3>
              {/* Своя горизонтальная прокрутка: длинная строка JSON
                  иначе растягивает окно шире экрана. */}
              <pre className="max-h-64 overflow-auto rounded-md bg-bg p-3 font-mono text-xs text-fg">
                {value}
              </pre>
            </section>
          ))}
        </div>
      </div>
    </Modal>
  );
}
