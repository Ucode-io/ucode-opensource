import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useTablePermission } from "@/features/auth";
import {
  DataGrid,
  FilterBar,
  GridFooter,
  GridSkeleton,
  ItemDrawer,
  TableToolbar,
  activeFilterCount,
  fromConditions,
  nextSorts,
  orderColumns,
  seedFilters,
  toConditions,
  useCreateItem,
  useDrawerLayout,
  useItems,
  useUpdateItem,
  type Filters,
  type Item,
  type Sort,
} from "@/features/item";
import { useTableSchema, type Field, type Relation } from "@/features/table";
import type { DataLanguage } from "@/features/workspace";
import { toast } from "@/shared/lib/toast";
import { pinnedIds, resolveColumnIds } from "../model/columns";
import type { RelationTab } from "../model/relation-tabs";
import { useExportExcel } from "../api/excel";
import { ExcelImportDialog } from "./ExcelImportDialog";
import { ViewOptions } from "./ViewOptions";

/**
 * Правки настроек вкладки. Все они уезжают в раскладку карточки одним
 * и тем же PUT: у вкладки нет своей строки в базе, кроме `tab`.
 */
export type TabSettings = {
  /** Имя на конкретном языке ДАННЫХ. */
  onRename: (name: string, language: string) => void;
  onColumns: (columnIds: string[]) => void;
  onFixedColumns: (columnIds: string[]) => void;
  /** Поля целиком: в attributes лежат они, а не идентификаторы. */
  onQuickFilters: (fields: Field[]) => void;
  /** Область видимости вкладки: карта «слаг → условие», как в get-list. */
  onDefaultFilters: (conditions: Record<string, unknown>) => void;
  /** Убрать вкладку из карточки. */
  onRemove: () => void;
};

/**
 * Вкладка связи в карточке записи: строки ЧУЖОЙ таблицы, относящиеся
 * к открытой записи.
 *
 * Отбор — по колонке-ссылке из раскладки (`relation.relation_field_slug`),
 * а не по собранному имени `<слаг родительской таблицы>_id`. Старая
 * админка собирала имя сама, и у второй связи на ту же таблицу — где
 * колонка называется иначе — вкладка показывала не то.
 *
 * Живёт в features/view, а не в features/item: колонки вкладки — это
 * список id полей, и разворачивает его resolveColumnIds. Обратный импорт
 * (item → view) замкнул бы фичи в кольцо — ViewOptions уже импортирует
 * features/item.
 *
 * Сортировка, страница, поиск и отбор — своё состояние, а не адрес:
 * адрес занят основной таблицей, и второй набор тех же параметров
 * в нём означал бы либо префиксы у каждого ключа, либо путаницу «чья
 * это страница». Ссылка открывает вкладку с начала — это и есть та цена.
 */
export function RelationView({
  tab,
  parentGuid,
  parentValue,
  menuId,
  locale,
  language,
  languages = EMPTY_LANGUAGES,
  onLanguage,
  settings,
  saving = false,
}: {
  tab: RelationTab;
  /** guid открытой записи. По нему отбираются связанные строки. */
  parentGuid: string;
  /**
   * Значение колонки-ссылки в НАШЕЙ строке. Нужно вкладке обратного
   * направления: там связанная строка одна, и найти её можно только
   * по guid, который лежит у нас.
   */
  parentValue?: string | undefined;
  /**
   * Пункт меню. Нужен раскладке связанной строки: карточка внутри
   * вкладки показывает поля в том же порядке, что и своя.
   */
  menuId: string;
  /** Локаль интерфейса: форматы дат и чисел. */
  locale: string;
  /** Язык данных: подписи полей и вариантов. */
  language: string;
  /** Языки данных проекта: по ним сводятся колонки мультиязычного поля. */
  languages?: DataLanguage[];
  /**
   * Сменить язык ДАННЫХ. Нужен карточке связанной строки: у чужой
   * таблицы мультиязычные поля свои, и переключать их приходится там же.
   */
  onLanguage?: ((code: string) => void) | undefined;
  /**
   * Правки настроек вкладки. Не заданы — панели «⋯» нет вовсе: настройки
   * лежат в раскладке карточки, и правит их тот, у кого есть права
   * на неё. У вложенной вкладки их нет намеренно — см. RelatedRow.
   */
  settings?: TabSettings | undefined;
  /** Идёт запись раскладки: панель настроек показывает это спиннером. */
  saving?: boolean;
}) {
  const { t } = useTranslation();

  /*
   * Настройки связей запрашиваются по показанным колонкам. У вкладки
   * без своих колонок показаны ВСЕ поля, поэтому и список ограничивать
   * нечем: иначе колонка-связь во вкладке осталась бы пустой — её
   * view_fields никто бы не загрузил.
   */
  const { schema, isLoading: schemaLoading } = useTableSchema(
    tab.tableSlug,
    tab.columnIds.length ? tab.columnIds : undefined,
  );
  /*
   * Колонки вкладки. Пустой список — обычное дело: бэкенд заводит вкладку
   * вместе со связью и колонок в неё не кладёт. Пустая вкладка ничего
   * не сообщает, поэтому по умолчанию показываются все поля чужой
   * таблицы, а сузить их можно настройкой вкладки.
   */
  const columns = useMemo(
    () =>
      tab.columnIds.length ? resolveColumnIds(tab.columnIds, schema.fields) : schema.fields,
    [tab.columnIds, schema.fields],
  );

  const [sorts, setSorts] = useState<Sort[]>([]);
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(LIMIT);
  const [search, setSearch] = useState("");
  /*
   * Отбор человека. null — его ещё не трогали, и в подшапке стоят чипы,
   * предложенные в настройках вкладки. Пустой объект — трогали и сняли
   * всё: подсказка тогда не возвращается, иначе снятый чип приходил бы
   * обратно сам.
   *
   * Поля подсказки ищутся в схеме, а она приезжает не сразу, — поэтому
   * подсказка вычисляется, а не кладётся в начальное состояние.
   */
  const [own, setOwn] = useState<Filters | null>(null);
  const [filtersOpen, setFiltersOpen] = useState(false);
  /** Раскрытая связанная строка. Пусто — открыт список. */
  const [openGuid, setOpenGuid] = useState<string | null>(null);

  /*
   * `contains` — не поиск подстроки, а форма записи: в теле get-list это
   * голое значение рядом со слагом (`{author_id: "<guid>"}`), и ровно так
   * его шлёт старая админка. Отдельной операции «равно строке» в наших
   * фильтрах нет: `equals` занят булевыми.
   *
   * Отбор человека домешивается сверху, но связь с открытой записью
   * перекрыть нельзя: вкладка без неё показала бы чужую таблицу целиком.
   *
   * Куда смотрит связь, решает direction:
   *   incoming — чужая колонка со ссылкой на нас;
   *   outgoing — наша колонка со ссылкой на чужую строку, и тогда
   *              отбираем чужую таблицу по её же guid.
   */
  const link = tab.direction === "incoming" ? tab.fieldSlug : "guid";
  const value = tab.direction === "incoming" ? parentGuid : (parentValue ?? "");

  /*
   * Отбор по умолчанию — это ОБЛАСТЬ ВИДИМОСТИ вкладки, а не начальное
   * значение фильтра: он домешивается сверху и перекрывает условие
   * человека по тому же полю. Так же он работает у таблицы.
   */
  const scope = useMemo(() => fromConditions(tab.view.defaultFilters), [tab.view.defaultFilters]);

  const seeded = useMemo(
    () => seedFilters(tab.view.quickFilterIds, schema.fields),
    [tab.view.quickFilterIds, schema.fields],
  );
  const chips = own ?? seeded;

  const filters = useMemo(
    () => ({ ...chips, ...scope, [link]: { op: "contains" as const, values: [value] } }),
    [chips, scope, link, value],
  );

  /** Закреплённые колонки вкладки — в тех же ключах, что и у таблицы. */
  const pinned = useMemo(
    () => pinnedIds(tab.view.fixedColumnIds, columns),
    [tab.view.fixedColumnIds, columns],
  );

  const { page: rows, isLoading } = useItems(value ? tab.tableSlug : undefined, {
    limit,
    page,
    sorts,
    filters,
    search,
  });

  const update = useUpdateItem(tab.tableSlug);
  const create = useCreateItem(tab.tableSlug);
  const exportExcel = useExportExcel(tab.tableSlug);
  const [importing, setImporting] = useState(false);
  const can = useTablePermission(tab.tableSlug);

  /*
   * Выделение вкладке не нужно: массовых действий над связанными
   * строками здесь нет, а DataGrid без него не собирается. Пустое
   * и неизменное — колонка с флажками остаётся, но всегда пуста.
   */
  const [selected, setSelected] = useState<Set<string>>(() => new Set());

  /*
   * У обратной связи ссылка лежит в нашей строке, и пока её не задали,
   * показывать нечего: отбор по пустому guid вернул бы чужую таблицу
   * целиком.
   */
  if (!value) return <p className="p-6 text-sm text-fg-muted">{t("drawer.noRelated")}</p>;
  if (schemaLoading || isLoading) return <GridSkeleton columns={columns.length || 4} />;
  if (!columns.length) return <p className="p-6 text-sm text-fg-muted">{t("table.noColumns")}</p>;

  const openRow = rows.rows.find((item) => item.guid === openGuid);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Своя панель инструментов: у вкладки свой список, и искать
          в нём приходится ровно так же, как в основной таблице. */}
      <div className="flex h-9 shrink-0 items-center justify-end gap-0.5 border-b border-border px-2">
        <TableToolbar
          tableSlug={tab.tableSlug}
          columns={columns}
          language={language}
          sorts={sorts}
          onSorts={(next) => {
            setSorts(next);
            setPage(1);
          }}
          filtersOpen={filtersOpen}
          filterCount={activeFilterCount(chips)}
          onToggleFilters={() => setFiltersOpen((value) => !value)}
          search={search}
          onSearch={(next) => {
            setSearch(next);
            setPage(1);
          }}
        />

        {settings && (
          <ViewOptions
            view={tab.view}
            // ВСЕ поля чужой таблицы: скрытую колонку иначе не вернуть.
            fields={schema.fields}
            language={language}
            languages={languages}
            defaultFilters={scope}
            // Права роли на ЧУЖУЮ таблицу: настройки вкладки — про неё,
            // а не про таблицу, из которой открыли карточку.
            can={can}
            exporting={exportExcel.isPending}
            busy={saving}
            labels={{ title: "drawer.tabSettings", delete: "drawer.removeTab" }}
            handlers={{
              onRename: (name, nameLanguage) => settings.onRename(name, nameLanguage),
              onColumns: settings.onColumns,
              onQuickFilters: settings.onQuickFilters,
              onFixedColumns: settings.onFixedColumns,
              onDefaultFilters: (next) => settings.onDefaultFilters(toConditions(next)),
              onImport: () => setImporting(true),
              // Выгружается то, что видно: колонки вкладки, её отбор
              // и связь с открытой записью — без последней файл содержал
              // бы чужие строки.
              onExport: () =>
                exportExcel.mutate({
                  fieldIds: columns.map((field) => field.id),
                  filters,
                  search,
                }),
              onDelete: settings.onRemove,
            }}
          />
        )}
      </div>

      {filtersOpen && (
        <FilterBar
          columns={columns}
          language={language}
          filters={chips}
          sorts={sorts}
          onFilters={(next) => {
            setOwn(next);
            setPage(1);
          }}
          onSorts={(next) => {
            setSorts(next);
            setPage(1);
          }}
        />
      )}

      <DataGrid
        tableSlug={tab.tableSlug}
        columns={columns}
        rows={rows.rows}
        relations={schema.relations}
        locale={locale}
        language={language}
        pinned={pinned}
        selected={selected}
        onSelect={setSelected}
        sorts={sorts}
        onSort={(field, direction) => {
          setSorts(direction ? [{ field, direction }] : nextSorts(sorts, field));
          setPage(1);
        }}
        onEdit={(guid, slug, value) => update.mutate({ guid, slug, value })}
        // Связанная строка раскрывается на месте, поверх вкладки:
        // у чужой таблицы своего экрана в этом меню нет, а посмотреть
        // на неё целиком нужно чаще, чем перейти в её таблицу.
        onOpenRow={setOpenGuid}
        /*
         * Ссылка на открытую запись проставляется сама: связанную строку
         * заводят ИЗ карточки, и заполнять её вручную значит предложить
         * человеку выбрать ту самую запись, из которой он смотрит.
         */
        {...(tab.canCreate
          ? {
              creating: create.isPending,
              onCreate: (values: Record<string, unknown>, done: () => void) =>
                create.mutate(
                  { ...values, [tab.fieldSlug]: parentGuid },
                  {
                    onSuccess: () => {
                      done();
                      toast.success(t("table.rowCreated"));
                    },
                  },
                ),
            }
          : {})}
      />

      <GridFooter
        page={page}
        limit={limit}
        total={rows.count}
        selectedCount={0}
        deleting={false}
        onPage={setPage}
        onLimit={(next) => {
          setLimit(next);
          setPage(1);
        }}
        onDeleteSelected={() => {}}
      />

      {importing && (
        <ExcelImportDialog
          tableSlug={tab.tableSlug}
          // Все поля чужой таблицы: столбец файла можно положить
          // и в колонку, которой во вкладке не видно.
          fields={schema.fields}
          language={language}
          onClose={() => setImporting(false)}
        />
      )}

      {/* Карточка связанной строки — со своей раскладкой и своими
          вкладками, пока хватает глубины. */}
      {openRow && (
        <RelatedRow
          key={openGuid}
          tableSlug={tab.tableSlug}
          row={openRow}
          fields={schema.fields}
          relations={schema.relations}
          menuId={menuId}
          locale={locale}
          language={language}
          languages={languages}
          {...(onLanguage ? { onLanguage } : {})}
          onEdit={(guid, slug, value) => update.mutate({ guid, slug, value })}
          onClose={() => setOpenGuid(null)}
        />
      )}
    </div>
  );
}

/**
 * Карточка связанной строки, раскрытая поверх вкладки.
 *
 * Раскладка у чужой таблицы своя, и берётся она тем же запросом, что
 * и раскладка открытой записи: ручка отдаёт раскладку пункта меню,
 * а если её нет — общую раскладку таблицы (layout.go, GetSingleLayout).
 * Поэтому связанная строка показывается в том же порядке полей, что
 * и в своей таблице, с тем же заголовком и теми же секциями.
 *
 * Запрос уходит только когда строку раскрыли: хук живёт в отдельном
 * компоненте, а не во вкладке, и до первого щелчка его нет вовсе.
 *
 * Ничего не правит: у чужой таблицы это ОБЩАЯ раскладка, и PUT привязал
 * бы её к нашему пункту меню (`menu_id` пишется безусловно) — с этого
 * момента все остальные меню видели бы её изменения. Порядок полей,
 * заголовок и набор вкладок здесь только читаются.
 */
function RelatedRow({
  tableSlug,
  row,
  fields,
  relations,
  menuId,
  locale,
  language,
  languages,
  onLanguage,
  onEdit,
  onClose,
}: {
  tableSlug: string;
  row: Item;
  /** Все поля чужой таблицы: карточка показывает запись целиком. */
  fields: Field[];
  relations: Relation[];
  menuId: string;
  locale: string;
  language: string;
  languages: DataLanguage[];
  onLanguage?: ((code: string) => void) | undefined;
  onEdit: (guid: string, slug: string, value: unknown) => void;
  onClose: () => void;
}) {
  const layout = useDrawerLayout({ tableSlug, menuId, language });

  const columns = useMemo(
    () => orderColumns(fields, layout.order).filter((field) => !layout.hidden.has(field.slug)),
    [fields, layout.order, layout.hidden],
  );

  return (
    <ItemDrawer
      tableSlug={tableSlug}
      columns={columns}
      row={row}
      relations={relations}
      locale={locale}
      language={language}
      languages={languages}
      {...(onLanguage ? { onLanguage } : {})}
      sections={layout.sections}
      heading={layout.heading}
      onEdit={onEdit}
      onClose={onClose}
    />
  );
}

/** Строк на странице вкладки. Меньше, чем у таблицы: места под неё меньше. */
const LIMIT = 10;

/** Постоянная ссылка: пустой литерал по умолчанию пересобирал бы поля. */
const EMPTY_LANGUAGES: DataLanguage[] = [];
