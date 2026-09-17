import { useDeferredValue, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Chip } from "@/shared/ui/chip";
import { DatePicker } from "@/shared/ui/date-picker";
import { Dropdown } from "@/shared/ui/dropdown";
import {
  FUNCTION_LOGS_PAGE,
  NO_LOG_FILTERS,
  useFunctionLogs,
  type FunctionLogFilters,
} from "../api/function-logs";
import { useProjectFunctions } from "../api/functions";
import { relativeTime } from "../model/time";
import { Empty, MethodBadge, Pager, SectionHeader, Td, Th, formatDateTime } from "./parts";
import { TableFilter } from "./TableFilter";

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
  const [limit, setLimit] = useState(FUNCTION_LOGS_PAGE);

  const deferred = useDeferredValue(filters);
  const { logs, count, isLoading, error } = useFunctionLogs(deferred, page, limit);

  const put = (patch: Partial<FunctionLogFilters>) => {
    setFilters((current) => ({ ...current, ...patch }));
    setPage(1);
  };

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("functionLogs.title")} hint={t("functionLogs.hint")} />

      <div className="flex shrink-0 flex-wrap items-end gap-2 border-b border-border px-4 py-2">
        <div className="w-44">
          <FunctionFilter
            value={filters.functionId}
            onChange={(functionId) => put({ functionId })}
          />
        </div>

        {/*
          Статус — список из двух значений, и это не догадка: в журнал
          его пишут ровно два места, и оба знают только «success»
          и «error» (`helper/invoke_function.go:61,101` и
          `function_service/api/handlers/function.go:1241,1271`).
          Третьего не бывает.

          Список здесь обязателен, а не желателен: сравнение ТОЧНОЕ,
          `l.status = $1` (`version_history.go:396`), — в отличие
          от журнала изменений, где всё через ILIKE. Набранное руками
          «succ» молча вернуло бы пустой список.
        */}
        <div className="w-36">
          <Dropdown
            value={filters.status}
            items={[
              ...(filters.status ? [{ value: "", label: t("table.clearFilters") }] : []),
              { value: "success", label: t("functionLogs.success") },
              { value: "error", label: t("functionLogs.error") },
            ]}
            placeholder={t("activity.status")}
            ariaLabel={t("activity.status")}
            onChange={(status) => put({ status })}
            size="sm"
          />
        </div>

        {/* Таблица — тот же выбор из списка, что и в журнале изменений:
            здесь сравнение тоже точное (`l.table_slug = $1`), и слаг
            по памяти не набирают. */}
        <div className="w-40">
          <TableFilter value={filters.table} onChange={(table) => put({ table })} />
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
                <Td className="text-fg-muted">
                  {formatDateTime(log.sentAt, i18n.language)}
                  <span className="block text-2xs text-fg-subtle">
                    {relativeTime(log.sentAt, i18n.language)}
                  </span>
                </Td>
                <Td>{log.functionName || log.functionId}</Td>
                <Td className="text-fg-muted">{log.tableSlug}</Td>
                <Td>
                  <span className="flex items-center gap-1.5 text-fg-muted">
                    <MethodBadge method={log.method} />
                    {log.actionType}
                  </span>
                </Td>
                <Td className="text-fg-muted">{log.duration ? `${log.duration} ms` : ""}</Td>
                <Td>
                  {/*
                    Исход — плашка, и здесь цвет достаётся именно ей:
                    на этом экране главный вопрос не «что вызывали»,
                    а «чем кончилось», и ради него список открывают.
                    Значений ровно два, оба известны (см. отбор выше),
                    поэтому зелёная и красная — весь набор.

                    Слово из базы переводится, а не показывается как
                    есть: «success» в русском интерфейсе — не термин,
                    а недоделка. Незнакомое значение всё же покажем
                    как есть, серым: соврать хуже, чем удивить.
                  */}
                  {log.status === "success" && (
                    <Chip color="green">{t("functionLogs.success")}</Chip>
                  )}
                  {log.status === "error" && <Chip color="red">{t("functionLogs.error")}</Chip>}
                  {log.status !== "success" && log.status !== "error" && log.status && (
                    <Chip color="gray">{log.status}</Chip>
                  )}
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Pager
        page={page}
        total={count}
        limit={limit}
        onPage={setPage}
        onLimit={(next) => {
          setLimit(next);
          setPage(1);
        }}
      />
    </div>
  );
}

/**
 * Отбор по функции — списком, а не строкой поиска.
 *
 * Дело не только в удобстве: поиск по имени на этой ручке отвечает
 * ошибкой на весь журнал (условие ссылается на присоединённую таблицу
 * функций, а счётчик строк выполняется без неё — см. `api/function-logs`).
 * Отбор по идентификатору такого условия не создаёт.
 *
 * Список — первая страница `useProjectFunctions` с поиском на сервере:
 * функций в проекте единицы, а не сотни, как таблиц, и догрузка
 * по прокрутке здесь была бы механизмом ради двух десятков строк.
 * Не нашлось — набирают в поиске, он уходит на сервер.
 */
function FunctionFilter({ value, onChange }: { value: string; onChange: (id: string) => void }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const { functions, isLoading } = useProjectFunctions(search, 1);

  const found = functions.map((item) => ({
    value: item.id,
    label: item.name || item.path || item.id,
  }));

  const items = [
    // Первым пунктом — снять отбор: стереть выбор в списке больше нечем.
    ...(value ? [{ value: "", label: t("table.clearFilters") }] : []),
    /* Выбранная функция остаётся в списке, даже когда поиск её не нашёл,
       — иначе набранное чужое слово стирает подпись с кнопки, и кажется,
       что отбора нет, а он есть. */
    ...(value && !found.some((item) => item.value === value)
      ? [{ value, label: value }]
      : []),
    ...found,
  ];

  return (
    <Dropdown
      value={value}
      items={items}
      placeholder={t("functionLogs.function")}
      ariaLabel={t("functionLogs.function")}
      searchPlaceholder={t("functions.search")}
      emptyText={t("functions.empty")}
      search={search}
      loading={isLoading}
      onSearch={setSearch}
      onChange={onChange}
      size="sm"
    />
  );
}
