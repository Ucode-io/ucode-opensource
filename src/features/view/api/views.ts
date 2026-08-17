import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Field } from "@/features/table";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";
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
  /** Имя вкладки. Пустая строка встречается чаще, чем непустая. */
  name?: string;
  /** Строка, а не число: в настройках это свободное поле ввода. */
  default_limit?: string | number;
  columns?: string[];
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
    select: (data) => (data.views ?? []).filter((dto) => dto.id).map(toView),
  });

  return { views: query.data ?? [], isLoading: query.isLoading, error: query.error };
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
 * Тип только TABLE: остальные требуют своих настроек (у BOARD —
 * группирующее поле, у CALENDAR — пара дат), без которых вкладка
 * создаётся пустой, а нарисовать их этот экран всё равно не умеет.
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
    mutationFn: ({ name, language }: { name: string; language: string }) =>
      api.post<ViewDto>(`/v2/views/${slug}`, {
        table_slug: slug,
        menu_id: menuId,
        type: "TABLE",
        order,
        is_relation_view: false,
        // Имя пишется дважды: в колонку и в attributes на языке данных.
        // Старая админка читает только attributes, мы — сначала их же.
        name: name.trim(),
        attributes: { [`name_${language}`]: name.trim() },
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
  };

  return {
    ...raw,
    columns: columns ?? view.columnIds,
    attributes,
    ...(trimmed === undefined ? {} : { name: trimmed }),
    ...(type === undefined ? {} : { type }),
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
    columnIds: dto.columns ?? [],
    fixedColumnIds: toFixedColumnIds(dto.attributes),
    defaultFilters: toDefaultFilters(dto.attributes),
    navigate: toUrlTemplate(dto.attributes?.["navigate"]),
    objectUrl: toUrlTemplate(dto.attributes?.["url_object"]),
    pdfUrl: typeof dto.attributes?.["pdf_url"] === "string" ? dto.attributes["pdf_url"] : "",
    raw: { ...dto },
  };
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

function toDefaultFilters(attributes: Record<string, unknown> | undefined): Record<string, unknown> {
  const filters = attributes?.["default_filters"];
  if (typeof filters !== "object" || filters === null || Array.isArray(filters)) return {};

  return filters as Record<string, unknown>;
}
