import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toCharts, toChartsAttribute, type ChartConfig } from "@/features/item";
import type { Field } from "@/features/table";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { errorMessage, reportError, toast } from "@/shared/lib/toast";
import type { View } from "../model/types";
import { toUrlTemplate, type UrlTemplate } from "../model/url-template";

/** Сырой view. Наружу не выходит. */
type ViewDto = {
  id?: string;
  type?: string;
  table_slug?: string;
  menu_id?: string;
  order?: number;
  is_relation_view?: boolean;
  /** Таблица вкладки связи и сама связь — см. View.relationTableSlug. */
  relation_table_slug?: string;
  relation_id?: string;
  /** Подпись таблицы из relation_table_slug: её подставляет запрос списка. */
  table_label?: string;
  /** Имя вкладки. Пустая строка встречается чаще, чем непустая. */
  name?: string;
  /** Строка, а не число: в настройках это свободное поле ввода. */
  default_limit?: string | number;
  columns?: string[];
  /** Поле раскладки вкладками. Колонка таблицы, а не ключ attributes. */
  group_fields?: string[];
  /** Поля дат календаря и таймлайна. Колонки таблицы, и здесь это слаги. */
  calendar_from_slug?: string;
  calendar_to_slug?: string;
  /** Поле цвета события. Колонка есть, ручки правки для неё нет. */
  status_field_slug?: string;
  /** Нерабочие дни: чужая таблица и колонки дня и времени в ней. */
  disable_dates?: { table_slug?: string; day_slug?: string } | null;
  attributes?: Record<string, unknown>;
};

type ViewsResponseDto = { views?: ViewDto[] };

/**
 * View'шки пункта меню. Именно меню, а не таблицы: /v2/views/{slug}
 * отдаёт все view таблицы разом, включая чужие пункты меню.
 */
export function useMenuViews(menuId: string) {
  const session = useSession();
  const envId = session.getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.views.byMenu(envId, menuId),
    queryFn: () => api.get<ViewsResponseDto>(`/v3/menus/${menuId}/views`),
    enabled: Boolean(menuId),
    // Настройки view меняет админ, а не пользователь при работе со строками.
    staleTime: 5 * 60_000,
    // Постоянная ссылка: со стрелкой на месте react-query гоняет select
    // на каждый рендер и отдаёт новый массив — а на нём висят вкладки
    // экрана и вкладки карточки.
    select: toViews,
  });

  return {
    views: query.data ?? [],
    isLoading: query.isLoading,
    /** Причина отказа словами. null — всё в порядке. */
    error: errorMessage(query.error, "table.loadFailed"),
    refetch: () => void query.refetch(),
  };
}

/**
 * Новый view пункта меню.
 *
 * Ручка — POST /v2/views/{table_slug}, хотя view принадлежит меню:
 * слаг в пути бэкенд использует только для журнала версий, а всё
 * настоящее читает из тела, включая menu_id. Соседняя POST
 * /v3/menus/{id}/views делает ровно то же самое, но отвечает дважды
 * (h.HandleResponse и в ветке успеха, и после неё), и второй JSON
 * приклеивается к первому — тело перестаёт разбираться, а вместе с ним
 * теряется id созданного view.
 *
 * Колонки не передаём. Их бэкенд подставляет сам: все поля таблицы плюс
 * все её связи (view.go, INSERT). Именно поэтому у нового view в columns
 * оказываются оба ключа поля-связи — id поля и id связи, — и колонки
 * нужно разворачивать через resolveColumns, а не по одному ключу.
 *
 * Тип — из тех, что мы рисуем (форма создания предлагает только их).
 * Вместе с ним приезжает ровно одна настройка — поля дат календаря:
 * без поля начала он не рисует ничего, поэтому их спрашивают вторым
 * шагом создания, как и в старой админке. У доски такого шага нет:
 * поле раскладки выбирают в настройках уже созданной, а колонку
 * порядка (`board_order`) бэкенд заводит сам, увидев тип BOARD
 * (storage/postgres/view.go:66).
 */
export function useCreateView({
  menuId,
  tableSlug,
  order,
}: {
  menuId: string;
  tableSlug: string | undefined;
  order: number;
}) {
  const queryClient = useQueryClient();
  const session = useSession();
  const envId = session.getEnvironmentId() ?? "";
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: ({ name, language, type, relation, dateFrom, dateTo }: NewView) =>
      api.post<ViewDto>(`/v2/views/${slug}`, {
        table_slug: slug,
        menu_id: menuId,
        type: type ?? "TABLE",
        order,
        // Имя пишется дважды: в колонку и в attributes на языке данных.
        // Старая админка читает только attributes, мы — сначала их же.
        name: name.trim(),
        attributes: { [`name_${language}`]: name.trim() },
        /*
         * Вкладка карточки — это view со связью, и ровно так её заводит
         * старая админка: `is_relation_view` плюс слаг ЧУЖОЙ таблицы
         * (useViewCreatePopupProps.jsx). `relation_id` она не шлёт, и
         * вкладка потом ищет свою связь по слагу — с двумя связями
         * на одну таблицу это лотерея. Мы шлём: колонка в базе есть,
         * и по ней связь находится однозначно.
         */
        is_relation_view: Boolean(relation),
        ...(relation
          ? { relation_table_slug: relation.tableSlug, relation_id: relation.id }
          : {}),
        /*
         * Даты календаря — колонками, а не ключами attributes: INSERT
         * читает `req.CalendarFromSlug` (view.go:138). Старая админка
         * при создании клала их только в attributes
         * (useViewCreatePopupProps.jsx), а при сохранении — только
         * в колонку; отсюда её же `view.calendar_from_slug ??
         * view.attributes.calendar_from_slug` в девяти местах.
         */
        ...(dateFrom === undefined ? {} : { calendar_from_slug: dateFrom }),
        ...(dateTo === undefined ? {} : { calendar_to_slug: dateTo }),
      }),

    onError: (error) => reportError(error, "common.createFailed"),
    /*
     * Список перезапрашивается до того, как вызывающий переключится на
     * новую вкладку: иначе её ещё нет в списке, pickView вернёт первую,
     * и вкладки на глазах дёрнутся туда и обратно.
     */
    onSuccess: async () => {
      toast.success(i18n.t("view.created"));
      await queryClient.invalidateQueries({ queryKey: keys.views.byMenu(envId, menuId) });
    },
  });
}

/**
 * Удаление view. Уходит вкладка и её настройки — колонки, фильтры,
 * порядок; строки таблицы остаются на месте.
 *
 * id обязателен, и это не формальность: в бэкенде условие удаления
 * выбирается по тому, что пришло (view.go, Delete), и запрос без id, но
 * со слагом таблицы сносит ВСЕ её view разом. Шлюз пропускает только
 * настоящий uuid, но полагаться на это в вызывающем коде нельзя.
 */
export function useDeleteView({ menuId, tableSlug }: { menuId: string; tableSlug: string | undefined }) {
  const queryClient = useQueryClient();
  const session = useSession();
  const envId = session.getEnvironmentId() ?? "";
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (view: View) => api.delete<unknown>(`/v2/views/${slug}/${view.id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("view.deleted"));
      await queryClient.invalidateQueries({ queryKey: keys.views.byMenu(envId, menuId) });
    },
  });
}

/**
 * Правка view: имя и набор колонок.
 *
 * Ответ не читается сознательно. Ручка отвечает дважды — h.HandleResponse
 * и в ветке успеха, и после неё, — поэтому в теле лежат два JSON подряд,
 * и разобрать его нельзя. Свежий view приезжает перезапросом списка.
 */
export function useUpdateView({
  menuId,
  tableSlug,
}: {
  menuId: string;
  tableSlug: string | undefined;
}) {
  const queryClient = useQueryClient();
  const session = useSession();
  const envId = session.getEnvironmentId() ?? "";
  const slug = tableSlug ?? "";

  const key = keys.views.byMenu(envId, menuId);

  return useMutation({
    mutationFn: (edit: ViewEdit) => api.put<unknown>(`/v2/views/${slug}`, toUpdateBody(edit)),
    /*
     * Правка кладётся в кэш сразу. Ответ ручки не читается (см. выше),
     * и без этого порядок колонок меняется только после ответа PUT
     * и перезапроса списка — поле, брошенное мышью, секунду стоит
     * на старом месте, будто бросок не засчитан.
     *
     * В кэше лежит сырой ответ, а тело PUT — это и есть сырой view
     * с правкой (toUpdateBody = {...raw, правка}). Поэтому подменяется
     * ровно им: второго места, где правка превращается в view, нет.
     */
    onMutate: (edit: ViewEdit) => {
      const previous = queryClient.getQueryData<ViewsResponseDto>(key);

      queryClient.setQueryData<ViewsResponseDto>(key, (data) =>
        data
          ? {
              ...data,
              views: (data.views ?? []).map((dto) =>
                dto.id === edit.view.id ? (toUpdateBody(edit) as ViewDto) : dto,
              ),
            }
          : data,
      );

      return previous;
    },
    // Откат: сервер правку не принял, а на экране она уже показана.
    onError: (error, _edit, previous) => {
      if (previous) queryClient.setQueryData(key, previous);
      reportError(error, "common.saveFailed");
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: key }),
  });
}

/** Что нужно новому view. `relation` — только у вкладки карточки. */
export type NewView = {
  name: string;
  /** Язык ДАННЫХ для имени. */
  language: string;
  /** Тип view. Не задан — TABLE: самый частый выбор и у экрана, и у вкладки. */
  type?: string;
  relation?: { id: string; tableSlug: string };
  /** Поля дат календаря: их выбирают вторым шагом создания. */
  dateFrom?: string;
  dateTo?: string;
};

export type ViewEdit = {
  view: View;
  /** Новое имя. Не задано — не трогаем. */
  name?: string;
  /** Язык ДАННЫХ для имени. Нужен вместе с name. */
  language?: string;
  /** Новый тип: таблица, доска, календарь. */
  type?: string;
  /** Новый список колонок целиком, в порядке показа. */
  columns?: string[];
  /**
   * Поля, предложенные чипами в подшапке. Целиком: в attributes лежат
   * не идентификаторы, а сами поля — так их пишет и читает старая
   * админка, и ломать формат ради краткости незачем.
   */
  quickFilters?: Field[];
  /**
   * Закреплённые колонки целиком. Ключ — id ПОЛЯ, не id связи, в отличие
   * от `columns`: старая админка и пишет, и читает эту настройку только
   * по `column.id`, и колонка-связь, закреплённая чужим ключом, у неё
   * не закрепится. Читаем мы по-прежнему оба ключа — данные с чужим
   * ключом уже лежат в проектах.
   */
  fixedColumns?: string[];
  /** Отбор по умолчанию: карта «слаг → условие», как её ждёт get-list. */
  defaultFilters?: Record<string, unknown>;
  /** Адрес, куда уводит щелчок по строке. */
  navigate?: UrlTemplate;
  /** Адрес, куда ведёт «новая запись». */
  objectUrl?: UrlTemplate;
  /** Адрес PDF записи. */
  pdfUrl?: string;
  /** Догружать строки прокруткой вместо номеров страниц. */
  infiniteScroll?: boolean;
  /** Поля группировки в порядке уровней. Пустой список — снять группировку. */
  groupBy?: string[];
  /** Поле раскладки вкладками. Пустая строка — убрать вкладки. */
  tabGroup?: string;
  /** Поле дорожек доски. Пустая строка — убрать дорожки. */
  subGroup?: string;
  /** Слаг поля начала события. Пустая строка — календарю нечего рисовать. */
  dateFrom?: string;
  /** Слаг поля конца события. Пустая строка — событие точкой в дне. */
  dateTo?: string;
  /**
   * Графики экрана CHART целиком, в порядке показа. Пустой список —
   * убрать все: это тоже правка, а не «не трогали».
   */
  charts?: ChartConfig[];
};

/**
 * Черновик поверх исходного ответа, как и у полей: PUT перезаписывает
 * строку целиком.
 *
 * columns подставляются ВСЕГДА, даже когда правится только имя: бэкенд
 * пишет эту колонку без всяких условий, и тело без неё оставило бы view
 * вообще без колонок.
 *
 * Имя записывается в двух местах — колонка `name` и `attributes.name_<язык>`.
 * Так его читает и старая админка, и мы. ponytail: пустым имя не сделать —
 * колонку `name` бэкенд обновляет только непустым значением, и вкладка
 * откатится на прежнее имя, а не на тип.
 */
export function toUpdateBody({
  view,
  name,
  language,
  type,
  columns,
  quickFilters,
  fixedColumns,
  defaultFilters,
  navigate,
  objectUrl,
  pdfUrl,
  infiniteScroll,
  groupBy,
  tabGroup,
  subGroup,
  dateFrom,
  dateTo,
  charts,
}: ViewEdit): Record<string, unknown> {
  const raw = view.raw;
  const trimmed = name?.trim();

  const attributes: Record<string, unknown> = {
    ...((raw["attributes"] as Record<string, unknown> | undefined) ?? {}),
    ...(trimmed === undefined ? {} : { [`${NAME_PREFIX}${language ?? ""}`]: trimmed }),
    /*
     * is_checked дописывается к каждому полю: по нему старая админка
     * считает счётчик чипов (ViewForm) и решает, рисовать ли поле
     * в подшапке. Без флага наши quick_filters она видит, но не считает.
     * Флаг всегда true — «поле в списке» и означает «предложено».
     */
    ...(quickFilters === undefined
      ? {}
      : { quick_filters: quickFilters.map((field) => ({ ...field.raw, is_checked: true })) }),
    // Объект, а не список: так эту настройку читает старая админка,
    // и она же остаётся единственным источником правды о закреплении.
    ...(fixedColumns === undefined
      ? {}
      : { fixedColumns: Object.fromEntries(fixedColumns.map((id) => [id, true])) }),
    ...(defaultFilters === undefined ? {} : { default_filters: defaultFilters }),
    /*
     * Адреса пишутся объектом `{url, params}` — так их читает и пишет
     * старая админка. Пустой адрес отправляется тоже: иначе стёртый
     * руками адрес возвращался бы из прежних attributes (тело собирается
     * поверх них), и щелчок по строке продолжал бы уводить со страницы.
     */
    ...(navigate === undefined ? {} : { navigate: toUrlAttribute(navigate) }),
    ...(objectUrl === undefined ? {} : { url_object: toUrlAttribute(objectUrl) }),
    ...(pdfUrl === undefined ? {} : { pdf_url: pdfUrl.trim() }),
    ...(infiniteScroll === undefined ? {} : { infinite_scroll: infiniteScroll }),
    /*
     * Список ключей полей — так настройку хранит и читает старая админка
     * (attributes.group_by_columns; её панель «Group» — список галочек,
     * а не выбор одного поля). Порядок = порядок уровней.
     */
    ...(groupBy === undefined ? {} : { group_by_columns: groupBy }),
    /*
     * Дорожки доски. Снятие — именно `null`, а не пустая строка: так
     * его пишет старая админка, и её же экран сравнивает ключ с id поля
     * (BoardSubGroup.jsx:77) — пустая строка сравнением не отличается
     * от «поля с пустым id», а null отличается.
     */
    ...(subGroup === undefined ? {} : { sub_group_by_id: subGroup || null }),
    /*
     * Графики. Список целиком, включая пустой: убрать последний график
     * — такая же правка, как добавить первый, и «не трогали» здесь
     * означает только `undefined`.
     */
    ...(charts === undefined ? {} : { charts: toChartsAttribute(charts) }),
  };

  return {
    ...raw,
    columns: columns ?? view.columnIds,
    attributes,
    ...(trimmed === undefined ? {} : { name: trimmed }),
    ...(type === undefined ? {} : { type }),
    /*
     * Колонка таблицы, а не ключ attributes: бэкенд пишет её отдельным
     * `group_fields = $N`. Нетронутой она уезжает через `raw`, поэтому
     * подменяется только когда правили именно её.
     */
    ...(tabGroup === undefined ? {} : { group_fields: tabGroup ? [tabGroup] : [] }),
    /*
     * Даты календаря — тоже колонки таблицы, и бэкенд пишет их БЕЗ
     * условия: `calendar_from_slug = $N` уходит в UPDATE при каждом PUT,
     * что бы ни правили (view.go, Update). Поэтому тело, собранное
     * заново, стёрло бы настройку календаря правкой имени; нетронутыми
     * они уезжают через `raw`.
     */
    ...(dateFrom === undefined ? {} : { calendar_from_slug: dateFrom }),
    ...(dateTo === undefined ? {} : { calendar_to_slug: dateTo }),
  };
}

/**
 * Адрес → attributes. Параметры без ключа отбрасываются: это строки,
 * которые добавили и не заполнили.
 */
function toUrlAttribute(template: UrlTemplate): Record<string, unknown> {
  return {
    url: template.url.trim(),
    params: template.params
      .filter((param) => param.key.trim())
      .map((param) => ({ key: param.key.trim(), value: param.value.trim() })),
  };
}

/** Пустая строка и мусор — это «не задано», а не ноль строк на странице. */
function toLimit(value: string | number | undefined): number | null {
  const parsed = typeof value === "number" ? value : parseInt(String(value ?? ""), 10);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null;
}

/**
 * quick_filters приходит массивом полей целиком. У связей ключ тот же,
 * что и в columns, — id связи, а не поля.
 */
function toQuickFilterIds(attributes: Record<string, unknown> | undefined): string[] {
  const list = attributes?.["quick_filters"];
  if (!Array.isArray(list)) return [];

  return list
    .map((item: unknown) => {
      if (typeof item !== "object" || item === null) return "";
      const field = item as { id?: string; relation_id?: string };
      return field.relation_id || field.id || "";
    })
    .filter(Boolean);
}

/**
 * Имя вкладки по языкам данных. Лежит в attributes россыпью ключей
 * `name_<short_name>` — тот же приём, что и `label_<short_name>` у полей,
 * и те же языки проекта (features/workspace), а не локаль интерфейса.
 *
 * Пустые пропускаются: у половины view в attributes лежит `name_ru: ""`,
 * и без проверки вкладка получала бы пустое имя вместо типа.
 */
const NAME_PREFIX = "name_";

function toNames(attributes: Record<string, unknown> | undefined): Record<string, string> {
  const names: Record<string, string> = {};

  for (const [key, value] of Object.entries(attributes ?? {})) {
    if (!key.startsWith(NAME_PREFIX) || typeof value !== "string" || !value.trim()) continue;
    names[key.slice(NAME_PREFIX.length)] = value;
  }

  return names;
}

function toViews(data: ViewsResponseDto): View[] {
  return (data.views ?? []).filter((dto) => dto.id).map(toView);
}

export function toView(dto: ViewDto): View {
  return {
    id: dto.id ?? "",
    type: dto.type ?? "",
    tableSlug: dto.table_slug ?? "",
    name: dto.name?.trim() ?? "",
    names: toNames(dto.attributes),
    // Поле есть не у всех типов: у TABLE его в ответе нет вовсе.
    order: dto.order ?? 0,
    defaultLimit: toLimit(dto.default_limit),
    quickFilterIds: toQuickFilterIds(dto.attributes),
    isRelationView: dto.is_relation_view ?? false,
    relationTableSlug: dto.relation_table_slug ?? "",
    relationId: dto.relation_id ?? "",
    tableLabel: dto.table_label?.trim() ?? "",
    columnIds: dto.columns ?? [],
    barFieldSlugs: toBarFieldSlugs(dto.attributes),
    fixedColumnIds: toFixedColumnIds(dto.attributes),
    defaultFilters: toDefaultFilters(dto.attributes),
    navigate: toUrlTemplate(dto.attributes?.["navigate"]),
    objectUrl: toUrlTemplate(dto.attributes?.["url_object"]),
    pdfUrl: typeof dto.attributes?.["pdf_url"] === "string" ? dto.attributes["pdf_url"] : "",
    infiniteScroll: dto.attributes?.["infinite_scroll"] === true,
    groupByIds: toGroupByIds(dto.attributes),
    tabGroupId: typeof dto.group_fields?.[0] === "string" ? dto.group_fields[0] : "",
    /* Дорожки доски. Ключ снимается через null: старая админка пишет
       туда именно `null`, а не пустую строку (useBoardSubGroupProps.jsx:15). */
    subGroupId:
      typeof dto.attributes?.["sub_group_by_id"] === "string"
        ? dto.attributes["sub_group_by_id"]
        : "",
    dateFromSlug: toDateSlug(dto.calendar_from_slug, dto.attributes, "calendar_from_slug"),
    dateToSlug: toDateSlug(dto.calendar_to_slug, dto.attributes, "calendar_to_slug"),
    statusFieldSlug: dto.status_field_slug?.trim() ?? "",
    disableDates: toDisableDates(dto.disable_dates),
    charts: toCharts(dto.attributes?.["charts"]),
    period: typeof dto.attributes?.["period"] === "string" ? dto.attributes["period"] : "",
    raw: { ...dto },
  };
}

/**
 * Поля, показанные на полосе таймлайна, — у view, заведённых старой
 * админкой.
 *
 * Её панель «колонки» на таймлайне пишет НЕ `columns`, а
 * `attributes.visible_field` — слаги через косую черту
 * (ColumnsVisibility/useColumnsVisibilityProps.jsx: `visible_field + "/" +
 * column.slug`). То есть у такого view набор колонок лежит в другом
 * месте, чем у всех остальных, и по `columns` он пуст.
 *
 * Читается только как запасной вариант (см. resolveColumns) и только
 * ради этих view: пишем мы всегда `columns`, и первое же сохранение
 * колонок в нашей панели делает эту ступень ненужной.
 */
function toBarFieldSlugs(attributes: Record<string, unknown> | undefined): string[] {
  const value = attributes?.["visible_field"];
  if (typeof value !== "string") return [];

  return value
    .split("/")
    .map((slug) => slug.trim())
    .filter(Boolean);
}

/**
 * Поле даты: колонка таблицы, а при её пустоте — тот же ключ
 * из attributes.
 *
 * Это не `a || b` из запрещённых: истина одна — колонка, и пишем мы
 * только её. Ключ в attributes читается ради view, СОЗДАННЫХ старой
 * админкой: её форма создания клала слаги только туда
 * (useViewCreatePopupProps.jsx), хотя её же сохранение писало колонку.
 * Без этой ступени настроенный годами календарь открылся бы у нас
 * пустым экраном «выберите поле даты». Первое же сохранение переносит
 * значение в колонку, и ступень перестаёт срабатывать.
 *
 * Место у неё ровно одно — здесь: наружу уходит один слаг, и ни один
 * компонент про attributes не знает.
 */
function toDateSlug(
  column: string | undefined,
  attributes: Record<string, unknown> | undefined,
  key: string,
): string {
  const legacy = attributes?.[key];
  return column?.trim() || (typeof legacy === "string" ? legacy.trim() : "");
}

/**
 * Нерабочие дни. Обе половины обязательны: без слага таблицы спрашивать
 * не у кого, без колонки дня — нечего сопоставлять с клетками календаря.
 *
 * Колонки времени (`time_from_slug`, `time_to_slug`) не читаются: они
 * описывают ЧАСТЬ дня, а закрашенный наполовину день — это отдельная
 * раскладка, которой в v2 нет. Целый день закрыт или нет — вот и всё,
 * что календарь показывает.
 */
function toDisableDates(raw: ViewDto["disable_dates"]): View["disableDates"] {
  const tableSlug = raw?.table_slug?.trim() ?? "";
  const daySlug = raw?.day_slug?.trim() ?? "";

  return tableSlug && daySlug ? { tableSlug, daySlug } : null;
}

/**
 * fixedColumns лежит объектом `{id: true}`. Снятая колонка иногда
 * остаётся ключом со значением false — старая админка писала туда
 * `Object.fromEntries(fixed.map(...))` не всегда, — поэтому значение
 * проверяется, а не только наличие ключа.
 */
function toFixedColumnIds(attributes: Record<string, unknown> | undefined): string[] {
  const fixed = attributes?.["fixedColumns"];
  if (typeof fixed !== "object" || fixed === null) return [];

  return Object.entries(fixed as Record<string, unknown>)
    .filter(([, value]) => Boolean(value))
    .map(([id]) => id);
}

/** Первый элемент group_by_columns: группируем по одному полю. */
function toGroupByIds(attributes: Record<string, unknown> | undefined): string[] {
  const list = attributes?.["group_by_columns"];
  if (!Array.isArray(list)) return [];

  return list.filter((id): id is string => typeof id === "string" && Boolean(id));
}

function toDefaultFilters(attributes: Record<string, unknown> | undefined): Record<string, unknown> {
  const filters = attributes?.["default_filters"];
  if (typeof filters !== "object" || filters === null || Array.isArray(filters)) return {};

  return filters as Record<string, unknown>;
}
