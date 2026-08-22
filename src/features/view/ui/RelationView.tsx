import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useTablePermission } from "@/features/auth";
import {
  BOARD_ORDER,
  Board,
  CalendarView,
  DataGrid,
  FilterBar,
  GridFooter,
  GridSkeleton,
  ItemDrawer,
  TableToolbar,
  TreeGrid,
  activeFilterCount,
  fromConditions,
  groupValue,
  nextSorts,
  orderColumns,
  seedFilters,
  toConditions,
  useCreateItem,
  applyRights,
  dayKey,
  periodRange,
  toPeriod,
  useDisabledDays,
  useDrawerLayout,
  useItems,
  useUpdateItem,
  Timeline,
  useUndatedRows,
  type CalendarPeriod,
  type TimelineScale,
  type Filters,
  type Item,
  type Sort,
} from "@/features/item";
import { useTableSchema, type Field, type Relation } from "@/features/table";
import type { DataLanguage } from "@/features/workspace";
import { toast } from "@/shared/lib/toast";
import { useUi } from "@/shared/lib/ui-store";
import { pinnedIds, resolveColumnIds } from "../model/columns";
import { hasUrl, openCreateUrl, openRowUrl } from "../model/url-template";
import type { RelationTab } from "../model/relation-tabs";
import { tabGroupField } from "../api/tab-group";
import { useExportExcel } from "../api/excel";
import { CalendarSetup } from "./CalendarFields";
import { ExcelImportDialog } from "./ExcelImportDialog";
import { ViewOptions } from "./ViewOptions";

/**
 * Правки настроек вкладки. Все они уезжают в раскладку карточки одним
 * и тем же PUT: у вкладки нет своей строки в базе, кроме `tab`.
 */
export type TabSettings = {
  /** Имя на конкретном языке ДАННЫХ. */
  onRename: (name: string, language: string) => void;
  /** Тип вкладки: таблица, доска, календарь. Набор — TAB_VIEW_TYPES. */
  onType: (type: string) => void;
  /** Поля дат календаря. Слаги: так их хранит база. */
  onDateFrom: (slug: string) => void;
  onDateTo: (slug: string) => void;
  /**
   * Поле, по значениям которого собраны колонки доски. У таблицы то же
   * поле разбивает её вкладками — настройка одна, и хранится она в той
   * же колонке view.
   */
  onTabGroup: (fieldId: string) => void;
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
  trail,
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
   * Путь до вкладки: таблица и запись, из карточки которой её открыли.
   * Нужен не самой вкладке, а карточке связанной строки — та закрывает
   * собой всё остальное, и без крошек непонятно, где мы оказались.
   */
  trail?: { label: string; onClick: () => void }[] | undefined;
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
  const {
    schema,
    isLoading: schemaLoading,
    error: schemaError,
    refetch: refetchSchema,
  } = useTableSchema(
    tab.tableSlug,
    tab.columnIds.length ? tab.columnIds : undefined,
  );

  /*
   * Раскладка ЧУЖОЙ таблицы — ради прав роли на её поля: в схеме
   * (GET /v2/fields) их нет вовсе, а раскладка подставляет их по роли
   * из токена (см. features/item/model/layout).
   *
   * Своей раскладки у чужой таблицы под нашим пунктом меню нет, и ручка
   * отдаёт её общую (layout.go:1197: не нашли по menu_id — берём
   * `is_default`). Прав это не меняет: они считаются по таблице и роли,
   * а не по раскладке. Тот же запрос делает карточка связанной строки —
   * ключ у него один, и второй раз он не уходит.
   */
  const layout = useDrawerLayout({ tableSlug: tab.tableSlug, menuId, language });

  /** Поля чужой таблицы, из которых убрано запрещённое роли. */
  const fields = useMemo(
    () => applyRights(schema.fields, layout.rights),
    [schema.fields, layout.rights],
  );

  /*
   * Колонки вкладки. Пустой список — обычное дело: бэкенд заводит вкладку
   * вместе со связью и колонок в неё не кладёт. Пустая вкладка ничего
   * не сообщает, поэтому по умолчанию показываются все поля чужой
   * таблицы, а сузить их можно настройкой вкладки.
   */
  const columns = useMemo(
    () => (tab.columnIds.length ? resolveColumnIds(tab.columnIds, fields) : fields),
    [tab.columnIds, fields],
  );

  /*
   * Тип вкладки — это тип её view. Доска здесь та же, что и на экране:
   * карточки в колонках по значению поля раскладки. Дерева среди типов
   * нет намеренно — см. tabTypes в features/view/model/types.
   */
  const board = tab.view.type === "BOARD";
  /** Поле, по значениям которого собраны колонки доски. */
  const boardField = board ? tabGroupField(tab.view, fields) : undefined;
  /* Колонку порядка заводит бэкенд под BOARD. Нет её — сортировать нечем,
     и сортировка по несуществующей колонке роняет весь запрос. */
  const boardReady = schema.fields.some((field) => field.slug === BOARD_ORDER);

  /*
   * Дерево вкладки собирается из тех же строк, что и таблица: ручка
   * дерева отбор по связи не понимает и показала бы всю чужую таблицу
   * (docs/backend-notes.md). Нужна рекурсивная колонка — иначе вешать
   * детей не на что.
   */
  const tree = tab.view.type === "TREE";
  const treeReady = schema.fields.some((field) => field.slug === `${tab.tableSlug}_id`);

  /*
   * Календарь вкладки. Тот же, что на экране, но период и видимый день
   * — своё состояние, а не адрес: адрес занят основной таблицей, из
   * карточки которой открыта вкладка. Цена та же, что у сортировки
   * и страницы вкладки: ссылка открывает её с текущего месяца.
   */
  const calendar = tab.view.type === "CALENDAR";
  /*
   * Таймлайн вкладки. Настройка у него та же, что у календаря — поля
   * дат, — и живёт он по тем же правилам: масштаб и видимый день своё
   * состояние, а не адрес.
   */
  const timeline = tab.view.type === "TIMELINE";
  /** Вкладки, отбирающие строки по видимому диапазону дат. */
  const dated = calendar || timeline;
  /* Поля дат — из ВСЕХ полей чужой таблицы: срок бывает и не показан
     колонкой вкладки, а событию он всё равно нужен. Ключ здесь слаг. */
  const dateFrom = fields.find((field) => field.slug === tab.view.dateFromSlug);
  const dateTo = fields.find((field) => field.slug === tab.view.dateToSlug);
  const dateStatus = fields.find((field) => field.slug === tab.view.statusFieldSlug);

  const [period, setPeriod] = useState<CalendarPeriod>(() => toPeriod(tab.view.period));
  const [scale, setScale] = useState<TimelineScale>("DAY");
  const [cursor, setCursor] = useState(() => new Date());
  /** Сколько месяцев ленты загружено вокруг курсора: MONTH — лента недель. */
  const [span, setSpan] = useState({ past: 1, future: 1 });
  /** Видимый диапазон: он же отбор строк календаря. */
  const range = useMemo(
    () => periodRange(timeline ? "MONTH" : period, cursor, span.past, span.future),
    [timeline, period, cursor, span],
  );

  /** Страницами не листается ни доска, ни дерево, ни календарь. */
  const whole = board || tree || dated;

  const [sorts, setSorts] = useState<Sort[]>([]);
  const [limit, setLimit] = useState(LIMIT);
  /** Страница вкладки. У view с прокруткой не используется. */
  const [page, setPage] = useState(1);
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

  /**
   * Отбор БЕЗ диапазона дат. Им же спрашивается список записей без дат
   * у таймлайна: диапазон отсёк бы как раз то, чего у них нет.
   */
  const scopeFilters = useMemo(
    () => ({ ...chips, ...scope, [link]: { op: "contains" as const, values: [value] } }),
    [chips, scope, link, value],
  );

  const filters = useMemo(
    () => ({
      ...chips,
      ...scope,
      /* Видимый диапазон календаря и таймлайна — такой же отбор, как
         остальные: `{поле: {$gte, $lte}}`. Стоит перед связью, потому
         что связь с открытой записью перекрыть нельзя ничем. */
      ...(dated && dateFrom
        ? {
            [dateFrom.slug]: {
              op: "between" as const,
              values: [dayKey(range.from), dayKey(range.to)],
            },
          }
        : {}),
      [link]: { op: "contains" as const, values: [value] },
    }),
    [chips, scope, dated, dateFrom, range, link, value],
  );

  /** Закреплённые колонки вкладки — в тех же ключах, что и у таблицы. */
  const pinned = useMemo(
    () => pinnedIds(tab.view.fixedColumnIds, columns),
    [tab.view.fixedColumnIds, columns],
  );

  /** Права роли на ЧУЖУЮ таблицу: у неё они свои. */
  const can = useTablePermission(tab.tableSlug);


  /** Постоянная ссылка: литерал в аргументе перезапрашивал бы строки. */
  const calendarSort = useMemo(
    () => (dateFrom ? [{ field: dateFrom.slug, direction: "asc" as const }] : NO_SORTS),
    [dateFrom],
  );

  /*
   * Нерабочие дни календаря — второй get-list в ещё одну чужую таблицу
   * (`view.disable_dates`). Не настроено — запроса нет вовсе.
   */
  const disabledDays = useDisabledDays({
    tableSlug: calendar ? tab.view.disableDates?.tableSlug ?? "" : "",
    daySlug: tab.view.disableDates?.daySlug ?? "",
    from: dayKey(range.from),
    to: dayKey(range.to),
  });

  const {
    page: rows,
    isLoading,
    hasMore,
    loadingMore,
    loadMore,
    error: rowsError,
    refetch: refetchRows,
  } = useItems(
    value && can.read && (!dated || Boolean(dateFrom)) ? tab.tableSlug : undefined,
    {
      /*
       * Ни доска, ни дерево, ни календарь не листаются страницами: доске
       * нужны колонки целиком, дереву — предки детей (разрезанные
       * страницей, они стали бы сиротами и всплыли в корень), календарю
       * — весь видимый диапазон.
       */
      limit: whole ? WHOLE_LIMIT : limit,
      page: whole ? 1 : page,
      /*
       * Режим листания у вкладки тот же, что у её view: вкладка и есть view.
       * Доска догружается прокруткой колонки; дерево — нет: догружать
       * в него нечем, оно показывает первую сотню связанных строк.
       */
      infinite: board || dated || tab.view.infiniteScroll,
      /* Доска сортируется своей колонкой порядка и ничем больше: карточки
       в ней расставляют руками, и чужая сортировка эту расстановку прячет.
       У дерева порядок свой — обход иерархии. Календарь сортируется полем
       начала: на сетке порядок не виден, но он решает, какие строки
       приедут, когда их в диапазоне больше порции. */
      sorts: board
        ? boardReady
          ? BOARD_SORT
          : NO_SORTS
        : tree
          ? NO_SORTS
          : dated
            ? calendarSort
            : sorts,
      filters,
      search,
    },
  );

  /*
   * Записи без дат — свой запрос, тот же, что и на экране: условия
   * «поле пусто» у get-list нет (features/item/api/timeline).
   */
  const undated = useUndatedRows({
    tableSlug: timeline && can.read && dateFrom && value ? tab.tableSlug : "",
    fromSlug: dateFrom?.slug ?? "",
    filters: scopeFilters,
    search,
  });

  const update = useUpdateItem(tab.tableSlug);
  const create = useCreateItem(tab.tableSlug);

  /**
   * Открыть связанную строку. Адрес из настроек вкладки важнее: у её view
   * есть свой `attributes.navigate`, и раз админ его задал — открывается
   * его страница, а не наша карточка. До этого настройка на вкладке
   * не действовала вовсе.
   */
  const openRelated = (guid: string) => {
    const row =
      rows.rows.find((item) => item.guid === guid) ??
      undated.find((item) => item.guid === guid);
    if (openRowUrl(tab.view, row)) return;

    setOpenGuid(guid);
  };

  /**
   * Завести связанную строку. Тоже сначала настройка: `url_object`
   * уводит на страницу проекта, и тогда ни строки, ни ссылки на родителя
   * мы не создаём — это делает та страница.
   */
  const createRelated = (values: Record<string, unknown>) =>
    openCreateUrl(tab.view)
      ? undefined
      : create.mutate(
          { ...values, [tab.fieldSlug]: parentGuid },
          { onSuccess: () => toast.success(t("table.rowCreated")) },
        );
  const exportExcel = useExportExcel(tab.tableSlug);
  // Ширины колонок вкладка помнит там же, где таблица: это настройка
  // экрана человека, и у одной таблицы она одна на все места показа.
  const { columnWidths: widths, setColumnWidth } = useUi();
  const [importing, setImporting] = useState(false);

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
  // Чтение чужой таблицы запрещено роли: вкладка честно говорит об этом,
  // а не показывает пустой список связанных записей.
  if (!can.read) return <p className="p-6 text-sm text-fg-muted">{t("table.noReadAccess")}</p>;
  if (schemaLoading || isLoading) return <GridSkeleton columns={columns.length || 4} />;
  // Отказ — словами сервера и с кнопкой: пустая вкладка вместо него
  // выглядела бы как «связанных записей нет».
  const failure = schemaError ?? rowsError;
  if (failure) return <Failure text={failure} onRetry={schemaError ? refetchSchema : refetchRows} />;
  if (!columns.length) return <p className="p-6 text-sm text-fg-muted">{t("table.noColumns")}</p>;

  /* И среди записей без дат: у таймлайна они лежат отдельным списком
     и в общую страницу не входят — иначе щелчок по такой строке
     не открывал бы ничего. */
  const openRow =
    rows.rows.find((item) => item.guid === openGuid) ??
    undated.find((item) => item.guid === openGuid);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Своя панель инструментов: у вкладки свой список, и искать
          в нём приходится ровно так же, как в основной таблице. */}
      <div className="flex h-9 shrink-0 items-center justify-end gap-0.5 border-b border-border px-2">
        <TableToolbar
          tableSlug={tab.tableSlug}
          columns={columns}
          language={language}
          /* Ни на доске, ни в дереве сортировки нет: там свой порядок —
             расставленный руками и обход иерархии. */
          sorts={whole ? NO_SORTS : sorts}
          {...(whole
            ? {}
            : {
                onSorts: (next: Sort[]) => {
                  setSorts(next);
                  setPage(1);
                },
              })}
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
              onType: settings.onType,
              onTabGroup: settings.onTabGroup,
              onDateFrom: settings.onDateFrom,
              onDateTo: settings.onDateTo,
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
          relations={schema.relations}
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

      {tree ? (
        /*
         * Дерево вкладки. Строки те же, что у таблицы, — уже отобранные
         * по связи; иерархию собирает сам TreeGrid по колонке `<слаг>_id`.
         * Строка, чьего родителя в наборе нет (он не связан с открытой
         * записью или отсеян фильтром), встаёт в корень — иначе её
         * не было бы видно вовсе.
         */
        !treeReady ? (
          <p className="p-6 text-sm text-fg-muted">{t("table.noTreeRelation")}</p>
        ) : (
          <TreeGrid
            tableSlug={tab.tableSlug}
            columns={columns}
            rows={rows.rows}
            pinned={pinned}
            widths={widths[tab.tableSlug]}
            onWidth={(fieldId, width) => setColumnWidth(tab.tableSlug, fieldId, width)}
            relations={schema.relations}
            locale={locale}
            language={language}
            selected={selected}
            onSelect={setSelected}
            onOpenRow={openRelated}
            {...(can.update
              ? {
                  onEdit: (guid: string, slug: string, value: unknown) =>
                    update.mutate({ guid, values: { [slug]: value } }),
                }
              : {})}
            /*
             * Дочерняя запись наследует две ссылки сразу: на родителя
             * в дереве и на открытую запись — иначе она выпадет из вкладки,
             * в которой её только что завели.
             */
            {...(tab.canCreate && can.write
              ? {
                  onAddChild: (parent: Item) =>
                    create.mutate(
                      {
                        [`${tab.tableSlug}_id`]: parent.guid ?? null,
                        [tab.fieldSlug]: parentGuid,
                      },
                      { onSuccess: () => toast.success(t("table.rowCreated")) },
                    ),
                }
              : {})}
          />
        )
      ) : board ? (
        /*
         * Доска вкладки. Поле раскладки задаётся в её настройках — без
         * него колонок нет, и доска честно говорит об этом вместо пустого
         * экрана. Текст тот же, что у доски на экране: причина одна.
         */
        !boardField ? (
          <p className="p-6 text-sm text-fg-muted">{t("board.noGroupField")}</p>
        ) : (
          <Board
            tableSlug={tab.tableSlug}
            columns={columns}
            rows={rows.rows}
            field={boardField}
            relations={schema.relations}
            locale={locale}
            language={language}
            hasMore={hasMore || loadingMore}
            onOpenRow={openRelated}
            /* Перенос карточки — обычная правка строки. Право нужно
               и на таблицу, и на само поле раскладки: роль, которой
               запрещено менять статус, не таскает карточки. */
            {...(can.update && boardField.editable
              ? {
                  onMove: (guid: string, values: Record<string, unknown>) =>
                    update.mutate({ guid, values }),
                }
              : {})}
            {...(can.update
              ? {
                  onEdit: (guid: string, slug: string, value: unknown) =>
                    update.mutate({ guid, values: { [slug]: value } }),
                }
              : {})}
            /*
             * Новая карточка сразу с двумя проставленными полями: колонка
             * доски и ссылка на открытую запись. Черновика, как на экране,
             * здесь нет — панель вкладки узкая, и вторая карточка поверх
             * первой ради одного поля не окупается.
             */
            {...(tab.canCreate && can.write
              ? {
                  onAddCard: (columnId: string) =>
                    createRelated({ [boardField.slug]: groupValue(boardField, columnId) }),
                }
              : {})}
            {...(hasMore ? { onEndReached: loadMore } : {})}
          />
        )
      ) : dated ? (
        /*
         * Календарь и таймлайн вкладки. Поле начала события задаётся
         * в её настройках — без него ни сетки дней, ни оси не будет
         * никогда. Экран настройки тот же, что и на большом экране:
         * причина одна.
         */
        !dateFrom ? (
          <CalendarSetup
            view={tab.view}
            // ВСЕ поля чужой таблицы: дата бывает и не показана колонкой.
            fields={schema.fields}
            language={language}
            {...(settings
              ? { onDateFrom: settings.onDateFrom, onDateTo: settings.onDateTo }
              : {})}
          />
        ) : timeline ? (
          <Timeline
            tableSlug={tab.tableSlug}
            columns={columns}
            rows={rows.rows}
            undated={undated}
            fromField={dateFrom}
            toField={dateTo}
            statusField={dateStatus}
            /* Группировки у вкладки нет ни на одном типе: её настройка
               живёт на экране, и таблица вкладки тоже идёт без неё. */
            groups={NO_GROUPS}
            relations={schema.relations}
            scale={scale}
            cursor={cursor}
            rangeFrom={range.from}
            rangeTo={range.to}
            locale={locale}
            language={language}
            hasMore={hasMore || loadingMore}
            onScale={setScale}
            onCursor={(next) => {
              setCursor(next);
              setSpan({ past: 1, future: 1 });
            }}
            /* Не функцией обновления: событий прокрутки за один жест
               десятки, и каждое добавляло бы по месяцу. */
            onLoadPast={() => setSpan({ ...span, past: span.past + 1 })}
            onLoadFuture={() => setSpan({ ...span, future: span.future + 1 })}
            onOpenRow={openRelated}
            {...(hasMore ? { onLoadMore: loadMore } : {})}
            {...(can.update && dateFrom.editable
              ? {
                  onMove: (guid: string, values: Record<string, unknown>) =>
                    update.mutate({ guid, values }),
                }
              : {})}
            /* Новая запись сразу с двумя проставленными полями: даты
               из протяжки и ссылка на открытую запись. */
            {...(tab.canCreate && can.write
              ? {
                  onCreate: (values: Record<string, unknown>) => createRelated(values),
                }
              : {})}
          />
        ) : (
          <CalendarView
            tableSlug={tab.tableSlug}
            columns={columns}
            rows={rows.rows}
            fromField={dateFrom}
            toField={dateTo}
            statusField={dateStatus}
            relations={schema.relations}
            period={period}
            cursor={cursor}
            rangeFrom={range.from}
            rangeTo={range.to}
            disabledDays={disabledDays}
            locale={locale}
            language={language}
            hasMore={hasMore || loadingMore}
            onPeriod={(next) => {
              setPeriod(next);
              setSpan({ past: 1, future: 1 });
            }}
            onCursor={(next) => {
              setCursor(next);
              setSpan({ past: 1, future: 1 });
            }}
            /* Не функцией обновления: событий прокрутки за один жест
               десятки, и каждое добавляло бы по месяцу. Значение из
               текущего рендера делает их повторы безвредными. */
            onLoadPast={() => setSpan({ ...span, past: span.past + 1 })}
            onLoadFuture={() => setSpan({ ...span, future: span.future + 1 })}
            onOpenRow={openRelated}
            {...(hasMore ? { onLoadMore: loadMore } : {})}
            /* Перенос и растягивание — обычная правка строки. Право
               нужно и на само поле начала: роль, которой запрещено
               менять дату, не двигает события. */
            {...(can.update && dateFrom.editable
              ? {
                  onMove: (guid: string, values: Record<string, unknown>) =>
                    update.mutate({ guid, values }),
                }
              : {})}
            /*
             * Новое событие сразу с двумя проставленными полями: даты
             * из клетки и ссылка на открытую запись. Черновика, как
             * на экране, здесь нет — панель вкладки узкая, и вторая
             * карточка поверх первой ради одного поля не окупается.
             */
            {...(tab.canCreate && can.write
              ? {
                  onCreate: (values: Record<string, unknown>) => createRelated(values),
                }
              : {})}
          />
        )
      ) : (
      <DataGrid
        tableSlug={tab.tableSlug}
        columns={columns}
        rows={rows.rows}
        relations={schema.relations}
        locale={locale}
        language={language}
        pinned={pinned}
        widths={widths[tab.tableSlug]}
        onWidth={(fieldId, width) => setColumnWidth(tab.tableSlug, fieldId, width)}
        selected={selected}
        onSelect={setSelected}
        sorts={sorts}
        onSort={(field, direction) => {
          setSorts(direction ? [{ field, direction }] : nextSorts(sorts, field));
          setPage(1);
        }}
        /* Правка — по правам роли на ЧУЖУЮ таблицу: у неё они свои,
           и от прав на таблицу, из которой открыли вкладку, не зависят. */
        {...(can.update
          ? {
              onEdit: (guid: string, slug: string, value: unknown) =>
                update.mutate({ guid, values: { [slug]: value } }),
            }
          : {})}
        // Связанная строка раскрывается на месте, поверх вкладки:
        // у чужой таблицы своего экрана в этом меню нет, а посмотреть
        // на неё целиком нужно чаще, чем перейти в её таблицу.
        onOpenRow={openRelated}
        {...(tab.view.infiniteScroll && hasMore ? { onEndReached: loadMore } : {})}
        /*
         * Ссылка на открытую запись проставляется сама: связанную строку
         * заводят ИЗ карточки, и заполнять её вручную значит предложить
         * человеку выбрать ту самую запись, из которой он смотрит.
         */
        /* Свой адрес создания заменяет строку-черновик — так же, как
           в таблице на экране. */
        {...(tab.canCreate && can.write && hasUrl(tab.view.objectUrl)
          ? { onAddRow: () => openCreateUrl(tab.view) }
          : {})}
        {...(tab.canCreate && can.write
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
      )}

      {/* Ни у доски, ни у календаря подвала нет: страницами их не
          листают. Сколько карточек в колонке — написано в её шапке,
          а календарь показывает свой диапазон целиком. */}
      {!whole && (
      <GridFooter
        {...(tab.view.infiniteScroll ? {} : { page, onPage: setPage })}
        shown={rows.rows.length}
        limit={limit}
        total={rows.count}
        loadingMore={loadingMore}
        selectedCount={0}
        deleting={false}
        onLimit={(next) => {
          setLimit(next);
          setPage(1);
        }}
      />
      )}

      {importing && (
        <ExcelImportDialog
          tableSlug={tab.tableSlug}
          // Все поля чужой таблицы: столбец файла можно положить
          // и в колонку, которой во вкладке не видно.
          fields={fields}
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
          fields={fields}
          relations={schema.relations}
          menuId={menuId}
          locale={locale}
          language={language}
          languages={languages}
          {...(onLanguage ? { onLanguage } : {})}
          /* К списку вкладки — крошкой с её именем: закрыть карточку
             и вернуться к строкам можно и крестиком, но из крошек это
             видно как путь, а не как «закрыть». */
          trail={[...(trail ?? []), { label: tab.label, onClick: () => setOpenGuid(null) }]}
          {...(can.update
            ? {
                onEdit: (guid: string, slug: string, value: unknown) =>
                  update.mutate({ guid, values: { [slug]: value } }),
              }
            : {})}
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
  trail,
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
  /** Путь до этой карточки: таблица, запись, вкладка. */
  trail: { label: string; onClick: () => void }[];
  /** Правка значения. Не задан — карточка открывается только на чтение. */
  onEdit?: ((guid: string, slug: string, value: unknown) => void) | undefined;
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
      trail={trail}
      {...(onEdit ? { onEdit } : {})}
      onClose={onClose}
    />
  );
}

/** Отказ во вкладке: причина и «повторить». */
function Failure({ text, onRetry }: { text: string; onRetry: () => void }) {
  const { t } = useTranslation();

  return (
    <div className="flex flex-col items-start gap-2 p-6">
      <p className="text-sm text-fg-muted">{text}</p>
      <button
        type="button"
        onClick={onRetry}
        className="h-8 rounded-md border border-border-strong px-3 text-sm text-fg transition-colors hover:bg-surface-hover"
      >
        {t("action.retry")}
      </button>
    </div>
  );
}

/** Строк на странице вкладки. Меньше, чем у таблицы: места под неё меньше. */
const LIMIT = 10;

/** Доска и дерево грузятся порциями по сто и догружаются прокруткой. */
const WHOLE_LIMIT = 100;

/** Постоянные ссылки: литерал в аргументе перезапрашивал бы строки. */
const NO_SORTS: Sort[] = [];

/** То же и для группировки: постоянная ссылка вместо литерала. */
const NO_GROUPS: Field[] = [];
const BOARD_SORT: Sort[] = [{ field: BOARD_ORDER, direction: "asc" }];

/** Постоянная ссылка: пустой литерал по умолчанию пересобирал бы поля. */
const EMPTY_LANGUAGES: DataLanguage[] = [];
