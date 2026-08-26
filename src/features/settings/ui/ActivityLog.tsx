import { useDeferredValue, useState } from "react";
import { IconFileSpreadsheet } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, useTables } from "@/features/table";
import { useDataLanguages } from "@/features/workspace";
import { Button } from "@/shared/ui/button";
import { DatePicker } from "@/shared/ui/date-picker";
import { Dropdown } from "@/shared/ui/dropdown";
import { Icon } from "@/shared/ui/icon";
import { Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import { Tabs } from "@/shared/ui/tabs";
import {
  ACTIVITY_PAGE,
  NO_FILTERS,
  useActivity,
  useActivityEntry,
  useExportActivity,
  type ActivityFilters,
} from "../api/activity";
import { unwrapEntry } from "../model/activity";
import { FunctionLogs } from "./FunctionLogs";
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
 *
 * Рядом — вкладка выполнения функций. Предмет у неё другой (как
 * отработал вызов, а не что поменялось в проекте), и общих колонок
 * с этим списком нет ни одной, кроме даты, — поэтому вкладка,
 * а не строки вперемешку. См. FunctionLogs.
 */
export function ActivityLog() {
  const { t } = useTranslation();
  const [tab, setTab] = useState("changes");

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="shrink-0 border-b border-border px-4 py-2">
        <Tabs
          tabs={[
            { id: "changes", label: t("activity.tabChanges") },
            { id: "functions", label: t("activity.tabFunctions") },
          ]}
          activeId={tab}
          onSelect={setTab}
        />
      </div>

      {tab === "changes" ? <ChangesLog /> : <FunctionLogs />}
    </div>
  );
}

function ChangesLog() {
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
  const exportExcel = useExportActivity();

  const put = (patch: Partial<ActivityFilters>) => {
    setFilters((current) => ({ ...current, ...patch }));
    setPage(1);
  };

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("activity.title")} hint={t("activity.hint")}>
        {/* Выгружается отобранное, а не страница: файл делают, чтобы
            посмотреть шире экрана. */}
        <Button
          size="sm"
          variant="secondary"
          disabled={exportExcel.isPending}
          onClick={() => exportExcel.mutate(deferred)}
        >
          <Icon as={IconFileSpreadsheet} size={14} />
          {exportExcel.isPending ? t("common.loading") : t("activity.export")}
        </Button>
      </SectionHeader>

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
          <TableFilter value={filters.table} onChange={(table) => put({ table })} />
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

        {/* Наш календарь, а не нативное поле: у `<input type="date">`
            свой вид в каждой системе и светлый календарь в тёмной теме. */}
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
 * Отбор по таблице — выбором из списка, а не набором подстроки: слаг
 * таблицы (`orders`, `user_addresses`) человек по памяти не наберёт,
 * а с опечаткой журнал молча покажет пусто.
 *
 * Список тот же, что у связей и у пункта меню, — `useTables`: та же
 * ручка, тот же поиск на сервере и та же догрузка по страницам.
 * Сотни таблиц в проекте — норма, поэтому целиком он не тянется.
 *
 * В отбор уходит СЛАГ: сервер сравнивает его по подстроке и со слагом,
 * и с подписью (`version_history.go:158`), но подпись переводится
 * и повторяется у разных таблиц, а слаг один.
 */
function TableFilter({ value, onChange }: { value: string; onChange: (slug: string) => void }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const { current: language } = useDataLanguages();
  const tables = useTables(search);

  const found = tables.items.map((table) => ({
    value: table.slug,
    label: localized(table.labels, language, table.label),
  }));

  const items = [
    // Первым пунктом — снять отбор: стереть выбор в списке больше нечем.
    ...(value ? [{ value: "", label: t("table.clearFilters") }] : []),
    /*
     * Выбранная таблица остаётся в списке, даже когда поиск её не нашёл:
     * иначе набранное в поиске чужое слово стирает подпись с кнопки,
     * и кажется, что отбора нет, — а он есть. Слага хватает: он же
     * и лежит в отборе.
     */
    ...(value && !found.some((item) => item.value === value)
      ? [{ value, label: value }]
      : []),
    ...found,
  ];

  return (
    <Dropdown
      value={value}
      items={items}
      placeholder={t("activity.table")}
      ariaLabel={t("activity.table")}
      searchPlaceholder={t("menuForm.tableSearch")}
      emptyText={t("menuForm.tableEmpty")}
      search={search}
      loading={tables.isLoading}
      hasMore={tables.hasMore}
      onSearch={setSearch}
      onLoadMore={tables.loadMore}
      onChange={onChange}
      size="sm"
    />
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
