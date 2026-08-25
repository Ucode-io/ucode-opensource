import { useMemo, useState, type ReactNode } from "react";
import {
  IconChevronRight,
  IconPlus,
  IconSortAscending,
  IconSortDescending,
  IconX,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, type Field } from "@/features/table";
import { Checkbox } from "@/shared/ui/checkbox";
import { Dropdown } from "@/shared/ui/dropdown";
import { Icon } from "@/shared/ui/icon";
import {
  AGGREGATIONS,
  NUMERIC_FIELDS,
  isHidden,
  pathKey,
  pivotTable,
  sameSortTarget,
  type Aggregation,
  type PivotSetup,
  type PivotSort,
} from "../model/pivot";
import type { Item } from "../model/types";

/**
 * Сводная таблица: строки собраны в клетки «значение × значение».
 *
 * Считается по ЗАГРУЖЕННЫМ строкам — серверной сводки нет (см.
 * model/pivot). Поэтому под настройками стоит счётчик: сколько строк
 * вошло в расчёт. Число, которое выглядит итогом по всей таблице и им
 * не является, — это то, ради чего в v2 отказались от `view.summaries`.
 *
 * Настройки живут в адресе: это не то, что админ настроил всем, а то,
 * как человек сейчас смотрит, — как сортировка и фильтры.
 */
export function PivotView({
  columns,
  rows,
  setup,
  language,
  loaded,
  total,
  onSetup,
}: {
  /** Колонки view: из них выбирают поля сводной. */
  columns: Field[];
  rows: Item[];
  setup: PivotSetup;
  language: string;
  /** Приехали не все строки диапазона — счётчик тогда врать не должен. */
  loaded: boolean;
  /**
   * Сколько строк под этим отбором ВСЕГО, по данным сервера.
   *
   * Без него счётчик говорит «посчитано по 200 загруженным» и умалчивает
   * главное: загружено 200 из двенадцати тысяч. Число знает сервер —
   * оно приходит в каждом ответе get-list.
   */
  total: number;
  onSetup: (next: Partial<PivotSetup>) => void;
}) {
  const { t, i18n } = useTranslation();
  const table = useMemo(() => pivotTable(rows, setup), [rows, setup]);
  /**
   * Свёрнутые группы — состояние экрана, а не настройка: в адрес
   * не уезжают. Их сбрасывает уже смена полей строк, потому что
   * путь свёрнутой группы после неё ничему не соответствует.
   */
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set());
  /*
   * Настройки свёрнуты, когда сводная уже настроена: их задают один раз,
   * а место они занимают всегда — на сводной оно нужно данным. У пустой
   * сводной открыты: сворачивать там нечего, надо выбрать поле.
   */
  const [settingsOpen, setSettingsOpen] = useState(() => !setup.rowSlugs.length);

  const label = (field: Field) => localized(field.labels, language, field.label);
  const bySlug = useMemo(() => new Map(columns.map((field) => [field.slug, field])), [columns]);
  /** Считать можно по числовому полю; количество — по любому. */
  const numeric = columns.filter((field) => NUMERIC_FIELDS.has(field.type));

  const options = (fields: Field[], empty: string) => [
    { value: "", label: empty },
    ...fields.map((field) => ({ value: field.slug, label: label(field) })),
  ];

  const setRows = (next: string[]) => {
    setCollapsed(new Set());
    onSetup({ rowSlugs: next.filter(Boolean) });
  };

  /*
   * Пустая сводная всё равно показывает один список — тот, который надо
   * заполнить. Без него в зоне строк не было бы ничего, кроме кнопки,
   * и первый шаг был бы неочевиден.
   */
  const rowSlugs = setup.rowSlugs.length ? setup.rowSlugs : [""];

  /**
   * Щелчок по заголовку: первый раз — по возрастанию, второй — по
   * убыванию. Третьего состояния нет: «как пришло» для сводной
   * не значит ничего, строки в ней собраны заново.
   */
  const sortBy = (by: PivotSort["by"]) =>
    onSetup({
      sort: { by, desc: sameSortTarget(setup.sort.by, by) ? !setup.sort.desc : false },
    });

  /**
   * Что настроено, одной строкой — её показывает свёрнутая панель.
   * Без неё на экране остались бы числа без объяснения, откуда они.
   */
  const colField = setup.colSlug ? bySlug.get(setup.colSlug) : undefined;
  const summary = [
    rowSlugs
      .map((slug) => bySlug.get(slug))
      .map((field) => (field ? label(field) : ""))
      .filter(Boolean)
      .join(" · ") || t("pivot.pick"),
    colField ? label(colField) : t("pivot.none"),
    t(`pivot.aggregation.${setup.aggregation}` as const),
  ].join(" × ");

  const number = (value: number | undefined) => show(value, i18n.language);
  const visible = table.rows.filter((row) => !isHidden(row.path, collapsed));
  const deepest = rowSlugs.filter(Boolean).length - 1;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Настройки на своей плашке: у сводной их вчетверо больше, чем
          у таблицы, и без фона они сливались с первой строкой данных. */}
      <div className="shrink-0 border-b border-border bg-surface-hover px-3 py-2">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <button
            type="button"
            aria-expanded={settingsOpen}
            onClick={() => setSettingsOpen((open) => !open)}
            className="flex min-w-0 items-center gap-1.5 rounded-md text-left text-xs text-fg-muted transition-colors hover:text-fg"
          >
            <Icon
              as={IconChevronRight}
              size={12}
              className={`shrink-0 transition-transform ${settingsOpen ? "rotate-90" : ""}`}
            />
            <span className="shrink-0 font-medium">{t("pivot.settings")}</span>

            {/* Свёрнутая панель обязана говорить, что настроено: иначе
                на экране остаются числа без объяснения, откуда они. */}
            {!settingsOpen && <span className="truncate text-fg-subtle">· {summary}</span>}
          </button>

          {/* Счётчик виден всегда, в том числе свёрнутым: это итог
              по загруженному, а не по таблице, и прятать его нельзя. */}
          <span className="ml-auto shrink-0 text-xs text-fg-subtle">
            {loaded
              ? t("pivot.countedAll", { count: table.count })
              : t("pivot.counted", {
                  count: table.count,
                  total: total.toLocaleString(i18n.language),
                })}
          </span>
        </div>

        {settingsOpen && (
          <>
            <div className="mt-2 grid gap-2 md:grid-cols-3">
              <Zone title={t("pivot.rows")} hint={t("pivot.rowsHint")}>
                {/* Полей строк несколько: каждое следующее — вложенный
                    уровень. Так же было в старой админке, где в Row Groups
                    уезжал весь список group_by_columns. */}
                {rowSlugs.map((slug, index) => (
                  <div key={`${slug}:${index}`} className="flex items-center gap-1">
                    <Dropdown
                      value={slug}
                      placeholder={t("pivot.pick")}
                      className="min-w-0 flex-1"
                      items={options(columns, t("pivot.pick"))}
                      onChange={(next) =>
                        setRows(rowSlugs.map((item, at) => (at === index ? next : item)))
                      }
                    />

                    {/* Первый уровень не убираем: без него сводить нечего. */}
                    {rowSlugs.length > 1 && (
                      <button
                        type="button"
                        aria-label={t("pivot.removeLevel")}
                        title={t("pivot.removeLevel")}
                        onClick={() => setRows(rowSlugs.filter((_, at) => at !== index))}
                        className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-danger"
                      >
                        <Icon as={IconX} size={14} />
                      </button>
                    )}
                  </div>
                ))}

                {/* Уровень добавляют, только когда есть что вкладывать:
                    уровень без поля — пустая строка в настройках. */}
                {rowSlugs.every(Boolean) && rowSlugs.length < columns.length && (
                  <button
                    type="button"
                    onClick={() => setRows([...rowSlugs, firstUnused(columns, rowSlugs)])}
                    className="flex h-8 items-center justify-center gap-1 rounded-md border border-dashed border-border-strong text-xs text-fg-muted transition-colors hover:border-fg-subtle hover:text-fg"
                  >
                    <Icon as={IconPlus} size={12} />
                    {t("pivot.addLevel")}
                  </button>
                )}
              </Zone>

              <Zone title={t("pivot.columns")} hint={t("pivot.columnsHint")}>
                <Dropdown
                  value={setup.colSlug}
                  placeholder={t("pivot.none")}
                  items={options(columns, t("pivot.none"))}
                  onChange={(colSlug) => onSetup({ colSlug })}
                />
              </Zone>

              <Zone title={t("pivot.value")} hint={t("pivot.valueHint")}>
                <Dropdown
                  value={setup.aggregation}
                  onChange={(aggregation) => onSetup({ aggregation: aggregation as Aggregation })}
                  items={AGGREGATIONS.map((item) => ({
                    value: item,
                    label: t(`pivot.aggregation.${item}` as const),
                  }))}
                />

                {/* Поле значения нужно всем, кроме количества: считать
                    «сколько строк» можно и без него. */}
                {setup.aggregation !== "count" && (
                  <Dropdown
                    value={setup.valueSlug}
                    placeholder={t("pivot.pick")}
                    items={options(numeric, t("pivot.pick"))}
                    onChange={(valueSlug) => onSetup({ valueSlug })}
                  />
                )}
              </Zone>
            </div>

            <label className="mt-2 flex w-fit items-center gap-1.5 text-xs text-fg-muted">
              <Checkbox
                checked={setup.skipEmpty}
                onChange={(event) => onSetup({ skipEmpty: event.target.checked })}
              />
              {t("pivot.skipEmpty")}
            </label>
          </>
        )}
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        {!setup.rowSlugs.some(Boolean) ? (
          <p className="p-6 text-sm text-fg-muted">{t("pivot.setup")}</p>
        ) : !table.rows.length ? (
          <p className="p-6 text-sm text-fg-muted">{t("pivot.empty")}</p>
        ) : (
          <table className="min-w-max border-collapse text-sm">
            <thead className="sticky top-0 z-10 bg-surface">
              <tr>
                <Header sticky align="left" sort={setup.sort} by={{ kind: "label" }} onSort={sortBy}>
                  {rowSlugs
                    .map((slug) => bySlug.get(slug))
                    .map((field) => (field ? label(field) : ""))
                    .filter(Boolean)
                    .join(" · ")}
                </Header>

                {table.columns.map((column) => (
                  <Header
                    key={column}
                    sort={setup.sort}
                    by={{ kind: "column", key: column }}
                    onSort={sortBy}
                  >
                    {column || t("pivot.blank")}
                  </Header>
                ))}

                <Header sort={setup.sort} by={{ kind: "total" }} onSort={sortBy}>
                  {t("pivot.total")}
                </Header>
              </tr>
            </thead>

            <tbody>
              {visible.map((row) => {
                const key = pathKey(row.path);
                const foldable = row.hasChildren && row.level < deepest;
                const group = row.level < deepest;

                return (
                  <tr key={key} className="hover:bg-surface-hover">
                    <th
                      className="sticky left-0 border-r border-b border-border bg-surface py-1.5 pr-3 text-left font-normal"
                      /* Отступ уровнем, а не вложенной разметкой: клетка
                         в таблице одна, а уровней бывает сколько угодно. */
                      style={{ paddingLeft: `${0.75 + row.level * 1.5}rem` }}
                    >
                      <span className="flex items-center gap-1.5">
                        {/* Направляющая: на третьем уровне одного отступа
                            мало, чтобы увидеть, чей это потомок. */}
                        {row.level > 0 && (
                          <span className="-ml-3 h-4 w-px shrink-0 bg-border" aria-hidden />
                        )}

                        {foldable ? (
                          <button
                            type="button"
                            aria-expanded={!collapsed.has(key)}
                            onClick={() =>
                              setCollapsed((prev) => {
                                const next = new Set(prev);
                                if (!next.delete(key)) next.add(key);
                                return next;
                              })
                            }
                            className="grid size-4 shrink-0 place-items-center rounded text-fg-muted transition-colors hover:bg-surface-active hover:text-fg"
                          >
                            {/* Вправо — свёрнуто, вниз — развёрнуто. */}
                            <Icon
                              as={IconChevronRight}
                              size={12}
                              className={`transition-transform ${collapsed.has(key) ? "" : "rotate-90"}`}
                            />
                          </button>
                        ) : (
                          /* Место под стрелку занято и у листа: иначе
                             подписи соседних строк разъезжались бы. */
                          <span className="size-4 shrink-0" />
                        )}

                        <span className={group ? "font-medium" : ""}>
                          {row.key || t("pivot.blank")}
                        </span>
                      </span>
                    </th>

                    {table.columns.map((column) => (
                      <td
                        key={column}
                        className="border-r border-b border-border px-3 py-1.5 text-right tabular-nums"
                      >
                        {number(row.cells.get(column))}
                      </td>
                    ))}

                    <td className="border-b border-border px-3 py-1.5 text-right font-medium tabular-nums">
                      {number(row.total)}
                    </td>
                  </tr>
                );
              })}

              {/* Итог отделён двойной линией и фоном: это не ещё одна
                  строка данных, а черта под ними. */}
              <tr className="bg-surface-active font-semibold">
                <th className="sticky left-0 border-t-2 border-r border-border-strong bg-surface-active px-3 py-1.5 text-left">
                  {t("pivot.total")}
                </th>
                {table.columns.map((column) => (
                  <td
                    key={column}
                    className="border-t-2 border-r border-border-strong px-3 py-1.5 text-right tabular-nums"
                  >
                    {number(table.columnTotals.get(column))}
                  </td>
                ))}
                <td className="border-t-2 border-border-strong px-3 py-1.5 text-right tabular-nums">
                  {number(table.total)}
                </td>
              </tr>
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

/** Зона настроек: подпись, пояснение и содержимое столбиком. */
function Zone({ title, hint, children }: { title: string; hint: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5 rounded-lg border border-border bg-surface p-2">
      <span className="flex items-baseline gap-1.5">
        <span className="text-xs font-medium text-fg">{title}</span>
        <span className="truncate text-2xs text-fg-subtle">{hint}</span>
      </span>
      {children}
    </div>
  );
}

/**
 * Заголовок колонки. Он же кнопка сортировки: в старой админке сортировку
 * по сводной давал ag-grid тем же щелчком по шапке.
 */
function Header({
  children,
  sort,
  by,
  onSort,
  align = "right",
  sticky = false,
}: {
  children: ReactNode;
  sort: PivotSort;
  by: PivotSort["by"];
  onSort: (by: PivotSort["by"]) => void;
  align?: "left" | "right";
  sticky?: boolean;
}) {
  const active = sameSortTarget(sort.by, by);

  return (
    <th
      className={`border-b border-border bg-surface px-3 py-1.5 font-medium text-fg ${
        align === "left" ? "text-left" : "text-right"
      } ${sticky ? "sticky left-0 z-10 border-r" : "border-r last:border-r-0"}`}
    >
      <button
        type="button"
        onClick={() => onSort(by)}
        className={`flex w-full items-center gap-1 rounded transition-colors hover:text-accent-text ${
          align === "left" ? "justify-start" : "justify-end"
        }`}
      >
        <span className="truncate">{children}</span>
        {active && (
          <Icon
            as={sort.desc ? IconSortDescending : IconSortAscending}
            size={12}
            className="shrink-0 text-accent-text"
          />
        )}
      </button>
    </th>
  );
}

/** Первое поле, которого ещё нет в строках: новый уровень не должен повторять. */
function firstUnused(columns: Field[], taken: string[]): string {
  return columns.find((field) => !taken.includes(field.slug))?.slug ?? "";
}

/**
 * Число в клетке — с разделителем разрядов и по правилам языка
 * интерфейса: `1339` и `1 339` читаются с разной скоростью, а в сводной
 * числа именно сравнивают глазами. Дробные — до двух знаков: больше
 * в своде не значит точнее.
 *
 * Пустая клетка — прочерк, а не ноль: в этой паре значений строк
 * не было вовсе, и ноль соврал бы о том, чего не считали.
 */
function show(value: number | undefined, locale: string): string {
  if (value === undefined) return "—";

  return value.toLocaleString(locale, { maximumFractionDigits: 2 });
}
