import { useDeferredValue, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { DatePicker } from "@/shared/ui/date-picker";
import { Input } from "@/shared/ui/input";
import {
  FUNCTION_LOGS_PAGE,
  NO_LOG_FILTERS,
  useFunctionLogs,
  type FunctionLogFilters,
} from "../api/function-logs";
import { Empty, Pager, SectionHeader, Td, Th, formatDateTime } from "./parts";

/**
 * Выполнение функций: когда вызвали, что вызвали и чем кончилось.
 *
 * Вкладка рядом с журналом изменений, а не строки в нём: предмет
 * другой — не правка проекта, а вызов, — и колонки не совпадают
 * ни одной, кроме даты.
 *
 * Раскрытой записи нет: в отличие от журнала изменений, где хранятся
 * «было» и «стало», здесь у строки нет ни тела запроса, ни ответа —
 * `FunctionLogModel` их не несёт (`pg_version_history.proto:70`).
 * Показывать пустое окно по щелчку хуже, чем не открывать его.
 */
export function FunctionLogs() {
  const { t, i18n } = useTranslation();

  const [filters, setFilters] = useState<FunctionLogFilters>(NO_LOG_FILTERS);
  const [page, setPage] = useState(1);

  const deferred = useDeferredValue(filters);
  const { logs, count, isLoading, error } = useFunctionLogs(deferred, page);

  const put = (patch: Partial<FunctionLogFilters>) => {
    setFilters((current) => ({ ...current, ...patch }));
    setPage(1);
  };

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("functionLogs.title")} hint={t("functionLogs.hint")} />

      <div className="flex shrink-0 flex-wrap items-end gap-2 border-b border-border px-4 py-2">
        <div className="w-44">
          <Input
            value={filters.search}
            onChange={(event) => put({ search: event.target.value })}
            placeholder={t("functionLogs.search")}
            aria-label={t("functionLogs.search")}
            className="h-7 text-sm"
          />
        </div>

        {/*
          Статус — поле ввода, а не список: перечня значений у бэкенда
          нет. Колонка `status` свободная строка, и сервер сравнивает её
          сам — зашитый в код набор устарел бы молча, как это уже вышло
          с типами действий в соседней вкладке.
        */}
        <div className="w-32">
          <Input
            value={filters.status}
            onChange={(event) => put({ status: event.target.value })}
            placeholder={t("activity.status")}
            aria-label={t("activity.status")}
            className="h-7 text-sm"
          />
        </div>

        <div className="w-36">
          <Input
            value={filters.table}
            onChange={(event) => put({ table: event.target.value })}
            placeholder={t("activity.table")}
            aria-label={t("activity.table")}
            className="h-7 text-sm"
          />
        </div>

        <div className="w-36">
          <DatePicker
            value={filters.from}
            locale={i18n.language}
            placeholder={t("activity.from")}
            ariaLabel={t("activity.from")}
            clearLabel={t("table.clearFilters")}
            onChange={(from) => put({ from })}
            className="h-7"
          />
        </div>

        <div className="w-36">
          <DatePicker
            value={filters.to}
            locale={i18n.language}
            placeholder={t("activity.to")}
            ariaLabel={t("activity.to")}
            clearLabel={t("table.clearFilters")}
            onChange={(to) => put({ to })}
            className="h-7"
          />
        </div>

        {Object.values(filters).some(Boolean) && (
          <Button
            size="sm"
            variant="ghost"
            onClick={() => {
              setFilters(NO_LOG_FILTERS);
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
              <Th>{t("functionLogs.function")}</Th>
              <Th>{t("activity.table")}</Th>
              <Th className="w-32">{t("actions.type")}</Th>
              <Th className="w-24">{t("functionLogs.duration")}</Th>
              <Th className="w-28">{t("activity.status")}</Th>
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={6} />}
            {!isLoading && !logs.length && (
              <Empty text={error ?? t("functionLogs.empty")} colSpan={6} />
            )}

            {logs.map((log) => (
              <tr key={log.id} className="hover:bg-surface-hover">
                <Td className="text-fg-muted">{formatDateTime(log.sentAt, i18n.language)}</Td>
                <Td>{log.functionName || log.functionId}</Td>
                <Td className="text-fg-muted">{log.tableSlug}</Td>
                <Td className="text-fg-muted">
                  {[log.actionType, log.method].filter(Boolean).join(" · ")}
                </Td>
                <Td className="text-fg-muted">{log.duration ? `${log.duration} ms` : ""}</Td>
                <Td>
                  <span
                    className={`text-2xs ${
                      /fail|error/i.test(log.status) ? "text-danger" : "text-fg-muted"
                    }`}
                  >
                    {log.status}
                  </span>
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Pager page={page} total={count} limit={FUNCTION_LOGS_PAGE} onPage={setPage} />
    </div>
  );
}
