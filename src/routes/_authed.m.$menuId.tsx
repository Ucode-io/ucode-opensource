import { useEffect, useMemo, useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import {
  DataGrid,
  FilterBar,
  GridSkeleton,
  ItemDrawer,
  GridFooter,
  MAX_LIMIT,
  MIN_LIMIT,
  TableToolbar,
  emptyFilter,
  filterKind,
  formatSorts,
  nextSorts,
  parseSorts,
  fieldIcon,
  orderColumns,
  useCreateItem,
  useDeleteItems,
  useDrawerLayout,
  useItem,
  useItems,
  useUpdateItem,
  activeFilterCount,
  filtersSchema,
  fromConditions,
  parseFilters,
  toConditions,
  type Filters,
} from "@/features/item";
import { useTablePermission } from "@/features/auth";
import { IMPLEMENTED_TYPES, SidebarToggleButton, useMenu } from "@/features/sidebar";
import {
  FieldEditor,
  TableActions,
  collapseLanguages,
  baseSlug,
  languageGroups,
  toDraft,
  useCreateField,
  useCreateRelation,
  useDeleteField,
  useDeleteRelation,
  useTableSchema,
  useUpdateField,
  useUpdateRelation,
  type Field,
  type FieldDraft,
} from "@/features/table";
import {
  ExcelImportDialog,
  IMPLEMENTED_VIEW_TYPES,
  RelationView,
  ViewCreateButton,
  ViewOptions,
  ViewTabs,
  columnKey,
  pickView,
  resolveColumns,
  tabViews,
  useCreateView,
  useDeleteView,
  useExportExcel,
  useMenuViews,
  useUpdateView,
  fillTemplate,
  fillUrl,
  hasUrl,
  isExternal,
  type View,
} from "@/features/view";
import { useDataLanguages } from "@/features/workspace";
import { toast } from "@/shared/lib/toast";
import { useUi } from "@/shared/lib/ui-store";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";

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

const searchSchema = z.object({
  view: z.string().optional(),
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
  /*
   * Фильтр — это условие и его аргументы. Аргументы всегда списком
   * строк, каким бы ни было условие: одна форма и в адресе, и в схеме,
   * и в проверке. Что значит каждая позиция — знает features/item.
   */
  filters: filtersSchema.optional().catch(undefined),
  search: z.string().optional().catch(undefined),
});

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

  const { views } = useMenuViews(menuId);
  const tabs = useMemo(() => tabViews(views), [views]);
  const view = pickView(views, search.view);

  /*
   * Права роли на эту таблицу. Решают, что рисовать: настоящую проверку
   * делает сервер, и без гейта роль без прав всё равно получала бы 403 —
   * но уже после того, как переименовала вкладку у себя на экране.
   */
  const can = useTablePermission(view?.tableSlug);

  // Колонки view — не только «что показать», но и «что грузить»:
  // настройки связей за пределами этого списка никому не нужны.
  const { schema, isLoading: schemaLoading } = useTableSchema(view?.tableSlug, view?.columnIds);
  /*
   * Колонки view, а затем — сведённые языковые. Мультиязычное поле
   * лежит в схеме НЕСКОЛЬКИМИ колонками (`title_en`, `title_cyr`),
   * и без сведения таблица показывает их подряд с одинаковой подписью:
   * какая из них узбекская, видно только по значению.
   *
   * Сводится после выбора колонок view: скрытая колонка остаётся
   * скрытой на всех языках.
   */
  const columns = useMemo(
    () => collapseLanguages(resolveColumns(view, schema.fields), codes, language),
    [view, schema.fields, codes, language],
  );

  /*
   * Порядок полей в drawer — свой, из раскладки пункта меню, и с колонками
   * таблицы не связан: перестановка в карточке не двигает колонки, а
   * перестановка колонок — поля карточки.
   */
  const drawerLayout = useDrawerLayout({ tableSlug: view?.tableSlug ?? "", menuId, language });
  /*
   * Поля карточки: колонки view в порядке раскладки, минус спрятанные
   * из карточки (`field_hide_layout`). Скрытие из карточки — настройка
   * раскладки, а не поля: колонкой таблицы то же поле остаётся.
   */
  const drawerColumns = useMemo(
    () =>
      orderColumns(columns, drawerLayout.order).filter(
        (field) => !drawerLayout.hidden.has(field.slug),
      ),
    [columns, drawerLayout.order, drawerLayout.hidden],
  );

  const supportedView = view ? IMPLEMENTED_VIEW_TYPES.has(view.type) : false;
  /*
   * Размер страницы: из адреса, иначе последний выбранный для этой
   * таблицы, иначе настройка view. Значение из localStorage проверяется
   * — испорченное руками «0» оставило бы таблицу пустой навсегда.
   */
  const { tableLimits, setTableLimit, tableFilters, setTableFilters } = useUi();
  const rememberedLimit = view ? tableLimits[view.tableSlug] : undefined;
  const limit =
    search.limit ??
    (rememberedLimit && rememberedLimit >= MIN_LIMIT && rememberedLimit <= MAX_LIMIT
      ? rememberedLimit
      : undefined) ??
    view?.defaultLimit ??
    FALLBACK_LIMIT;
  const sorts = parseSorts(search.sort);

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

  /** Отбор, которым управляет человек: он в адресе, он же в подшапке. */
  const filters: Filters = useMemo(
    () =>
      search.filters ??
      rememberedFilters ??
      seedFilters(view?.quickFilterIds ?? [], schema.fields),
    [search.filters, rememberedFilters, view?.quickFilterIds, schema.fields],
  );

  /** То, что действительно уходит в запрос. Область видимости — сверху. */
  const effectiveFilters: Filters = useMemo(
    () => ({ ...filters, ...defaultFilters }),
    [filters, defaultFilters],
  );

  const lockedSlugs = useMemo(() => new Set(Object.keys(defaultFilters)), [defaultFilters]);

  /** Закреплённые колонки — по id поля: DataGrid знает только их. */
  const pinned = useMemo(
    () => new Set(columns.filter((field) => isPinned(view, field)).map((field) => field.id)),
    [columns, view],
  );

  const createView = useCreateView({ menuId, tableSlug: view?.tableSlug, order: tabs.length + 1 });
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
      sort: undefined,
      filters: undefined,
      search: undefined,
      limit: undefined,
      filtersOpen: undefined,
    });

  /** Правка отбора: в адрес — чтобы переслать, в память — чтобы вернуться. */
  const applyFilters = (next: Filters) => {
    if (filtersKey) setTableFilters(filtersKey, next);
    setSearch({ filters: next, page: 1 }, true);
  };

  const {
    page: rows,
    isLoading: rowsLoading,
    isFetching,
  } = useItems(supportedView ? view?.tableSlug : undefined, {
    limit,
    page: search.page,
    sorts,
    filters: effectiveFilters,
    search: search.search,
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
  const relationTab = drawerLayout.relationTabs.find((item) => item.id === search.tab);

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
  const [confirming, setConfirming] = useState(false);
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

  const supported = menu ? IMPLEMENTED_TYPES.has(menu.type) : true;

  return (
    <div className="flex h-full flex-col">
      <header className="flex h-header shrink-0 items-center gap-2 border-b border-border px-4">
        <SidebarToggleButton />
        <span className="text-sm font-medium">{menu?.label ?? t("menu.title")}</span>
        {/* Число без слова: «16 записей» требует согласования по падежу
            в русском и узбекском, а множественные формы i18next стоят
            трёх ключей на язык ради одного счётчика. */}
        {supportedView && <span className="text-xs text-fg-muted">· {rows.count}</span>}
        {isFetching && !rowsLoading && (
          <span className="text-xs text-fg-subtle">{t("common.loading")}</span>
        )}
      </header>

      {/* Полоса вкладок живёт, пока у пункта меню есть хоть один view, —
          в том числе когда открыт view неподдержанного типа. Иначе доска
          прячет вкладки вместе с собой, и вернуться к таблице можно только
          кнопкой «назад» в браузере. */}
      {supported && tabs.length > 0 && (
        <div className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-3">
          <ViewTabs
            views={tabs}
            activeId={view?.id ?? ""}
            language={language}
            onSelect={(next) => openView(next.id)}
          />

          {can.viewCreate && (
            <ViewCreateButton
              busy={createView.isPending}
              // Новая вкладка сразу открывается: её создали, чтобы в неё
              // смотреть. Список к этому моменту уже перезапрошен — см.
              // useCreateView, иначе вкладки дёрнулись бы на первую и обратно.
              onCreate={(name) =>
                createView.mutate(
                  { name, language },
                  { onSuccess: (created) => created?.id && openView(created.id) },
                )
              }
            />
          )}

          {view && (
            <div className="ml-auto flex shrink-0 items-center gap-0.5">
              {/* Поиск, отбор и сортировка — про таблицу: у нарисованного
                  заглушкой view искать нечего. */}
              {supportedView && (
                <TableToolbar
                  tableSlug={view.tableSlug}
                  columns={columns}
                  language={language}
                  languages={languages}
                  onLanguage={setLanguage}
                  sorts={sorts}
                  onSorts={(next) => setSearch({ sort: formatSorts(next), page: 1 }, true)}
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
                fields={schema.fields}
                language={language}
                languages={languages}
                defaultFilters={defaultFilters}
                can={can}
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
                  // Единственную вкладку удалять нечем и незачем: без view
                  // у пункта меню не остаётся ни экрана, ни кнопки «создать».
                  ...(tabs.length > 1 ? { onDelete: () => setDeletingView(view) } : {}),
                }}
              />
            </div>
          )}
        </div>
      )}

      {!supported ? (
        <Notice text={t("menu.notImplemented")} />
      ) : !view ? (
        <Notice text={t("table.noView")} />
      ) : !supportedView ? (
        <Notice text={t("table.notImplementedView")} />
      ) : schemaLoading ? (
        <GridSkeleton />
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
          text={schema.fields.length ? t("table.noColumns") : t("table.noFieldsYet")}
          actions={[
            // Поля есть, но ни одно не показано — их показывают, а не заводят.
            ...(schema.fields.length && can.columns
              ? [
                  {
                    label: t("view.showAll"),
                    onClick: () =>
                      updateView.mutate({ view, columns: schema.fields.map(columnKey) }),
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
      ) : (
        <>
          {filtersVisible && (
            <FilterBar
              columns={columns}
              language={language}
              filters={filters}
              sorts={sorts}
              // Поля, закрытые отбором по умолчанию: свой фильтр по ним
              // всё равно перекрывается настройкой view, и чип соврал бы.
              locked={lockedSlugs}
              // Любая правка отбора возвращает на первую страницу: на
              // седьмой после сужения выборки обычно пусто.
              onFilters={applyFilters}
              onSorts={(next) => setSearch({ sort: formatSorts(next), page: 1 }, true)}
            />
          )}

          {rowsLoading ? (
            <GridSkeleton columns={columns.length} />
          ) : (
            <DataGrid
              tableSlug={view.tableSlug}
              columns={columns}
              pinned={pinned}
              rows={rows.rows}
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
              onEdit={(guid, slug, value) => update.mutate({ guid, slug, value })}
              /*
               * Уведомление обязательно: список отсортирован и отфильтрован,
               * и новая строка нередко уезжает на другую страницу — без
               * него создание выглядит как «ничего не произошло».
               */
              onCreate={(values, done) =>
                create.mutate(values, {
                  onSuccess: () => {
                    done();
                    toast.success(t("table.rowCreated"));
                  },
                })
              }
              creating={create.isPending}
              /*
               * «Новая запись» ведёт на свою форму проекта, когда админ
               * задал её адрес (attributes.url_object). Иначе строка
               * заводится на месте, в таблице.
               */
              {...(hasUrl(view.objectUrl)
                ? { onAddRow: () => openUrl(fillUrl(view.objectUrl, {})) }
                : {})}
              /*
               * Щелчок по строке открывает карточку — если админ не задал
               * своего адреса. Задал (attributes.navigate) — уводим туда:
               * у проекта своя страница заказа, и карточка ей не замена.
               *
               * Вкладка связи сбрасывается вместе со строкой: у другой
               * записи набор вкладок тот же, а открытая — уже не та.
               */
              onOpenRow={(guid) => {
                const row = rows.rows.find((item) => item.guid === guid);
                if (row && openRowUrl(view, row)) return;

                setSearch({ item: guid, tab: undefined });
              }}
              onAddField={(anchor) => setFieldPanel({ field: null, anchor })}
              columnActions={{
                // Переименование — единственная правка схемы, которую
                // делают на бегу: остальное открывает диалог.
                rename: (field, label) =>
                  updateField.mutate({
                    field,
                    draft: { ...toDraft(field, language), label },
                    language,
                  }),
                settings: (field, anchor) => setFieldPanel({ field, anchor }),
                filter: (field) => {
                  const kind = filterKind(field);
                  if (kind) applyFilters({ ...filters, [field.slug]: emptyFilter(kind) });
                },
                remove: setDeletingField,
              }}
            />
          )}

          <GridFooter
            page={search.page}
            limit={limit}
            total={rows.count}
            selectedCount={selected.size}
            deleting={remove.isPending}
            onPage={(next) => setSearch({ page: next })}
            // Смена размера страницы возвращает на первую: строка, которая
            // была на седьмой странице по 25, на седьмой по 200 не лежит.
            onLimit={(next) => {
              setTableLimit(view.tableSlug, next);
              setSearch({ limit: next, page: 1 });
            }}
            onDeleteSelected={() => setConfirming(true)}
          />
        </>
      )}

      {search.item && view && (
        <ItemDrawer
          key={search.item}
          tableSlug={view.tableSlug}
          columns={drawerColumns}
          row={drawerRow}
          relations={schema.relations}
          locale={i18n.language}
          language={language}
          languages={languages}
          onLanguage={setLanguage}
          /* Печатная форма записи, если админ задал её адрес. */
          {...(view.pdfUrl && drawerRow
            ? { onPdf: () => openUrl(fillTemplate(view.pdfUrl, drawerRow)) }
            : {})}
          sections={drawerLayout.sections}
          heading={drawerLayout.heading}
          tabs={drawerLayout.relationTabs}
          tab={search.tab ?? ""}
          onTab={(id) => setSearch({ tab: id || undefined })}
          tabContent={
            relationTab && (
              <RelationView
                key={relationTab.id}
                tab={relationTab}
                parentGuid={search.item}
                locale={i18n.language}
                language={language}
                canEdit={can.settings}
                // Колонки вкладки лежат в раскладке карточки, и правит
                // их тот же PUT, что и порядок полей.
                onColumns={(columnIds) => drawerLayout.setTabColumns(relationTab.id, columnIds)}
              />
            )
          }
          onEdit={(guid, slug, value) => update.mutate({ guid, slug, value })}
          onSettings={(field, anchor) => setFieldPanel({ field, anchor })}
          onReorder={drawerLayout.reorder}
          // Заголовок карточки — настройка раскладки: её правит тот же,
          // кто правит настройки view.
          {...(can.settings ? { onHeading: drawerLayout.setHeading } : {})}
          onClose={() => setSearch({ item: undefined, tab: undefined })}
        />
      )}

      {importing && view && (
        <ExcelImportDialog
          tableSlug={view.tableSlug}
          // Все поля таблицы, а не колонки view: столбец файла можно
          // положить и в скрытую колонку — данные от этого не исчезнут.
          fields={schema.fields}
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
          languages={codes}
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

/**
 * Адрес, заданный админу вместо карточки. Вернул true — переход состоялся,
 * и карточку открывать не нужно.
 */
function openRowUrl(view: View, row: Record<string, unknown>): boolean {
  if (!hasUrl(view.navigate)) return false;

  openUrl(fillUrl(view.navigate, row));
  return true;
}

/**
 * Переход по адресу из настроек.
 *
 * Чужой сайт открывается новой вкладкой, свой — заменяет страницу.
 * `noopener` обязателен: без него открытая страница получает доступ
 * к нашему window через opener.
 */
function openUrl(url: string) {
  if (!url) return;

  if (isExternal(url)) window.open(url, "_blank", "noopener,noreferrer");
  else window.location.assign(url);
}

/**
 * Подсказка админа → пустые чипы в подшапке. Поля ищутся по тем же двум
 * ключам, что и колонки: у связей во view лежит id связи, а не поля.
 */
function seedFilters(ids: string[], fields: Field[]): Filters {
  if (!ids.length) return {};

  const index = new Map<string, Field>();
  for (const field of fields) {
    index.set(field.id, field);
    if (field.relationId) index.set(field.relationId, field);
  }

  const seeded: Filters = {};
  for (const id of ids) {
    const field = index.get(id);
    const kind = field && filterKind(field);
    if (field && kind) seeded[field.slug] = emptyFilter(kind);
  }

  return seeded;
}

/**
 * Закреплена ли колонка. Ключей у поля-связи два — id поля и id связи, —
 * и в настройке лежит любой из них, как и в columns.
 */
function isPinned(view: View | undefined, field: Field): boolean {
  const fixed = view?.fixedColumnIds ?? [];
  return fixed.includes(field.id) || (field.relationId ? fixed.includes(field.relationId) : false);
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
