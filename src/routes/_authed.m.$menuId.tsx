import { useEffect, useMemo, useRef, useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import {
  BOARD_ORDER,
  Board,
  CalendarView,
  DataGrid,
  FilterBar,
  blankItem,
  groupValue,
  GridSkeleton,
  ItemDrawer,
  TreeGrid,
  GridFooter,
  MAX_LIMIT,
  MIN_LIMIT,
  TableToolbar,
  emptyFilter,
  filterKind,
  formatSorts,
  nextSorts,
  parseSorts,
  seedFilters,
  fieldIcon,
  applyRights,
  itemTitle,
  orderColumns,
  useCreateItem,
  useDeleteItems,
  useDisabledDays,
  useDrawerLayout,
  useItem,
  useItems,
  useUpdateItem,
  dayKey,
  periodRange,
  formatPivotSort,
  parsePivotSort,
  PivotView,
  toAggregation,
  toPeriod,
  toScale,
  Timeline,
  useUndatedRows,
  type CalendarPeriod,
  type PivotSetup,
  type TimelineScale,
  activeFilterCount,
  filtersSchema,
  fromConditions,
  parseFilters,
  relationDataKey,
  rowErrors,
  toConditions,
  type ColumnActions,
  type Filters,
  type Item,
  type Sort,
} from "@/features/item";
import { useTablePermissions } from "@/features/auth";
import { FileBrowser } from "@/features/files";
import { MicrofrontendPage } from "@/features/microfrontend";
import { EmbeddedPage, SidebarToggleButton, showsTable, useMenu } from "@/features/sidebar";
import {
  FieldEditor,
  TableActions,
  collapseLanguages,
  baseSlug,
  languageGroups,
  localizeKeys,
  localizeSlug,
  toDraft,
  useCreateField,
  useCreateRelation,
  useDeleteField,
  useDeleteRelation,
  ALL_VIEW_RIGHTS,
  useTableDetails,
  useTableSchema,
  useUpdateField,
  useUpdateRelation,
  type Field,
  type FieldDraft,
} from "@/features/table";
import { PrintButton } from "@/features/docs";
import {
  ExcelImportDialog,
  IMPLEMENTED_VIEW_TYPES,
  TAB_VIEW_TYPES,
  RelationView,
  CalendarSetup,
  ViewCreateButton,
  ViewOptions,
  ViewTabs,
  viewIcon,
  columnKey,
  pickView,
  pinnedIds,
  resolveColumns,
  relationTabs as relationTabsFromViews,
  tabbableRelations,
  tabViews,
  useCreateView,
  useDeleteView,
  useExportExcel,
  useMenuViews,
  tabGroupField,
  useTabGroup,
  useUpdateView,
  fillTemplate,
  hasUrl,
  openCreateUrl,
  openRowUrl,
  openUrl,
  type View,
} from "@/features/view";
import { useDataLanguages } from "@/features/workspace";
import { toast } from "@/shared/lib/toast";
import { useUi } from "@/shared/lib/ui-store";
import type { TranslationKey } from "@/shared/lib/i18n";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Tabs } from "@/shared/ui/tabs";

/**
 * Экран пункта меню. Маршрут ключуется на menuId, а не на слаге таблицы:
 * набор view принадлежит пункту меню (GET /v3/menus/{menuId}/views),
 * и два пункта могут показывать одну таблицу с разными наборами view.
 *
 * View, страница, сортировка, фильтры и поиск живут в адресе, а не
 * в состоянии: такую ссылку можно переслать, и она откроет ровно то же
 * самое. `catch` вместо ошибки — испорченный вручную адрес возвращает
 * к значению по умолчанию, а не роняет экран.
 *
 * limit необязателен намеренно: без него берётся default_limit из
 * настроек view, и только потом наш собственный запас.
 */
const FALLBACK_LIMIT = 20;

/**
 * Порция строк доски. Своя и постоянная: страницами доску не листают —
 * она догружается прокруткой, и переключателя размера у неё нет вовсе.
 * Двадцать карточек на пять колонок — это по четыре в каждой, то есть
 * запрос на каждый оборот колеса.
 */
const BOARD_LIMIT = 100;

/** Постоянная ссылка: пустой отбор не должен менять зависимости памяток. */
const EMPTY_FILTERS: Filters = {};

const searchSchema = z.object({
  view: z.string().optional(),
  /**
   * Номер страницы. Нужен, пока view листается страницами: у view
   * с бесконечной прокруткой места, на которое можно вернуться, нет,
   * и параметр в адресе не появляется.
   */
  page: z.number().int().min(1).default(1).catch(1),
  limit: z.number().int().min(MIN_LIMIT).max(MAX_LIMIT).optional().catch(undefined),
  /** «слаг:направление,…» — читаемо в адресной строке; разбор в features/item. */
  sort: z.string().optional().catch(undefined),
  /** Подшапка фильтров открыта. Своё состояние, потому что её видно и пустой. */
  filtersOpen: z.boolean().optional().catch(undefined),
  /** Раскрытая строка (guid). В адресе, а не в состоянии: ссылку на
      конкретную запись пересылают чаще, чем на список. */
  item: z.string().optional().catch(undefined),
  /** Открытая вкладка связи в карточке (id relation view). Пусто — сама карточка. */
  tab: z.string().optional().catch(undefined),
  /**
   * Открытая вкладка раскладки (`group_fields`) — значение поля, а не id.
   * Пусто или неизвестное значение — первая вкладка: набор вариантов
   * меняется вместе с данными, и ссылка на исчезнувший не должна
   * оставлять экран пустым.
   */
  group: z.string().optional().catch(undefined),
  /*
   * Фильтр — это условие и его аргументы. Аргументы всегда списком
   * строк, каким бы ни было условие: одна форма и в адресе, и в схеме,
   * и в проверке. Что значит каждая позиция — знает features/item.
   */
  /**
   * Видимый день календаря («ГГГГ-ММ-ДД») и его режим.
   *
   * В адресе, а не в состоянии, и не зря: видимый период — это ОТБОР
   * строк, такой же, как фильтр. Ссылка на «эту неделю» обязана
   * открывать ту же неделю, а не текущую.
   *
   * Пусто — сегодня и режим из настроек view.
   */
  day: z.string().optional().catch(undefined),
  period: z.string().optional().catch(undefined),
  /* Настройки сводной — в адресе: это не настройка view для всех,
     а то, как человек сейчас смотрит. */
  /* Полей строк несколько — через запятую: слаг поля это [a-z0-9_],
     запятой в нём быть не может. */
  pivotRow: z.string().optional().catch(undefined),
  pivotCol: z.string().optional().catch(undefined),
  pivotValue: z.string().optional().catch(undefined),
  pivotAgg: z.string().optional().catch(undefined),
  pivotSort: z.string().optional().catch(undefined),
  pivotSkipEmpty: z.boolean().optional().catch(undefined),
  filters: filtersSchema.optional().catch(undefined),
  search: z.string().optional().catch(undefined),
});

/**
 * Порция строк календаря. Как и у доски, своя и постоянная: страницами
 * календарь не листают, диапазон сужается переключением на неделю
 * и день. Не влезло — в шапке календаря появляется «Показать ещё».
 */
const CALENDAR_LIMIT = 200;

/** «ГГГГ-ММ-ДД» из адреса → день. Мусор — сегодня, а не пустой экран. */
function parseDay(value: string | undefined): Date {
  const parts = (value ?? "").split("-").map(Number);
  const [year, month, day] = parts;

  if (parts.length !== 3 || !year || !month || !day) return new Date();

  const date = new Date(year, month - 1, day);
  return Number.isNaN(date.getTime()) ? new Date() : date;
}

export const Route = createFileRoute("/_authed/m/$menuId")({
  validateSearch: searchSchema,
  component: MenuPage,
});

function MenuPage() {
  const { menuId } = Route.useParams();
  const search = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });
  const { t, i18n } = useTranslation();

  const menu = useMenu(menuId);
  // Язык ДАННЫХ — не локаль интерфейса: подписи вариантов и мультиязычных
  // полей хранятся на языках проекта, см. features/workspace.
  const { languages, current: language, setCurrent: setLanguage } = useDataLanguages();
  /** Коды языков данных: по ним сводятся языковые колонки. */
  const codes = useMemo(() => languages.map((item) => item.code), [languages]);

  const {
    views,
    isLoading: viewsLoading,
    error: viewsError,
    refetch: refetchViews,
  } = useMenuViews(menuId);
  const allTabs = useMemo(() => tabViews(views), [views]);
  const view = pickView(views, search.view);

  /*
   * Права роли на эту таблицу. Решают, что рисовать: настоящую проверку
   * делает сервер, и без гейта роль без прав всё равно получала бы 403 —
   * но уже после того, как переименовала вкладку у себя на экране.
   */
  const permissionOf = useTablePermissions();
  const can = permissionOf(view?.tableSlug);

  /*
   * Права роли на view: какие показывать вкладкой, какие давать править
   * и удалять. Приходят отдельной ручкой — ни в списке view, ни в схеме
   * их нет (см. api/table-details).
   */
  const { viewRights } = useTableDetails(view?.tableSlug);
  const rightsOf = (id: string) => viewRights.get(id) ?? ALL_VIEW_RIGHTS;

  /** Вкладки, которые роли позволено видеть. */
  const tabs = useMemo(
    () => allTabs.filter((item) => (viewRights.get(item.id) ?? ALL_VIEW_RIGHTS).view),
    [allTabs, viewRights],
  );

  /**
   * Открытый view роли смотреть не дают. Не переключаем на соседний
   * молча: по ссылке пришли в конкретную вкладку, и подмена выглядела бы
   * так, будто открылось то, что просили.
   */
  const viewForbidden = Boolean(view) && !rightsOf(view?.id ?? "").view;

  // Колонки view — не только «что показать», но и «что грузить»:
  // настройки связей за пределами этого списка никому не нужны.
  const {
    schema,
    isLoading: schemaLoading,
    error: schemaError,
    refetch: refetchSchema,
  } = useTableSchema(view?.tableSlug, view?.columnIds);
  /*
   * Порядок полей в drawer — свой, из раскладки пункта меню, и с колонками
   * таблицы не связан: перестановка в карточке не двигает колонки, а
   * перестановка колонок — поля карточки.
   */
  const drawerLayout = useDrawerLayout({ tableSlug: view?.tableSlug ?? "", menuId, language });

  /*
   * Поля таблицы, приведённые к правам роли: запрещённого к показу
   * здесь уже нет, запрещённое к правке — только для чтения. Права
   * приходят с раскладкой: в схеме полей их нет (см. model/layout).
   *
   * Отсюда растёт всё остальное — колонки, карточка, быстрые фильтры,
   * список колонок view, разбор файла Excel: поле, которого роли видеть
   * не положено, не должно всплыть ни в одном из них. Сырой список
   * остаётся ровно у одного места — проверки уникальности слага
   * в редакторе поля: там нужны ВСЕ слаги, включая скрытые, потому что
   * совпадение даёт 500.
   */
  const tableFields = useMemo(
    () =>
      applyRights(schema.fields, drawerLayout.rights).filter(
        // Колонка порядка карточек — служебная: её заводит бэкенд сам
        // под доску (pkg/helper/view.go), заполняет тоже он, и место
        // ей в раскладке доски, а не в списках полей.
        (field) => field.slug !== BOARD_ORDER,
      ),
    [schema.fields, drawerLayout.rights],
  );

  /** Колонки view — и таблицы, и карточки: порядок у них разный, набор один. */
  const viewFields = useMemo(() => resolveColumns(view, tableFields), [view, tableFields]);

  /*
   * Колонки таблицы: те же поля, но со сведёнными языковыми. Мультиязычное
   * поле лежит в схеме НЕСКОЛЬКИМИ колонками (`title_en`, `title_cyr`),
   * и без сведения таблица показывает их подряд с одинаковой подписью:
   * какая из них узбекская, видно только по значению.
   */
  const columns = useMemo(
    () => collapseLanguages(viewFields, codes, language),
    [viewFields, codes, language],
  );

  /*
   * Колонки группировки (attributes.group_by_columns) в порядке уровней.
   * Настройка хранит ключи колонок — id поля или id связи, — а таблице
   * нужны сами колонки с активным языком: у мультиязычного поля группа
   * считается по тому варианту, который показан.
   */
  const groupColumns = useMemo(() => {
    const ids = view?.groupByIds ?? [];

    return ids
      .map((id) => {
        const field = viewFields.find((item) => item.id === id || item.relationId === id);
        if (!field) return undefined;

        const slug = localizeSlug(field.slug, viewFields, codes, language);
        return columns.find((item) => item.slug === slug);
      })
      .filter((field): field is Field => Boolean(field));
  }, [view?.groupByIds, viewFields, codes, language, columns]);

  /*
   * Вкладки связей в карточке — это view пункта меню с `is_relation_view`
   * (см. features/view/model/relation-tabs). Того же списка, что и вкладки
   * экрана: второго запроса не нужно.
   */
  const relationTabs = useMemo(
    () =>
      relationTabsFromViews(views, schema.relations, language).filter(
        // Вкладка ведёт в ЧУЖУЮ таблицу: без права на чтение её строк
        // вкладки нет вовсе. Старая админка отбирала их по
        // `relation.permission.view_permission`, которого бэкенд
        // не отдаёт вообще (layout.go, GetRelation — этого поля нет
        // в запросе), и потому не показывала ни одной.
        (tab) => permissionOf(tab.tableSlug).read,
      ),
    [views, schema.relations, language, permissionOf],
  );

  /** Связи, которые ещё можно показать вкладкой. */
  const addableRelations = useMemo(
    () => tabbableRelations(schema.relations),
    [schema.relations],
  );
  /*
   * Поля карточки: колонки view в порядке раскладки, минус спрятанные
   * из карточки (`field_hide_layout`). Скрытие из карточки — настройка
   * раскладки, а не поля: колонкой таблицы то же поле остаётся.
   *
   * Языковые колонки здесь НЕ сводятся, в отличие от колонок таблицы:
   * карточка сводит их сама и по ним же понимает, что запись
   * мультиязычная и нужен переключатель языка. Со сведённым набором
   * от поля остаётся один вариант — переключать нечего, и полоса языков
   * не появлялась вовсе (флаг `enable_multilanguage` для этого не годится:
   * object_builder не пишет его при вставке, см. ADR-0004).
   */
  const drawerFields = useMemo(
    () =>
      orderColumns(viewFields, drawerLayout.order).filter(
        (field) => !drawerLayout.hidden.has(field.slug),
      ),
    [viewFields, drawerLayout.order, drawerLayout.hidden],
  );

  /*
   * Те же поля, сведённые к активному языку: столько их и показывает
   * карточка. Проверяется заполненность именно их — требовать перевод
   * на каждый язык проекта значит не дать создать запись, пока не набран
   * узбекский вариант.
   */
  const drawerColumns = useMemo(
    () => collapseLanguages(drawerFields, codes, language),
    [drawerFields, codes, language],
  );

  const supportedView = view ? IMPLEMENTED_VIEW_TYPES.has(view.type) : false;
  /*
   * TREE — тот же грид, но строки идут деревом по рекурсивной связи,
   * а данные — из своей ручки (/v2/items/{slug}/tree). Она не читает
   * ни фильтров, ни поиска, ни сортировки, поэтому их инструментов
   * у дерева нет — рабочие на вид кнопки без действия хуже отсутствующих.
   */
  const treeView = supportedView && view?.type === "TREE";
  /* Родителя ручка ищет в колонке `<слаг таблицы>_id` — без неё дерева нет. */
  const treeReady = schema.fields.some((field) => field.slug === `${view?.tableSlug}_id`);
  /*
   * BOARD — те же строки обычного get-list, разложенные по значениям
   * одного поля. Своя пара ручек у доски есть, но она сортирует по
   * `created_at` раньше, чем по `board_order`, то есть теряет ровно тот
   * порядок, ради которого доску и двигают мышью; фильтры она понимает
   * только вида «значение из списка» (docs/backend-notes.md).
   */
  const boardView = supportedView && view?.type === "BOARD";
  /* Колонку порядка заводит бэкенд под BOARD. Нет её — сортировать нечем. */
  const boardReady = schema.fields.some((field) => field.slug === BOARD_ORDER);
  /*
   * CALENDAR — те же строки, но отобранные по видимому диапазону дат
   * и разложенные по дням. Запрос обычный get-list: своей ручки
   * у календаря нет и не нужно.
   */
  const calendarView = supportedView && view?.type === "CALENDAR";
  /*
   * PIVOT — те же строки, сведённые в клетки. Серверной сводки нет
   * (docs/backend-notes.md, «Группировка»), поэтому считается загруженная
   * выборка, и экран об этом говорит счётчиком.
   */
  const pivotView = supportedView && view?.type === "PIVOT";
  const pivotSetup: PivotSetup = {
    rowSlugs: (search.pivotRow ?? "").split(",").filter(Boolean),
    colSlug: search.pivotCol ?? "",
    valueSlug: search.pivotValue ?? "",
    aggregation: toAggregation(search.pivotAgg),
    sort: parsePivotSort(search.pivotSort),
    skipEmpty: search.pivotSkipEmpty === true,
  };
  /*
   * TIMELINE — те же строки того же get-list, но каждая своей полосой
   * на общей оси дней. Настройка у него та же, что у календаря: поля
   * дат; ручка та же; отличается только раскладка.
   */
  const timelineView = supportedView && view?.type === "TIMELINE";
  /** Экраны, отбирающие строки по видимому диапазону дат. */
  const dateView = calendarView || timelineView;
  /** Режим: из адреса, иначе из настроек view, иначе месяц. */
  const period: CalendarPeriod = toPeriod(search.period || view?.period);
  /*
   * Масштаб оси таймлайна живёт в том же параметре адреса, что и режим
   * календаря: у view один экран, и держать два параметра под «как
   * показано» незачем. Настройка `view.period` сюда не идёт — она про
   * календарь, и «WEEK» на оси значения не имеет.
   */
  const scale: TimelineScale = toScale(search.period);
  /*
   * Сколько месяцев ленты загружено вокруг курсора. MONTH — это не один
   * месяц со стрелками, а лента недель: она прокручивается без конца
   * и подгружает соседние месяцы, как и в старой админке.
   *
   * Состояние привязано к курсору ключом, а не сбрасывается эффектом:
   * эффект сбрасывал бы его ПОСЛЕ рендера, то есть один запрос уходил
   * бы со старой лентой вокруг нового месяца.
   */
  /*
   * У таймлайна масштаб в этот ключ не входит: диапазон от него
   * не зависит, а сброс ленты на смене «дни ↔ месяцы» выбрасывал бы
   * всё, что человек догрузил прокруткой.
   */
  const spanKey = `${timelineView ? "TIMELINE" : period}|${search.day ?? ""}|${view?.id ?? ""}`;
  const [loaded, setLoaded] = useState({ key: spanKey, past: 1, future: 1 });
  const span = loaded.key === spanKey ? loaded : { key: spanKey, past: 1, future: 1 };
  /*
   * Видимый период целиком. Одной памяткой: `new Date()` внутри неё
   * даёт новый объект на каждый рендер, а от этих границ зависит отбор
   * строк — без памятки таблица перезапрашивалась бы бесконечно.
   */
  const calendar = useMemo(() => {
    const cursor = parseDay(search.day);
    /* У таймлайна масштаб не сужает диапазон: ось — лента месяцев
       в обоих масштабах, «дни» и «месяцы» меняют только ширину
       колонки. */
    return {
      cursor,
      ...periodRange(timelineView ? "MONTH" : period, cursor, span.past, span.future),
    };
  }, [search.day, period, timelineView, span.past, span.future]);
  /* Поля дат — из ВСЕХ полей таблицы: срок задачи бывает и не показан
     колонкой, а событию он всё равно нужен. Ключ здесь слаг, а не id. */
  const calendarFrom = tableFields.find((field) => field.slug === view?.dateFromSlug);
  const calendarTo = tableFields.find((field) => field.slug === view?.dateToSlug);
  /** Поле, вариантами которого красятся события. Только читается. */
  const calendarStatus = tableFields.find((field) => field.slug === view?.statusFieldSlug);
  /** Не с чего начать событие: поле не выбрано в настройках view. */
  const calendarNotReady = dateView && !calendarFrom;

  /*
   * Нерабочие дни — второй запрос в ЧУЖУЮ таблицу по тому же диапазону
   * (`view.disable_dates`). Не настроено — запроса нет вовсе.
   */
  const disabledDays = useDisabledDays({
    tableSlug: calendarView ? (view?.disableDates?.tableSlug ?? "") : "",
    daySlug: view?.disableDates?.daySlug ?? "",
    from: dayKey(calendar.from),
    to: dayKey(calendar.to),
  });
  /*
   * Размер страницы: из адреса, иначе последний выбранный для этой
   * таблицы, иначе настройка view. Значение из localStorage проверяется
   * — испорченное руками «0» оставило бы таблицу пустой навсегда.
   */
  const { tableLimits, setTableLimit, tableFilters, setTableFilters, columnWidths, setColumnWidth } =
    useUi();
  const rememberedLimit = view ? tableLimits[view.tableSlug] : undefined;
  const limit = boardView
    ? BOARD_LIMIT
    : dateView || pivotView
    ? CALENDAR_LIMIT
    : search.limit ??
      (rememberedLimit && rememberedLimit >= MIN_LIMIT && rememberedLimit <= MAX_LIMIT
        ? rememberedLimit
        : undefined) ??
      view?.defaultLimit ??
      FALLBACK_LIMIT;
  /*
   * Слаги условий переезжают на активный язык данных: сортировка,
   * заведённая при английском (`title_en`), после переключения на
   * кириллицу должна бить по `title_cyr` — по той колонке, которую
   * человек видит. Обычных полей переезд не касается.
   */
  const sorts = useMemo(
    () =>
      parseSorts(search.sort).map((sort) => ({
        ...sort,
        field: localizeSlug(sort.field, viewFields, codes, language),
      })),
    [search.sort, viewFields, codes, language],
  );

  /*
   * С группировкой поле группы сортируется первым — иначе одинаковые
   * значения не идут подряд и групп не собрать. Своя сортировка человека
   * по этому же полю задаёт направление, остальные работают внутри групп.
   */
  /** Сортировки полей группировки — по уровням, по возрастанию. */
  const groupSorts = useMemo(
    () => groupColumns.map((field) => ({ field: field.slug, direction: "asc" as const })),
    [groupColumns],
  );

  const querySorts = useMemo(() => {
    /*
     * Доска сортируется своей колонкой порядка и ничем больше: карточки
     * в ней расставляют руками, и чужая сортировка эту расстановку
     * прячет. Колонки может ещё не быть — бэкенд заводит её при
     * сохранении view типа BOARD, — а сортировка по несуществующей
     * колонке роняет запрос целиком.
     */
    if (boardView) return boardReady ? [{ field: BOARD_ORDER, direction: "asc" as const }] : [];
    /*
     * Календарь сортируется своим полем начала и ничем больше: на сетке
     * порядок строк не виден вовсе, но он решает, КАКИЕ строки приедут,
     * когда их в диапазоне больше порции. По дате — значит с начала
     * периода, а не наугад.
     */
    if (dateView) {
      const byDate = calendarFrom
        ? [{ field: calendarFrom.slug, direction: "asc" as const }]
        : [];

      /* С группировкой поля групп идут первыми — иначе одинаковые
         значения не идут подряд и групп не собрать. Календарю она
         не рисуется, и менять ему порядок строк незачем. */
      return timelineView ? [...groupSorts, ...byDate] : byDate;
    }
    if (!groupSorts.length) return sorts;

    /* Своя сортировка человека по полю группы задаёт ей направление,
       остальные работают внутри групп. */
    const slugs = new Set(groupSorts.map((sort) => sort.field));
    return [
      ...groupSorts.map((sort) => sorts.find((own) => own.field === sort.field) ?? sort),
      ...sorts.filter((sort) => !slugs.has(sort.field)),
    ];
  }, [boardView, boardReady, dateView, timelineView, calendarFrom, groupSorts, sorts]);

  /*
   * Фильтры в адресе отсутствуют — берём набор, предложенный админом
   * в настройках view (attributes.quick_filters). Пустой объект при
   * этом означает «человек убрал все чипы» и подсказку не возвращает:
   * иначе снятый фильтр возвращался бы сам.
   */
  /*
   * Отбор запоминается по паре «таблица + view»: два view одной таблицы
   * показывают разные колонки, и общий фильтр по ним — чужой.
   */
  const filtersKey = view ? `${view.tableSlug}|${view.id}` : "";
  // useMemo обязателен: parseFilters отдаёт новый объект на каждый вызов,
  // и без него отбор менял бы ссылку на каждый рендер.
  const stored = tableFilters[filtersKey];
  const rememberedFilters = useMemo(() => parseFilters(stored), [stored]);

  /*
   * Отбор по умолчанию — это ОБЛАСТЬ ВИДИМОСТИ view, а не начальное
   * значение фильтра. Так он работает в старой админке
   * (modules/Table/useTableProps.jsx: `{...filters, ...defaultFiltersMap}`),
   * и от этого зависит, какие строки человек вообще видит: админ ставит
   * «только активные записи» и рассчитывает, что снять это нельзя.
   *
   * Поэтому он не участвует в цепочке «адрес → память → подсказка»,
   * а домешивается к результату и перекрывает пользовательское условие
   * по тому же полю. Стоял он последним вариантом цепочки — и любой
   * фильтр, once осевший в localStorage, отменял его навсегда.
   */
  const defaultFilters = useMemo(
    () => fromConditions(view?.defaultFilters),
    [view?.defaultFilters],
  );

  /** Отбор, которым управляет человек: он в адресе, он же в подшапке.
      Слаги мультиязычных полей переезжают на активный язык — как у сортировки. */
  const filters: Filters = useMemo(
    () =>
      localizeKeys(
        search.filters ??
          rememberedFilters ??
          seedFilters(view?.quickFilterIds ?? [], tableFields),
        viewFields,
        codes,
        language,
      ),
    [search.filters, rememberedFilters, view?.quickFilterIds, tableFields, viewFields, codes, language],
  );

  /*
   * Раскладка вкладками (`group_fields`): таблица показывает строки
   * только выбранной вкладки. Это отбор, а не украшение, — поэтому
   * условие домешивается туда же, где область видимости view.
   *
   * Поля — ВСЕ, а не колонки: раскладывать по скрытой колонке нормально.
   */
  const tabGroup = useTabGroup({
    view,
    fields: tableFields,
    relations: schema.relations,
    language,
    selected: search.group,
    /*
     * Доске вкладки не нужны: из той же настройки она делает КОЛОНКИ —
     * те же значения, но показанные все сразу, а подписи колонок по
     * связи берёт из самих строк. Спрашивать ради этого полсотни чужих
     * строк незачем.
     */
    enabled: !boardView,
  });

  /* Условие вкладки на доске не домешивается — иначе на ней осталась
     бы одна колонка из десяти. */
  const tabFilters = boardView ? EMPTY_FILTERS : tabGroup.filters;
  /** Поле, значения которого стали колонками доски. */
  const boardField = boardView ? tabGroupField(view, tableFields) : undefined;
  /** Доску не по чему раскладывать: поле не выбрано в настройках view. */
  const boardNotReady = boardView && !boardField;

  /*
   * Видимый диапазон календаря — тоже отбор, и уходит он тем же
   * условием, что и всё остальное: `{поле: {$gte, $lte}}`.
   *
   * `view_type: "CALENDAR"` в тело не кладём, хотя старая админка его
   * шлёт: get-list берёт из тела только ключи, совпавшие со СЛАГАМИ
   * полей таблицы (object_builder.go:1249 — `if _, ok := fieldsM[key]`),
   * а остальное молча выбрасывает. То есть ключ ничего не делает.
   *
   * Известное ограничение: отбор идёт по НАЧАЛУ события. Многодневное,
   * начавшееся до видимого периода и в него заходящее, не приедет.
   * Так же считает и старая админка; вторым условием по полю конца
   * это не чинится — они соединяются через AND, а нужен OR.
   */
  const calendarFilters: Filters = useMemo(
    () =>
      dateView && calendarFrom
        ? {
            [calendarFrom.slug]: {
              op: "between" as const,
              values: [dayKey(calendar.from), dayKey(calendar.to)],
            },
          }
        : EMPTY_FILTERS,
    [dateView, calendarFrom, calendar],
  );

  /**
   * Отбор БЕЗ диапазона дат: область видимости view, вкладка и фильтры
   * человека. Список записей без дат спрашивается им же — диапазон
   * отсёк бы как раз то, чего у них нет.
   */
  const scopeFilters: Filters = useMemo(
    () => ({ ...filters, ...defaultFilters, ...tabFilters }),
    [filters, defaultFilters, tabFilters],
  );

  /** То, что действительно уходит в запрос. Область видимости — сверху. */
  const effectiveFilters: Filters = useMemo(
    () => ({ ...scopeFilters, ...calendarFilters }),
    [scopeFilters, calendarFilters],
  );

  /*
   * Поля, по которым свой фильтр всё равно перекрыт: область видимости
   * view и открытая вкладка. Чип по ним соврал бы — он показывал бы
   * условие, которого в запросе нет.
   */
  const lockedSlugs = useMemo(
    () =>
      new Set([
        ...Object.keys(defaultFilters),
        ...Object.keys(tabFilters),
        ...Object.keys(calendarFilters),
      ]),
    [defaultFilters, tabFilters, calendarFilters],
  );

  /** Закреплённые колонки — по id поля: DataGrid знает только их. */
  const pinned = useMemo(
    () => pinnedIds(view?.fixedColumnIds ?? [], columns),
    [columns, view?.fixedColumnIds],
  );

  /*
   * Слаг таблицы переживает удаление последнего view: ручке создания
   * нужен именно он, а в пункте меню лежит `table_id`. Без этого
   * удаление последней вкладки запирало бы пункт меню — «+» слал бы
   * запрос с пустым слагом.
   */
  const lastSlug = useRef("");
  useEffect(() => {
    if (view?.tableSlug) lastSlug.current = view.tableSlug;
  }, [view?.tableSlug]);
  const tableSlug = view?.tableSlug ?? (lastSlug.current || undefined);

  // Порядок — среди ВСЕХ view пункта меню, а не среди видимых: скрытые
  // правами никуда не делись, и новый view должен встать за ними.
  const createView = useCreateView({ menuId, tableSlug, order: allTabs.length + 1 });
  const deleteView = useDeleteView({ menuId, tableSlug: view?.tableSlug });
  const updateView = useUpdateView({ menuId, tableSlug: view?.tableSlug });
  const [deletingView, setDeletingView] = useState<View | null>(null);

  /**
   * Открыть view. Сбрасывается всё, что относилось к прежнему: у другого
   * набор колонок свой, и фильтр по чужому слагу вернёт пусто.
   *
   * Без id — «любой»: после удаления открытого view в адресе не должно
   * остаться ссылки на несуществующий, и pickView возьмёт первую вкладку.
   */
  const openView = (id?: string) =>
    setSearch({
      view: id,
      page: 1,
      item: undefined,
      tab: undefined,
      group: undefined,
      // Видимый период календаря — настройка ЭТОГО view: у соседнего
      // и поля дат другие, и типа CALENDAR может не быть вовсе.
      day: undefined,
      period: undefined,
      sort: undefined,
      filters: undefined,
      search: undefined,
      limit: undefined,
      filtersOpen: undefined,
    });

  /**
   * Щелчок по строке открывает карточку — если админ не задал своего
   * адреса. Задал (`attributes.navigate`) — уводим туда: у проекта своя
   * страница заказа, и карточка ей не замена.
   *
   * Вкладка связи сбрасывается вместе со строкой: у другой записи набор
   * вкладок тот же, а открытая — уже не та.
   *
   * Карточка на экране одна. Таблица под панелью остаётся рабочей, и
   * строку открывают, не закрыв черновик новой записи, — раньше эти две
   * карточки просто ложились друг на друга, и открытая строка оказывалась
   * под «Новой записью». Нетронутый черновик уходит молча; в заполненный
   * человек что-то ввёл, и молча выбрасывать это нельзя.
   */
  const openRow = (guid: string) => {
    /* И среди записей без дат: у таймлайна они лежат отдельным списком,
       а свой адрес перехода (`attributes.navigate`) им положен такой же,
       как всем остальным. */
    const row =
      rows.rows.find((item) => item.guid === guid) ??
      undated.find((item) => item.guid === guid);
    if (view && row && openRowUrl(view, row)) return;

    if (draftTouched()) {
      setPendingRow(guid);
      return;
    }

    setDraft(null);
    setSearch({ item: guid, tab: undefined });
  };

  /** Правка отбора: в адрес — чтобы переслать, в память — чтобы вернуться. */
  const applyFilters = (next: Filters) => {
    if (filtersKey) setTableFilters(filtersKey, next);
    setSearch({ filters: next, page: 1 }, true);
  };

  /**
   * Как листается этот view: прокруткой или номерами страниц.
   *
   * Доска — всегда прокруткой: страницу сервер режет по всей доске
   * сразу, и «страница 2» показала бы десять колонок, в каждой из
   * которых чужая середина списка.
   */
  /* Сводная считает загруженное, поэтому и грузит прокруткой: страницы
     здесь означали бы «итог по третьей странице». */
  const infinite = view?.infiniteScroll === true || boardView || dateView || pivotView;

  /** Экраны, на которых порядок строк не виден и сортировать нечего. */
  /** Экраны, на которых порядок строк не виден и сортировать нечего. */
  const sortless = boardView || dateView || pivotView;

  const {
    page: rows,
    isLoading: rowsLoading,
    isFetching,
    hasMore,
    loadingMore,
    loadMore,
    error: rowsError,
    refetch: refetchRows,
  } = useItems(
    /* Пока варианты вкладок едут, спрашивать строки рано: без условия
       вкладки приехал бы весь список, и тут же — второй запрос с ним.
       У доски без поля раскладки колонок нет вовсе — и строк спрашивать
       не для чего. */
    supportedView &&
    !treeView &&
    can.read &&
    !tabGroup.pending &&
    (!boardView || Boolean(boardField)) &&
    (!dateView || Boolean(calendarFrom))
      ? view?.tableSlug
      : undefined,
    {
      limit,
      page: search.page,
      infinite,
      sorts: querySorts,
      filters: effectiveFilters,
      search: search.search,
    },
  );

  /*
   * Записи БЕЗ дат: на оси их нет, но и потерять их нельзя — задачу
   * без срока ставят на таймлайн перетаскиванием. Отбором по диапазону
   * они отсекаются, а условия «поле пусто» у get-list нет вовсе
   * (см. features/item/api/timeline).
   */
  const undated = useUndatedRows({
    tableSlug: timelineView && can.read && calendarFrom ? (view?.tableSlug ?? "") : "",
    fromSlug: calendarFrom?.slug ?? "",
    filters: scopeFilters,
    search: search.search ?? "",
  });

  /*
   * Строка для карточки. Сначала из уже загруженной страницы — drawer
   * открывают из таблицы, и запрашивать её второй раз незачем.
   *
   * Иначе запрашиваем по guid. Это случай пересланной ссылки: у того,
   * кто её открыл, своя страница, свой отбор и своя сортировка, и нужной
   * строки в списке просто нет. Раньше карточка честно писала «запись
   * не найдена» — то есть ссылка на запись работала только у автора.
   */
  const loadedRow = rows.rows.find((item) => item.guid === search.item);
  const fetched = useItem(view?.tableSlug, search.item, Boolean(search.item) && !loadedRow);
  const drawerRow = loadedRow ?? fetched.item;

  /** Открытая вкладка связи. Их набор приходит из раскладки карточки. */
  const relationTab = relationTabs.find((item) => item.id === search.tab);

  /**
   * Типы, которыми бывает вкладка связи. Набор короче общего: дерево
   * отбор по связи не понимает и показало бы всю чужую таблицу целиком
   * (см. TAB_VIEW_TYPES).
   */
  const tabTypes = useMemo(
    () =>
      TAB_VIEW_TYPES.map((type) => ({
        type,
        label: t(`view.type.${type}` as TranslationKey, { defaultValue: type }),
        icon: viewIcon(type),
      })),
    [t],
  );

  /*
   * Путь до карточки. Первая крошка — пункт меню: она же подписывает
   * экран в шапке, и возвращает туда же, где строка была открыта.
   *
   * Собирается из того, что уже на экране: имя меню и заголовок строки
   * лежат в загруженных данных, и ни одного запроса крошки не стоят.
   */
  const drawerTrail = [
    {
      label: menu?.label ?? t("menu.title"),
      onClick: () => setSearch({ item: undefined, tab: undefined }),
    },
  ];

  /*
   * Подшапка открыта, если её открыли явно или в ней уже что-то есть:
   * прятать заданный отбор нельзя — таблица показывала бы неполный
   * список без единого следа на экране.
   *
   * Открывается она по СВОЕМУ отбору, а считается по действующему.
   * Отбор по умолчанию снять нечем, и раскрывать ради него пустую
   * подшапку незачем; но значок «список неполный» гореть обязан —
   * иначе урезанная выборка ничем на экране не отмечена.
   */
  const activeFilters = activeFilterCount(effectiveFilters);
  const filtersVisible =
    search.filtersOpen ?? (activeFilterCount(filters) > 0 || sorts.length > 0);

  const [selected, setSelected] = useState<Set<string>>(() => new Set());
  /*
   * Новая запись, которую заполняют в карточке. Своё состояние, а не
   * адрес: полузаполненный черновик в ссылке бессмыслен — переслать
   * его нельзя, а восстановить нечем.
   */
  const [draft, setDraft] = useState<Item | null>(null);
  const [showErrors, setShowErrors] = useState(false);
  /**
   * Черновик в момент создания. Заводят его по-разному — с умолчаниями
   * схемы, со ссылкой на родителя в дереве, со значением колонки доски,
   * — поэтому «нетронутый» здесь не «без значений», а «такой же, каким
   * начался».
   */
  const initialDraft = useRef("");
  /**
   * Строка, которую откроют, как только решится судьба черновика.
   * Пусто — вопрос не задан.
   */
  const [pendingRow, setPendingRow] = useState<string | null>(null);
  const [confirming, setConfirming] = useState(false);

  /**
   * Завести черновик. Одно место на все три способа (кнопка в шапке,
   * «дочерняя» в дереве, «+» в колонке доски): здесь же закрывается
   * открытая карточка строки — панель на экране одна.
   */
  const startDraft = (item: Item) => {
    setShowErrors(false);
    setDraft(item);
    initialDraft.current = JSON.stringify(item);
    setSearch({ item: undefined, tab: undefined });
  };

  /** Трогали ли черновик. Нетронутый закрывается без вопросов. */
  const draftTouched = () => draft !== null && JSON.stringify(draft) !== initialDraft.current;
  /** Строка под урной у правого края — удаление одной, со своим диалогом. */
  const [deletingRow, setDeletingRow] = useState<string | null>(null);
  const remove = useDeleteItems(view?.tableSlug);
  const update = useUpdateItem(view?.tableSlug);
  const create = useCreateItem(view?.tableSlug);
  const createField = useCreateField(view?.tableSlug);
  const updateField = useUpdateField(view?.tableSlug);
  const deleteField = useDeleteField(view?.tableSlug);
  // Связь заводится своей ручкой: это не колонка, и слаг ей задаёт
  // целевая таблица. См. features/table/api/tables.
  const createRelation = useCreateRelation(view?.tableSlug);
  const updateRelation = useUpdateRelation(view?.tableSlug);
  const deleteRelation = useDeleteRelation(view?.tableSlug);

  /*
   * Панель поля одна на создание и на правку: разница только в том,
   * с чего она начинается. `field` — то, что правим; null — новое поле.
   * Якорь — то место, откуда её открыли: заголовок колонки, кнопка «+»
   * или раскрытая ячейка.
   */
  const [fieldPanel, setFieldPanel] = useState<{ field: Field | null; anchor: DOMRect } | null>(
    null,
  );
  const [deletingField, setDeletingField] = useState<Field | null>(null);
  const [importing, setImporting] = useState(false);
  const exportExcel = useExportExcel(view?.tableSlug);

  /*
   * Выделение сбрасывается, как только меняется набор строк. Отмеченная
   * строка, уехавшая на другую страницу, осталась бы отмеченной невидимо
   * — и удалилась бы вместе с теми, что человек видит.
   */
  const rowSetKey = `${view?.id}|${search.page}|${limit}|${search.sort}|${search.search}|${JSON.stringify(effectiveFilters)}`;
  useEffect(() => setSelected(new Set()), [rowSetKey]);

  /**
   * Уточнения одного и того же экрана — фильтр, поиск, сортировка —
   * заменяют запись в истории, а не добавляют новую: иначе «назад»
   * отматывает по одному нажатию клавиши в поле даты. Переход на другую
   * страницу или view остаётся настоящим шагом назад.
   */
  const setSearch = (next: Partial<typeof search>, replace = false) =>
    void navigate({ search: (prev) => ({ ...prev, ...next }), replace });

  /** Подпись сохраняется на языке ДАННЫХ, а не интерфейса: её увидят все. */
  const saveField = (draft: FieldDraft) => {
    const field = fieldPanel?.field;

    if (field) updateField.mutate({ field, draft, language });
    else createField.mutate({ draft, language });
  };

  /**
   * Меню колонки: правки СХЕМЫ, а не запроса. Одно и то же у таблицы
   * и у дерева — поле принадлежит таблице, а не тому, как её показывают.
   *
   * Фильтра здесь нет: он относится к запросу, и у дерева его ручка
   * не читает. Таблица дописывает его себе сама.
   */
  const columnActions: ColumnActions = {
    // Переименование — единственная правка схемы, которую делают
    // на бегу: остальное открывает диалог.
    rename: (field, label) =>
      updateField.mutate({ field, draft: { ...toDraft(field, language), label }, language }),
    settings: (field, anchor) => setFieldPanel({ field, anchor }),
    remove: setDeletingField,
  };

  /** Незаполненные обязательные и непрошедшие проверку поля черновика. */
  const draftErrors = useMemo(
    () => (draft ? rowErrors(drawerColumns, draft) : new Map()),
    [draft, drawerColumns],
  );

  const supported = menu ? showsTable(menu) : true;
  /*
   * Файловое хранилище — не таблица: ни view, ни строк у него нет,
   * и весь экран под ним другой. Папка задана настройкой пункта
   * (`attributes.path`), и пункт меню и ЕСТЬ папка.
   */
  const filesFolder = menu?.type === "MINIO_FOLDER" ? menu.folder : "";

  return (
    <div className="flex h-full flex-col">
      <header className="flex h-header shrink-0 items-center gap-2 border-b border-border px-4">
        <SidebarToggleButton />
        <span className="text-sm font-medium">{menu?.label ?? t("menu.title")}</span>
        {/* Число без слова: «16 записей» требует согласования по падежу
            в русском и узбекском, а множественные формы i18next стоят
            трёх ключей на язык ради одного счётчика. */}
        {supportedView && !treeView && (
          <span className="text-xs text-fg-muted">· {rows.count}</span>
        )}
        {isFetching && !rowsLoading && (
          <span className="text-xs text-fg-subtle">{t("common.loading")}</span>
        )}
      </header>

      {/* Полоса вкладок живёт и при открытом view неподдержанного типа:
          иначе доска прячет вкладки вместе с собой, и вернуться к таблице
          можно только кнопкой «назад» в браузере.

          И при нуле вкладок тоже — ради «+»: пункт меню без view иначе
          становится тупиком, из которого нечем завести первый. */}
      {supported && (tabs.length > 0 || (can.viewCreate && tableSlug)) && (
        <div className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-3">
          <ViewTabs
            views={tabs}
            activeId={view?.id ?? ""}
            language={language}
            onSelect={(next) => openView(next.id)}
          />

          {can.viewCreate && tableSlug && (
            <ViewCreateButton
              busy={createView.isPending}
              /* Поля таблицы: из них выбираются даты календаря вторым
                 шагом создания. Все, а не колонки, — срок задачи бывает
                 и не показан. */
              fields={tableFields}
              language={language}
              // Новая вкладка сразу открывается: её создали, чтобы в неё
              // смотреть. Список к этому моменту уже перезапрошен — см.
              // useCreateView, иначе вкладки дёрнулись бы на первую и обратно.
              onCreate={(name, type, dates) =>
                createView.mutate(
                  { name, language, type, ...(dates ?? {}) },
                  { onSuccess: (created) => created?.id && openView(created.id) },
                )
              }
            />
          )}

          {/* Инструменты — только там, где есть на что их применить.
              У роли без права на чтение строк нет вовсе: поиск, отбор
              и действия нажимались бы вхолостую, а настройки правили бы
              таблицу, которую ей не показывают. */}
          {view && can.read && !viewForbidden && (
            <div className="ml-auto flex shrink-0 items-center gap-0.5">
              {/* Поиск, отбор и сортировка — про таблицу: у нарисованного
                  заглушкой view искать нечего, а ручка дерева их не читает. */}
              {supportedView && !treeView && (
                <TableToolbar
                  tableSlug={view.tableSlug}
                  columns={columns}
                  language={language}
                  sorts={sortless ? [] : sorts}
                  /* Ни на доске, ни в календаре сортировки нет: там
                     порядок строк не виден вовсе — карточки расставлены
                     руками, события стоят по своим датам. */
                  {...(sortless
                    ? {}
                    : {
                        onSorts: (next: Sort[]) =>
                          setSearch({ sort: formatSorts(next), page: 1 }, true),
                      })}
                  filtersOpen={filtersVisible}
                  filterCount={activeFilters}
                  // Закрытие не стирает сами фильтры: спрятать строку и снять
                  // отбор — разные намерения.
                  onToggleFilters={() => setSearch({ filtersOpen: !filtersVisible }, true)}
                  search={search.search ?? ""}
                  onSearch={(next) => setSearch({ search: next || undefined, page: 1 }, true)}
                />
              )}

              {/* Действия таблицы: функции проекта над отмеченными
                  строками. Рядом с настройками, а не среди поиска
                  и фильтра: это не способ посмотреть на список,
                  а способ что-то с ним сделать. */}
              {supportedView && (
                <TableActions
                  tableSlug={view.tableSlug}
                  language={language}
                  languages={languages}
                  selected={[...selected]}
                  canEdit={can.settings}
                />
              )}

              {/* «Новая запись» карточкой, а не строкой в таблице:
                  у таблицы в сорок колонок заполнять запись вбок —
                  это горизонтальная прокрутка на каждое поле. Строкой
                  она по-прежнему заводится тоже, в подвале таблицы. */}
              {supportedView && can.write && (
                <button
                  type="button"
                  onClick={() => {
                    /* Адрес из настроек view важнее карточки: «Новая
                       запись» ведёт на страницу проекта, если админ её
                       задал (`attributes.url_object`). */
                    if (view && openCreateUrl(view)) return;

                    startDraft(blankItem(drawerColumns));
                  }}
                  className="mr-1 h-7 shrink-0 rounded-md bg-accent-solid px-3 text-sm font-medium text-accent-fg transition-opacity hover:opacity-90"
                >
                  {t("table.addRow")}
                </button>
              )}

              {/* Настройки — последними в ряду: это не действие над строками,
                  а вход в настройку всего экрана, и стоять он должен с краю,
                  а не между поиском и фильтром.

                  Открыты и у view, который мы не рисуем: иначе смена типа
                  на доску запирает view навсегда — панель, из которой тип
                  меняют, исчезает вместе с таблицей. */}
              <ViewOptions
                view={view}
                // Все поля таблицы, а не колонки view: скрытые нужно
                // показать, иначе вернуть их будет неоткуда.
                fields={tableFields}
                language={language}
                languages={languages}
                defaultFilters={defaultFilters}
                /* Настройка view — право не только на таблицу, но и
                   на сам view: их выдают по отдельности. */
                can={{ ...can, settings: can.settings && rightsOf(view.id).edit }}
                exporting={exportExcel.isPending}
                busy={updateView.isPending}
                handlers={{
                  onRename: (name, nameLanguage) =>
                    updateView.mutate({ view, name, language: nameLanguage }),
                  onType: (type) => updateView.mutate({ view, type }),
                  onColumns: (columns) => updateView.mutate({ view, columns }),
                  onQuickFilters: (quickFilters) => updateView.mutate({ view, quickFilters }),
                  onFixedColumns: (fixedColumns) => updateView.mutate({ view, fixedColumns }),
                  onDefaultFilters: (next) =>
                    updateView.mutate({ view, defaultFilters: toConditions(next) }),
                  onNavigate: (navigate) => updateView.mutate({ view, navigate }),
                  onObjectUrl: (objectUrl) => updateView.mutate({ view, objectUrl }),
                  onPdfUrl: (pdfUrl) => updateView.mutate({ view, pdfUrl }),
                  onInfiniteScroll: (infiniteScroll) =>
                    updateView.mutate({ view, infiniteScroll }),
                  onGroupBy: (groupBy) => updateView.mutate({ view, groupBy }),
                  // Поля дат календаря. Смена поля начала сбрасывает
                  // видимый день: диапазон считался по прежнему полю,
                  // и оставлять его — показывать чужой отбор.
                  onDateFrom: (dateFrom) => {
                    updateView.mutate({ view, dateFrom });
                    setSearch({ day: undefined });
                  },
                  onDateTo: (dateTo) => updateView.mutate({ view, dateTo }),
                  // Смена поля раскладки сбрасывает открытую вкладку:
                  // её значение принадлежало прежнему полю.
                  onTabGroup: (tabGroup) => {
                    updateView.mutate({ view, tabGroup });
                    setSearch({ group: undefined, page: 1 });
                  },
                  onEditField: (field, anchor) => setFieldPanel({ field, anchor }),
                  // Тот же диалог подтверждения, что и у меню колонки:
                  // удаление поля сносит его во всех view вместе с данными.
                  onDeleteField: setDeletingField,
                  onImport: () => setImporting(true),
                  // Выгружается то, что видно: колонки view и действующий
                  // отбор — вместе с областью видимости view, иначе файл
                  // содержал бы строки, которых на экране нет.
                  onExport: () =>
                    exportExcel.mutate({
                      fieldIds: columns.map((field) => field.id),
                      filters: effectiveFilters,
                      search: search.search ?? "",
                    }),
                  // Удаляется любой view, включая последний: так же ведёт
                  // себя старая админка. Пункт меню без view не тупик —
                  // «+» в полосе вкладок остаётся на месте.
                  //
                  // Право на удаление — своё, отдельное от права
                  // на правку: роль, которая настраивает view, не обязана
                  // иметь возможность его снести.
                  ...(rightsOf(view.id).delete ? { onDelete: () => setDeletingView(view) } : {}),
                }}
              />
            </div>
          )}
        </div>
      )}

      {menu?.embedUrl ? (
        /* Пункт со встроенной страницей: показываем её рамкой. Ни view,
           ни строк у такого пункта нет — как и у хранилища. */
        <EmbeddedPage url={menu.embedUrl} title={menu.label} />
      ) : menu?.type === "MINIO_FOLDER" ? (
        /* Право на запись берём с самого пункта меню: таблицы за ним
           нет, а значит нет и прав на таблицу. */
        <FileBrowser folder={filesFolder} canWrite={menu.can.write} />
      ) : menu?.type === "MICROFRONTEND" || menu?.microfrontendId ? (
        /* Чужое приложение внутри админки. Не рамкой, а модулем федерации
           — ему нужны наш токен, язык и окружение; цена этого решения
           записана в docs/adr/0005.
           Тип проверяется наравне с идентификатором: у пункта без
           выбранного приложения экран ЕСТЬ, и сказать он должен
           «приложение не выбрано», а не «экрана нет». */
        <MicrofrontendPage id={menu.microfrontendId} params={menu.params} />
      ) : !supported ? (
        <Notice text={t("menu.notImplemented")} />
      ) : viewsLoading ? (
        // Пока список view едет, «у таблицы нет view» — не правда, а
        // мигание: через мгновение он приедет и таблица нарисуется.
        <GridSkeleton />
      ) : viewsError ? (
        <Notice text={viewsError} actions={[{ label: t("action.retry"), onClick: refetchViews }]} />
      ) : !view ? (
        <Notice text={t("table.noView")} />
      ) : viewForbidden ? (
        /* Ссылка на view, который роли смотреть не дают. Соседний
           не подставляем: открылось бы не то, что просили. */
        <Notice text={t("table.noViewAccess")} />
      ) : !supportedView ? (
        <Notice text={t("table.notImplementedView")} />
      ) : !can.read ? (
        /* Чтение таблицы запрещено роли: строк не спрашиваем вовсе.
           Пункт меню при этом бывает виден — права на меню и на таблицу
           разные, и запрещают их по отдельности. */
        <Notice text={t("table.noReadAccess")} />
      ) : schemaLoading ? (
        <GridSkeleton />
      ) : schemaError ? (
        /* Схема не приехала — колонок нет ни одной, и без этой ветки
           экран предлагал бы завести первое поле в таблице, которая
           просто не ответила. */
        <Notice text={schemaError} actions={[{ label: t("action.retry"), onClick: refetchSchema }]} />
      ) : !columns.length ? (
        /*
         * Тупик без этой кнопки: «+» для нового поля живёт в шапке
         * таблицы, а таблица без колонок не рисуется вовсе. В таблице
         * без полей завести первое было нечем.
         *
         * Случая два, и действие у них разное: полей нет вовсе — их
         * заводят; поля есть, но ни одно не выбрано в этом view — их
         * показывают. Один текст на оба врал бы про половину.
         */
        <Notice
          text={tableFields.length ? t("table.noColumns") : t("table.noFieldsYet")}
          actions={[
            // Поля есть, но ни одно не показано — их показывают, а не заводят.
            ...(tableFields.length && can.columns
              ? [
                  {
                    label: t("view.showAll"),
                    onClick: () =>
                      updateView.mutate({ view, columns: tableFields.map(columnKey) }),
                  },
                ]
              : []),
            // Завести новое поле нужно и в пустой таблице, и в непустой:
            // человек пришёл сюда добавить колонку, а не искать спрятанную.
            ...(can.addField
              ? [
                  {
                    label: t("table.addField"),
                    onClick: (anchor: DOMRect) => setFieldPanel({ field: null, anchor }),
                  },
                ]
              : []),
          ]}
        />
      ) : treeView ? (
        !treeReady ? (
          /* Кнопка обязательна: «+» нового поля живёт в шапке грида,
             а грид без рекурсивной колонки не рисуется — без неё
             из этого экрана некуда идти. */
          <Notice
            text={t("table.noTreeRelation")}
            actions={
              can.addField
                ? [
                    {
                      label: t("table.addField"),
                      onClick: (anchor: DOMRect) => setFieldPanel({ field: null, anchor }),
                    },
                  ]
                : []
            }
          />
        ) : (
          <TreeGrid
            tableSlug={view.tableSlug}
            columns={columns}
            pinned={pinned}
            widths={view ? columnWidths[view.tableSlug] : undefined}
            onWidth={(fieldId: string, width: number) =>
              setColumnWidth(view.tableSlug, fieldId, width)
            }
            relations={schema.relations}
            locale={i18n.language}
            language={language}
            selected={selected}
            onSelect={setSelected}
            /* Через ту же дверь, что и таблица: своя ветка мимо openRow
               проходила и мимо адреса записи, и мимо черновика. */
            onOpenRow={openRow}
            {...(can.update
              ? {
                  onEdit: (guid: string, slug: string, value: unknown) =>
                    update.mutate({ guid, values: { [slug]: value } }),
                }
              : {})}
            {...(can.write
              ? {
                  /*
                   * «Дочерняя запись» — та же карточка создания, но
                   * родитель уже выбран: значение в колонку-ссылку,
                   * сама строка — рядом (`<слаг>_data`), из неё ячейка
                   * берёт подпись. Наружу `_data` не уезжает — его
                   * отбрасывает useCreateItem.
                   *
                   * Адрес из настроек здесь НЕ действует: на чужую
                   * страницу родителя не передать, а дочерняя запись без
                   * родителя — это не то, что просили.
                   */
                  onAddChild: (parent: Item) => {
                    startDraft({
                      ...blankItem(drawerColumns),
                      [`${view.tableSlug}_id`]: parent.guid ?? null,
                      [relationDataKey(`${view.tableSlug}_id`)]: parent,
                    });
                  },
                }
              : {})}
            {...(can.addField
              ? { onAddField: (anchor: DOMRect) => setFieldPanel({ field: null, anchor }) }
              : {})}
            /* Тот же диалог подтверждения, что и у таблицы. */
            {...(can.delete
              ? {
                  onDeleteSelected: () => setConfirming(true),
                  onDeleteRow: setDeletingRow,
                  deleting: remove.isPending,
                }
              : {})}
            /* Меню колонки — то же, что у таблицы, минус сортировка
               и фильтр: их ручка дерева не читает, и в меню их нет.
               Правки схемы к способу показа отношения не имеют. */
            columnActions={columnActions}
          />
        )
      ) : (
        <>
          {/* Раскладка вкладками: строки показываются по одной вкладке
              за раз. Полоса та же, что и у view, — вкладка есть вкладка,
              и двух разных на одном экране быть не должно.

              У доски та же настройка рисует колонки, а не вкладки:
              полоса поверх доски дублировала бы её же шапки. */}
          {tabGroup.tabs.length > 0 && !boardView && (
            <div className="flex h-11 shrink-0 items-center border-b border-border px-3">
              <Tabs
                tabs={tabGroup.tabs}
                activeId={tabGroup.activeId}
                onSelect={(id) => setSearch({ group: id, page: 1 }, true)}
              />
            </div>
          )}

          {filtersVisible && (
            <FilterBar
              columns={columns}
              relations={schema.relations}
              language={language}
              filters={filters}
              // Ни у доски, ни у календаря своей сортировки нет.
              sorts={sortless ? [] : sorts}
              // Поля, закрытые отбором по умолчанию: свой фильтр по ним
              // всё равно перекрывается настройкой view, и чип соврал бы.
              locked={lockedSlugs}
              // Любая правка отбора возвращает на первую страницу: на
              // седьмой после сужения выборки обычно пусто.
              onFilters={applyFilters}
              onSorts={(next) => setSearch({ sort: formatSorts(next), page: 1 }, true)}
            />
          )}

          {/* Пока варианты вкладок едут, запрос строк не запущен вовсе —
              и пустая таблица врала бы «записей нет». */}
          {boardNotReady ? (
            /* Доска без поля раскладки — не пустая сетка: рисовать
               нечего, пока не выбрано, что считать колонками. */
            <Notice text={t("board.noGroupField")} />
          ) : calendarNotReady ? (
            /* Календарь без поля начала — не пустая сетка: событию
               не с чего начаться. Выбор полей стоит прямо здесь, а не
               только за «⋯»: настройка, без которой экран пуст, не должна
               требовать догадки, где она лежит. */
            <CalendarSetup
              view={view}
              fields={tableFields}
              language={language}
              {...(can.settings && rightsOf(view.id).edit
                ? {
                    onDateFrom: (dateFrom: string) => {
                      updateView.mutate({ view, dateFrom });
                      setSearch({ day: undefined });
                    },
                    onDateTo: (dateTo: string) => updateView.mutate({ view, dateTo }),
                  }
                : {})}
            />
          ) : rowsLoading || tabGroup.pending ? (
            <GridSkeleton columns={columns.length} />
          ) : rowsError ? (
            /* Отказ показывается словами сервера. Пустая таблица вместо
               него врала бы: «записей нет» и «спросить не дали» — разные
               вещи, и вторая чинится, а первая нет. */
            <Notice
              text={rowsError}
              actions={[{ label: t("action.retry"), onClick: refetchRows }]}
            />
          ) : boardView && boardField ? (
            <Board
              tableSlug={view.tableSlug}
              columns={columns}
              rows={rows.rows}
              field={boardField}
              relations={schema.relations}
              locale={i18n.language}
              language={language}
              hasMore={hasMore || loadingMore}
              onOpenRow={openRow}
              /* Перенос карточки — обычная правка строки: значение
                 колонки и номер позиции одним PUT.

                 Право нужно не только на таблицу, но и на само поле
                 раскладки: роль, которой запрещено менять статус,
                 не должна таскать карточки между колонками — так же
                 считает и старая админка (canDragBoardItems). */
              {...(can.update && boardField.editable
                ? {
                    onMove: (guid: string, values: Record<string, unknown>) =>
                      update.mutate({ guid, values }),
                  }
                : {})}
              /* Правка поля прямо в карточке — тот же редактор и та же
                 мутация, что у ячейки таблицы. */
              {...(can.update
                ? {
                    onEdit: (guid: string, slug: string, value: unknown) =>
                      update.mutate({ guid, values: { [slug]: value } }),
                    onSettings: (field: Field, anchor: DOMRect) =>
                      setFieldPanel({ field, anchor }),
                  }
                : {})}
              /* Новая запись сразу в колонку: поле раскладки уже
                 заполнено, остальное — в карточке. */
              {...(can.write
                ? {
                    onAddCard: (columnId: string) => {
                      /* Адрес из настроек важнее черновика: админ задал
                         свою страницу создания — значит, заводят там. */
                      if (openCreateUrl(view)) return;

                      startDraft({
                        ...blankItem(drawerColumns),
                        [boardField.slug]: groupValue(boardField, columnId),
                      });
                    },
                  }
                : {})}
              {...(hasMore ? { onEndReached: loadMore } : {})}
            />
          ) : calendarView && calendarFrom ? (
            <CalendarView
              tableSlug={view.tableSlug}
              columns={columns}
              rows={rows.rows}
              fromField={calendarFrom}
              toField={calendarTo}
              statusField={calendarStatus}
              relations={schema.relations}
              period={period}
              cursor={calendar.cursor}
              rangeFrom={calendar.from}
              rangeTo={calendar.to}
              disabledDays={disabledDays}
              locale={i18n.language}
              language={language}
              hasMore={hasMore || loadingMore}
              /* Период и день — в адрес: это отбор строк, а ссылка
                 на неделю обязана открывать ту же неделю. */
              onPeriod={(next) => setSearch({ period: next })}
              onCursor={(next) => setSearch({ day: dayKey(next) })}
              /* Лента растёт от прокрутки: новые недели приезжают тем же
                 запросом строк — у них шире диапазон, и только. */
              onLoadPast={() => setLoaded({ ...span, past: span.past + 1 })}
              onLoadFuture={() => setLoaded({ ...span, future: span.future + 1 })}
              onOpenRow={openRow}
              {...(hasMore ? { onLoadMore: loadMore } : {})}
              /* Перенос и растягивание — обычная правка строки: те же
                 поля дат, тот же PUT, что у ячейки таблицы.

                 Право нужно и на само поле начала: роль, которой
                 запрещено менять дату, не должна двигать события. */
              {...(can.update && calendarFrom.editable
                ? {
                    onMove: (guid: string, values: Record<string, unknown>) =>
                      update.mutate({ guid, values }),
                  }
                : {})}
              /* Щелчок по пустой клетке — новая запись с проставленными
                 датами; остальное заполняется в карточке. */
              {...(can.write
                ? {
                    onCreate: (values: Record<string, unknown>) => {
                      if (openCreateUrl(view)) return;

                      startDraft({ ...blankItem(drawerColumns), ...values });
                    },
                  }
                : {})}
            />
          ) : pivotView ? (
            <PivotView
              columns={columns}
              rows={rows.rows}
              setup={pivotSetup}
              language={language}
              /* Приехало всё, что есть: тогда и счётчик говорит «по всем». */
              loaded={!hasMore && !loadingMore}
              onSetup={(next) =>
                setSearch({
                  ...(next.rowSlugs === undefined
                    ? {}
                    : { pivotRow: next.rowSlugs.join(",") || undefined }),
                  ...(next.colSlug === undefined ? {} : { pivotCol: next.colSlug || undefined }),
                  ...(next.valueSlug === undefined
                    ? {}
                    : { pivotValue: next.valueSlug || undefined }),
                  ...(next.aggregation === undefined ? {} : { pivotAgg: next.aggregation }),
                  ...(next.sort === undefined ? {} : { pivotSort: formatPivotSort(next.sort) }),
                  ...(next.skipEmpty === undefined
                    ? {}
                    : { pivotSkipEmpty: next.skipEmpty || undefined }),
                })
              }
            />
          ) : timelineView && calendarFrom ? (
            <Timeline
              tableSlug={view.tableSlug}
              columns={columns}
              rows={rows.rows}
              undated={undated}
              fromField={calendarFrom}
              toField={calendarTo}
              statusField={calendarStatus}
              /* Та же настройка, что группирует таблицу: у таймлайна
                 из неё получаются свёртываемые разделы списка слева. */
              groups={groupColumns}
              relations={schema.relations}
              scale={scale}
              cursor={calendar.cursor}
              rangeFrom={calendar.from}
              rangeTo={calendar.to}
              locale={i18n.language}
              language={language}
              hasMore={hasMore || loadingMore}
              /* Масштаб и день — в адрес: день задаёт отбор строк,
                 а ссылка на март обязана открывать март. */
              onScale={(next) => setSearch({ period: next })}
              onCursor={(next) => setSearch({ day: dayKey(next) })}
              /* Ось растёт от прокрутки: новые дни приезжают тем же
                 запросом строк — у него шире диапазон, и только. */
              onLoadPast={() => setLoaded({ ...span, past: span.past + 1 })}
              onLoadFuture={() => setLoaded({ ...span, future: span.future + 1 })}
              onOpenRow={openRow}
              {...(hasMore ? { onLoadMore: loadMore } : {})}
              /* Перенос, растягивание и постановка записи на ось —
                 обычная правка строки: те же поля дат, тот же PUT.

                 Право нужно и на само поле начала: роль, которой
                 запрещено менять дату, не должна двигать полосы. */
              {...(can.update && calendarFrom.editable
                ? {
                    onMove: (guid: string, values: Record<string, unknown>) =>
                      update.mutate({ guid, values }),
                  }
                : {})}
              /* Протяжка по пустой строке — новая запись с проставленными
                 датами; остальное заполняется в карточке. */
              {...(can.write
                ? {
                    onCreate: (values: Record<string, unknown>) => {
                      if (openCreateUrl(view)) return;

                      startDraft({ ...blankItem(drawerColumns), ...values });
                    },
                  }
                : {})}
            />
          ) : (
            <DataGrid
              tableSlug={view.tableSlug}
              columns={columns}
              pinned={pinned}
              /* Строки догружаются прокруткой: обработчик отдаётся,
                 только пока есть что грузить. */
              {...(infinite && hasMore ? { onEndReached: loadMore } : {})}
              /* Ширины колонок — настройка человека, не view: у соседа
                 другой монитор. Живут в localStorage, по слагу таблицы. */
              widths={view ? columnWidths[view.tableSlug] : undefined}
              {...(view
                ? { onWidth: (fieldId: string, width: number) => setColumnWidth(view.tableSlug, fieldId, width) }
                : {})}
              rows={rows.rows}
              groups={groupColumns}
              /* Номера строк продолжают счёт страниц: на второй по 20 — с 21. */
              startIndex={infinite ? 0 : (search.page - 1) * limit}
              {...(can.delete ? { onDeleteRow: setDeletingRow } : {})}
              relations={schema.relations}
              locale={i18n.language}
              language={language}
              selected={selected}
              onSelect={setSelected}
              sorts={sorts}
              // Из меню приходит направление, по клику в заголовок —
              // нет: там перебор вверх/вниз/никак.
              onSort={(field, direction) =>
                setSearch(
                  {
                    sort: formatSorts(
                      direction ? [{ field, direction }] : nextSorts(sorts, field),
                    ),
                    page: 1,
                  },
                  true,
                )
              }
              /* Без права на правку редактор не открывается вовсе:
                 таблица понимает отсутствие onEdit как «только чтение».
                 Новая строка при этом заполняется — это создание, и право
                 у него своё. */
              {...(can.update
                ? {
                    onEdit: (guid: string, slug: string, value: unknown) =>
                      update.mutate({ guid, values: { [slug]: value } }),
                  }
                : {})}
              /*
               * Строка заводится прямо в подвале таблицы — но только
               * с правом на запись: без него строки подвала нет вовсе.
               *
               * Уведомление обязательно: список отсортирован и отфильтрован,
               * и новая строка нередко уезжает на другую страницу — без
               * него создание выглядит как «ничего не произошло».
               */
              {...(can.write
                ? {
                    onCreate: (values: Item, done: () => void) =>
                      create.mutate(values, {
                        onSuccess: () => {
                          done();
                          toast.success(t("table.rowCreated"));
                        },
                      }),
                    creating: create.isPending,
                  }
                : {})}
              /*
               * «Новая запись» ведёт на свою форму проекта, когда админ
               * задал её адрес (attributes.url_object). Иначе строка
               * заводится на месте, в таблице.
               */
              {...(hasUrl(view.objectUrl)
                ? { onAddRow: () => openCreateUrl(view) }
                : {})}
              onOpenRow={openRow}
              onAddField={(anchor) => setFieldPanel({ field: null, anchor })}
              columnActions={{
                ...columnActions,
                filter: (field) => {
                  const kind = filterKind(field);
                  if (kind) applyFilters({ ...filters, [field.slug]: emptyFilter(kind) });
                },
              }}
            />
          )}

          {/* Подвал считает и листает загруженное — на отказе считать
              нечего, и «0 из 0» под сообщением об ошибке только сбивает.

              У доски подвала нет вовсе: страницами её не листают, а
              размер порции у неё свой и не настраивается. Сколько
              карточек в колонке — написано в её шапке. */}
          {!rowsError && !boardView && !dateView && !pivotView && (
          <GridFooter
            /* Со страницами подвал листает, с прокруткой — считает. */
            {...(infinite ? {} : { page: search.page, onPage: (next: number) => setSearch({ page: next }) })}
            shown={rows.rows.length}
            limit={limit}
            total={rows.count}
            loadingMore={loadingMore}
            selectedCount={selected.size}
            deleting={remove.isPending}
            /* Размер порции — он же настройка view. Смена возвращает
               на первую страницу: строка, которая была на седьмой
               по 25, на седьмой по 200 не лежит. */
            onLimit={(next) => {
              setTableLimit(view.tableSlug, next);
              setSearch({ limit: next, page: 1 });
            }}
            /* Удаление — отдельное право роли: без него кнопки нет.
               Выделение при этом остаётся: над отмеченными строками
               запускают действия. */
            {...(can.delete ? { onDeleteSelected: () => setConfirming(true) } : {})}
          />
          )}
        </>
      )}

      {search.item && view && (
        <ItemDrawer
          key={search.item}
          tableSlug={view.tableSlug}
          columns={drawerFields}
          row={drawerRow}
          // Карточка открыта по ссылке: строки нет ни на странице, ни
          // ещё в кэше — пока она едет, «записи не существует» неправда.
          loading={fetched.isLoading}
          error={fetched.error}
          relations={schema.relations}
          locale={i18n.language}
          language={language}
          languages={languages}
          onLanguage={setLanguage}
          /*
           * Действия над открытой строкой: те же, что над выделением
           * в таблице, только строка одна — та, которую видно.
           */
          actions={
            <>
              {/* Печатная форма записи: шаблон .docx таблицы, заполненный
                  значениями открытой строки. Шаблонов нет — кнопки нет. */}
              {drawerRow && <PrintButton tableSlug={view.tableSlug} row={drawerRow} />}

              <TableActions
                tableSlug={view.tableSlug}
                language={language}
                languages={languages}
                selected={[search.item]}
                canEdit={can.settings}
              />
            </>
          }
          /* Печатная форма записи, если админ задал её адрес. */
          {...(view.pdfUrl && drawerRow
            ? { onPdf: () => openUrl(fillTemplate(view.pdfUrl, drawerRow)) }
            : {})}
          sections={drawerLayout.sections}
          heading={drawerLayout.heading}
          trail={drawerTrail}
          tabs={relationTabs}
          /* Вкладку карточки заводит тот же, кто правит настройки view:
             вкладка и есть view — со своими колонками, отбором и именем.
             Имя и удаление — в её панели «⋯», как у таблицы. */
          {...(can.settings
            ? {
                addableRelations,
                tabTypes,
                onAddTab: (relationId: string, label: string, type: string) => {
                  const relation = schema.relations.find((item) => item.id === relationId);
                  if (!relation) return;

                  createView.mutate(
                    {
                      name: label,
                      language,
                      type,
                      relation: { id: relation.id, tableSlug: relation.toSlug },
                    },
                    { onSuccess: (created) => created?.id && setSearch({ tab: created.id }) },
                  );
                },
              }
            : {})}
          tab={search.tab ?? ""}
          onTab={(id) => setSearch({ tab: id || undefined })}
          tabContent={
            relationTab && (
              <RelationView
                key={relationTab.id}
                tab={relationTab}
                parentGuid={search.item}
                /* Вкладке обратного направления нужен не наш guid,
                   а значение нашей колонки-ссылки. */
                {...(relationTab.direction === "outgoing" && drawerRow
                  ? { parentValue: String(drawerRow[relationTab.fieldSlug] ?? "") }
                  : {})}
                menuId={menuId}
                locale={i18n.language}
                language={language}
                languages={languages}
                onLanguage={setLanguage}
                /* Крошка записи возвращает к самой карточке: вкладка
                   тогда закрывается, а вместе с ней и раскрытая в ней
                   связанная строка — она живёт внутри вкладки. */
                trail={[
                  ...drawerTrail,
                  {
                    label:
                      itemTitle(drawerRow, drawerLayout.heading) || t("drawer.noHeading"),
                    onClick: () => setSearch({ tab: undefined }),
                  },
                ]}
                saving={updateView.isPending}
                /* Настройки вкладки — те же, что у таблицы, и уезжают
                   тем же PUT view: у вкладки своя строка в базе. */
                {...(can.settings
                  ? {
                      settings: {
                        onRename: (name: string, nameLanguage: string) =>
                          updateView.mutate({
                            view: relationTab.view,
                            name,
                            language: nameLanguage,
                          }),
                        onType: (type: string) =>
                          updateView.mutate({ view: relationTab.view, type }),
                        /* Поле колонок доски. У таблицы та же настройка
                           разбивает её вкладками — колонка view одна. */
                        onTabGroup: (tabGroup: string) =>
                          updateView.mutate({ view: relationTab.view, tabGroup }),
                        /* Поля дат календаря вкладки — той же ручкой:
                           вкладка и есть view, со своей строкой в базе. */
                        onDateFrom: (dateFrom: string) =>
                          updateView.mutate({ view: relationTab.view, dateFrom }),
                        onDateTo: (dateTo: string) =>
                          updateView.mutate({ view: relationTab.view, dateTo }),
                        onColumns: (columns: string[]) =>
                          updateView.mutate({ view: relationTab.view, columns }),
                        onFixedColumns: (fixedColumns: string[]) =>
                          updateView.mutate({ view: relationTab.view, fixedColumns }),
                        onQuickFilters: (quickFilters: Field[]) =>
                          updateView.mutate({ view: relationTab.view, quickFilters }),
                        onDefaultFilters: (defaultFilters: Record<string, unknown>) =>
                          updateView.mutate({ view: relationTab.view, defaultFilters }),
                        onRemove: () => {
                          deleteView.mutate(relationTab.view);
                          setSearch({ tab: undefined });
                        },
                      },
                    }
                  : {})}
              />
            )
          }
          {...(can.update
            ? {
                onEdit: (guid: string, slug: string, value: unknown) =>
                  update.mutate({ guid, values: { [slug]: value } }),
              }
            : {})}
          onSettings={(field, anchor) => setFieldPanel({ field, anchor })}
          onReorder={drawerLayout.reorder}
          /* Секции правит тот же, кто двигает поля: это одна и та же
             раскладка и одно и то же право. */
          onAddSection={drawerLayout.addSection}
          onRenameSection={drawerLayout.renameSection}
          onRemoveSection={drawerLayout.removeSection}
          // Заголовок карточки — настройка раскладки: её правит тот же,
          // кто правит настройки view.
          {...(can.settings ? { onHeading: drawerLayout.setHeading } : {})}
          onClose={() => setSearch({ item: undefined, tab: undefined })}
        />
      )}

      {/*
        Новая запись в карточке: те же поля и тот же порядок, что
        у открытой строки, — но пустые, а правки копятся в черновике
        и уезжают одним запросом по кнопке.
      */}
      {draft && view && (
        <ItemDrawer
          tableSlug={view.tableSlug}
          columns={drawerFields}
          row={draft}
          relations={schema.relations}
          locale={i18n.language}
          language={language}
          languages={languages}
          onLanguage={setLanguage}
          sections={drawerLayout.sections}
          heading=""
          titlePlaceholder={t("table.addRow")}
          /* Тот же путь, что у открытой строки: без него шапка черновика
             — пустая полоса с двумя значками. Крошка меню закрывает
             черновик, потому что возвращает она туда же, в таблицу. */
          trail={[{ label: menu?.label ?? t("menu.title"), onClick: () => setDraft(null) }]}
          onEdit={(_guid, slug, value) =>
            setDraft((current) => (current ? { ...current, [slug]: value } : current))
          }
          /*
           * Связь черновика не уезжает запросом — строки в базе ещё нет.
           * Выбранная запись ложится рядом со ссылкой (`<слаг>_data`):
           * из неё ячейка берёт подпись, а наружу её отбрасывает
           * useCreateItem.
           */
          onLink={(slug, item) =>
            setDraft((current) =>
              current
                ? { ...current, [slug]: item?.guid ?? null, [relationDataKey(slug)]: item }
                : current,
            )
          }
          footer={
            <>
              {showErrors && draftErrors.size > 0 && (
                <span className="mr-auto text-xs text-danger">
                  {t("table.fillRequired", { count: draftErrors.size })}
                </span>
              )}

              <button
                type="button"
                onClick={() => setDraft(null)}
                className="h-8 rounded-md px-3 text-sm text-fg-muted transition-colors hover:bg-surface-hover"
              >
                {t("action.cancel")}
              </button>

              <button
                type="button"
                disabled={create.isPending}
                onClick={() => {
                  /*
                   * Проверка перед отправкой: колонка с NOT NULL ответит
                   * пятисоткой с текстом драйвера, а регулярное выражение
                   * бэкенд не смотрит вовсе.
                   */
                  if (draftErrors.size) {
                    setShowErrors(true);
                    return;
                  }

                  create.mutate(draft, {
                    onSuccess: () => {
                      setDraft(null);
                      toast.success(t("table.rowCreated"));
                    },
                  });
                }}
                className="h-8 rounded-md bg-accent-solid px-3 text-sm font-medium text-accent-fg transition-opacity hover:opacity-90 disabled:opacity-50"
              >
                {t("action.create")}
              </button>
            </>
          }
          onClose={() => setDraft(null)}
        />
      )}

      {importing && view && (
        <ExcelImportDialog
          tableSlug={view.tableSlug}
          // Все поля таблицы, а не колонки view: столбец файла можно
          // положить и в скрытую колонку — данные от этого не исчезнут.
          fields={tableFields}
          language={language}
          onClose={() => setImporting(false)}
        />
      )}

      {fieldPanel && (
        <FieldEditor
          key={fieldPanel.field?.id ?? "new"}
          field={fieldPanel.field ?? undefined}
          // Слаг новой колонки не должен совпасть с существующей: база
          // ответит 500, и это единственное, что увидит человек.
          fields={schema.fields}
          relations={schema.relations}
          language={language}
          languages={languages}
          anchor={fieldPanel.anchor}
          icon={fieldIcon}
          onClose={() => setFieldPanel(null)}
          onSubmit={saveField}
          onSubmitRelation={(draft) => createRelation.mutate({ draft, language })}
          onEditRelation={(relation, draft) => {
            /*
             * Целевая таблица не правится — связь пересоздаётся. Имя
             * колонки-ссылки выведено из неё при создании, а PUT колонку
             * не трогает: связь начала бы утверждать одно, а хранить
             * другое. Поэтому сносим и заводим заново — цену формa
             * называет до нажатия.
             *
             * Строго по очереди: пока старая колонка не удалена, новая
             * заводится в ту же таблицу, и порядок здесь — не вкусовщина.
             */
            if (draft.toSlug === relation.toSlug) {
              updateRelation.mutate({ relation, draft, language });
              return;
            }

            deleteRelation.mutate(relation, {
              onSuccess: () => createRelation.mutate({ draft, language }),
            });
          }}
          onDelete={setDeletingField}
        />
      )}

      {deletingView && (
        <ConfirmDialog
          title={t("view.deleteTitle")}
          description={t("view.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={deleteView.isPending}
          onClose={() => setDeletingView(null)}
          onConfirm={() =>
            deleteView.mutate(deletingView, {
              onSuccess: () => {
                setDeletingView(null);
                // Открытый view исчез — возвращаемся к первой вкладке,
                // а не оставляем в адресе ссылку на удалённый.
                if (deletingView.id === view?.id) openView();
              },
            })
          }
        />
      )}

      {/*
        Удаление колонки. У связи это другая сущность и другая ручка:
        уходит связь целиком, а вместе с ней колонка-ссылка и её значения.
        Сказать об этом надо ДО подтверждения, а не после, — поэтому
        и текст, и запрос разные.
      */}
      {deletingField &&
        (() => {
          const relation = deletingField.relationId
            ? schema.relations.find((item) => item.id === deletingField.relationId)
            : undefined;

          /*
           * У мультиязычного поля уходит вся группа: в таблице оно одна
           * колонка, а в схеме — по колонке на язык. Удалить только
           * показанный вариант значит оставить `naming_cyr` невидимым
           * мусором, который всплывёт при переключении языка.
           */
          const base = baseSlug(deletingField, codes);
          const variants =
            (base === null ? undefined : languageGroups(schema.fields, codes).get(base)) ??
            [deletingField];

          const done = () => setDeletingField(null);

          return (
            <ConfirmDialog
              title={t("fieldForm.deleteTitle", { label: deletingField.label })}
              description={
                relation
                  ? t("fieldForm.deleteRelationDescription")
                  : variants.length > 1
                    ? t("fieldForm.deleteLanguagesDescription", { count: variants.length })
                    : t("fieldForm.deleteDescription")
              }
              confirmLabel={t("action.delete")}
              busy={relation ? deleteRelation.isPending : deleteField.isPending}
              onClose={done}
              onConfirm={() => {
                if (relation) {
                  deleteRelation.mutate(relation, { onSuccess: done });
                  return;
                }

                // Языковые колонки удаляются по одной: общей ручки нет.
                void Promise.all(variants.map((field) => deleteField.mutateAsync(field)))
                  .then(done)
                  .catch(() => done());
              }}
            />
          );
        })()}

      {/* Открыть строку, не сохранив черновик новой записи. Спрашиваем,
          потому что щелчок по строке — это «посмотреть», а не «закрыть»:
          введённое пропало бы без единого следа. */}
      {pendingRow && (
        <ConfirmDialog
          title={t("table.draftOpenTitle")}
          description={t("table.draftOpenDescription")}
          confirmLabel={t("table.draftDiscard")}
          busy={false}
          onClose={() => setPendingRow(null)}
          onConfirm={() => {
            setDraft(null);
            setPendingRow(null);
            setSearch({ item: pendingRow, tab: undefined });
          }}
        />
      )}

      {/* Удаление одной строки — урной у правого края. Свой диалог:
          выделение человека здесь ни при чём и не трогается. */}
      {deletingRow && (
        <ConfirmDialog
          title={t("table.deleteTitle")}
          description={t("table.deleteDescription", { count: 1 })}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setDeletingRow(null)}
          onConfirm={() =>
            remove.mutate([deletingRow], {
              onSuccess: () => {
                // Удалённая не должна остаться отмеченной невидимо.
                setSelected((prev) => {
                  const next = new Set(prev);
                  next.delete(deletingRow);
                  return next;
                });
                setDeletingRow(null);
              },
            })
          }
        />
      )}

      {confirming && (
        <ConfirmDialog
          title={t("table.deleteTitle")}
          description={t("table.deleteDescription", { count: selected.size })}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setConfirming(false)}
          onConfirm={() =>
            remove.mutate([...selected], {
              onSuccess: () => {
                setSelected(new Set());
                setConfirming(false);
              },
            })
          }
        />
      )}
    </div>
  );
}


function Notice({
  text,
  actions = [],
}: {
  text: string;
  /** Кнопки под текстом. Панель поля всплывает под своей — отсюда якорь. */
  actions?: { label: string; onClick: (anchor: DOMRect) => void }[];
}) {
  return (
    <div className="grid flex-1 place-items-center p-8 text-center">
      <div className="flex max-w-sm flex-col items-center gap-3">
        <p className="text-sm text-fg-muted">{text}</p>

        {actions.length > 0 && (
          <div className="flex items-center gap-2">
            {actions.map((action) => (
              <button
                key={action.label}
                type="button"
                onClick={(event) => action.onClick(event.currentTarget.getBoundingClientRect())}
                className="h-8 rounded-md border border-border-strong px-3 text-sm text-fg transition-colors hover:bg-surface-hover"
              >
                {action.label}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
