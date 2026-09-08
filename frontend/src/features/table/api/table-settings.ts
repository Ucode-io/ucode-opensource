import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";
import type { Labels } from "../model/types";
import { invalidateSchema } from "./fields";
import { pickLabels } from "./normalize";

/**
 * Настройки самой ТАБЛИЦЫ — не view: имя, кэш, мягкое удаление, вход.
 * View показывает строки, таблица их хранит, и эти настройки меняют
 * поведение всех view сразу.
 *
 * Ручки v1, а не v3: v3 живёт под `/v3/menus/{id}/views/{id}/tables`
 * и требует menu_id с view_id, которых у настройки таблицы нет,
 * а его GET читает параметр `table_id`, которого в маршруте
 * `/:collection` не существует вовсе (handlers/v3/table.go:131) —
 * то есть всегда спрашивает пустой id.
 */

/** Сырая таблица. Наружу не выходит. */
type TableDto = {
  id?: string;
  slug?: string;
  label?: string;
  description?: string;
  icon?: string;
  show_in_menu?: boolean;
  subtitle_field_slug?: string;
  is_cached?: boolean;
  soft_delete?: boolean;
  order_by?: boolean;
  is_login_table?: boolean;
  increment_id?: { with_increment_id?: boolean; digit_number?: number; prefix?: string };
  attributes?: Record<string, unknown>;
};

export type TableSettings = {
  id: string;
  slug: string;
  /** Имя без языка — колонка `label`. */
  label: string;
  /** Имена по языкам ДАННЫХ: `attributes.label_<код>`. */
  labels: Labels;
  description: string;
  isCached: boolean;
  softDelete: boolean;
  /** Ручная сортировка строк: бэкенд заводит колонку порядка. */
  orderBy: boolean;
  isLoginTable: boolean;
  /** Способы входа. Пусто у обычной таблицы. */
  loginStrategies: LoginStrategy[];
  /** Отмечать время последнего входа полем `last_activity`. */
  lastActivity: boolean;
  raw: Record<string, unknown>;
};

/**
 * Способы входа, которые бэкенд действительно умеет: по каждому он
 * заводит поле сам (table.go:1019, 1076, 1150) и в паре с ним `password`.
 * Списка «email+телефон+логин» на выбор из шести пунктов, как в старой
 * админке, здесь нет — там половина ничего не делала.
 */
export const LOGIN_STRATEGIES = ["login", "email", "phone"] as const;
export type LoginStrategy = (typeof LOGIN_STRATEGIES)[number];

/**
 * Настройки таблицы по слагу.
 *
 * Слаг в пути — не описка: ручка ждёт id, но object_builder сам
 * подменяет условие на `slug = $1`, когда переданное не uuid
 * (storage/postgres/table.go:688). Id таблицы у нас нигде нет — мы
 * везде живём слагом, — а второй запрос ради него был бы лишним.
 */
export function useTableSettings(tableSlug: string | undefined, enabled = true) {
  const slug = tableSlug ?? "";

  const query = useQuery({
    queryKey: keys.tables.detail(slug),
    queryFn: () => api.get<TableDto>(`/v1/table/${slug}`),
    enabled: Boolean(slug) && enabled,
    // Настройки таблицы правит админ, и правит редко.
    staleTime: 5 * 60_000,
    select: toTableSettings,
  });

  return { table: query.data, isLoading: query.isLoading, error: query.error };
}

export type TableEdit = {
  table: TableSettings;
  /** Имена по языкам данных целиком. */
  labels?: Labels;
  description?: string;
  isCached?: boolean;
  softDelete?: boolean;
  orderBy?: boolean;
  isLoginTable?: boolean;
  loginStrategies?: LoginStrategy[];
  lastActivity?: boolean;
};

/**
 * Правка настроек таблицы.
 *
 * Тело — исходный ответ целиком плюс правка: UPDATE перезаписывает все
 * колонки без условий (storage/postgres/table.go:887), и тело, собранное
 * заново, стёрло бы значок, описание, подзаголовок карточки и настройки
 * инкрементного номера.
 *
 * После ответа обновляется и схема: включённая таблица входа заводит
 * поля `login`/`email`/`phone`/`password`/`last_activity` сама
 * (table.go:1019 и ниже), и без перезапроса их не видно до перезагрузки.
 */
export function useUpdateTableSettings(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (edit: TableEdit) => api.put<unknown>("/v1/table", toUpdateBody(edit)),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("tableSettings.saved"));
      invalidateSchema(queryClient);
      await queryClient.invalidateQueries({ queryKey: keys.tables.detail(slug) });
    },
  });
}

/**
 * Удаление таблицы.
 *
 * Уносит с собой всё, что на неё ссылалось, и это делает бэкенд одной
 * транзакцией (storage/postgres/table.go, Delete): строку `table`,
 * саму таблицу в базе (`DROP TABLE`), раскладки с вкладками и секциями,
 * связи в обе стороны, права на поля и записи, view и — что важно
 * для сайдбара — пункты меню с этим `table_id`. Отдельно чистить нечего,
 * но перезапросить надо почти всё.
 *
 * В пути id, а не слаг: шлюз проверяет его на uuid и отвечает отказом
 * на что угодно другое (api/handlers/v2/collection.go:456). Это редкое
 * место — везде остальное мы живём слагом.
 *
 * Системную таблицу удалить нельзя: бэкенд отвечает ошибкой, и её видно
 * как есть. Признака `is_system` в ответе о таблице нет вовсе, поэтому
 * заранее спрятать действие не по чему.
 */
export function useDeleteTable() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (table: TableSettings) => api.delete<unknown>(`/v2/collections/${table.id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("tableSettings.deleted"));
      /*
       * Меню, view и схема — всё разом: пункт сайдбара исчез вместе
       * с таблицей, а адрес, на котором человек стоит, больше никуда
       * не ведёт. Точечные ключи тут не спасают: удалённая таблица
       * могла быть чужой связью в соседней.
       */
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: keys.menus.all }),
        queryClient.invalidateQueries({ queryKey: keys.views.all }),
        queryClient.invalidateQueries({ queryKey: keys.tables.all }),
      ]);
    },
  });
}

/**
 * Черновик → тело PUT.
 *
 * Слаг уезжает обратно как пришёл: сменить его этой ручкой нельзя —
 * в UPDATE колонки `slug` нет вовсе, — но само значение бэкенду нужно:
 * по нему он заводит поля таблицы входа.
 *
 * `label` пишется и колонкой, и ключом по языку. Колонка одна на все
 * языки, поэтому в неё идёт первое непустое имя: пустая колонка `label`
 * превращает таблицу в безымянную строку во всех списках, где языка
 * данных нет.
 */
export function toUpdateBody({
  table,
  labels,
  description,
  isCached,
  softDelete,
  orderBy,
  isLoginTable,
  loginStrategies,
  lastActivity,
}: TableEdit): Record<string, unknown> {
  const raw = table.raw;
  const nextLabels = labels ?? table.labels;
  const attributes = (raw["attributes"] as Record<string, unknown> | undefined) ?? {};
  const login = isLoginTable ?? table.isLoginTable;
  const authInfo = (attributes["auth_info"] as Record<string, unknown> | undefined) ?? {};
  /*
   * Способы входа отправляются как пришли, пока их не правят: в списке
   * может лежать значение, которого мы не знаем, а разобранный нами
   * список такое отбрасывает — правка кэша молча отняла бы у таблицы
   * способ входа.
   */
  const strategies =
    loginStrategies ??
    (Array.isArray(authInfo["login_strategy"])
      ? (authInfo["login_strategy"] as unknown[])
      : table.loginStrategies);

  const named: Record<string, unknown> = {};
  for (const [code, value] of Object.entries(nextLabels)) {
    named[`label_${code}`] = value.trim();
  }

  return {
    ...raw,
    id: table.id,
    slug: table.slug,
    label: firstNamed(nextLabels) || table.label,
    ...(description === undefined ? {} : { description: description.trim() }),
    is_cached: isCached ?? table.isCached,
    soft_delete: softDelete ?? table.softDelete,
    order_by: orderBy ?? table.orderBy,
    is_login_table: login,
    attributes: {
      ...attributes,
      ...named,
      /*
       * auth_info отправляется только у таблицы входа, и в нём важен
       * ровно один ключ. Остальное бэкенд перезаписывает своим:
       * role_id и client_type_id он кладёт строками "role_id" и
       * "client_type_id", а поля логина выводит из способов входа
       * (table.go:1007). Шесть выпадающих списков старой админки
       * не значили ничего — сохранялось не то, что в них выбрали.
       *
       * Без непустого login_strategy ручка отвечает отказом
       * «login_strategy does not exist», поэтому включение таблицы
       * входа без единого способа сюда не доезжает — см. TableSettings.
       */
      ...(login
        ? {
            auth_info: { ...authInfo, login_strategy: strategies },
            last_activity: lastActivity ?? table.lastActivity,
          }
        : {}),
    },
  };
}

/** Первое непустое имя. Порядок ключей — порядок языков проекта. */
function firstNamed(labels: Labels): string {
  for (const value of Object.values(labels)) {
    if (value.trim()) return value.trim();
  }
  return "";
}

export function toTableSettings(dto: TableDto): TableSettings {
  const attributes = dto.attributes ?? {};
  const authInfo = (attributes["auth_info"] as Record<string, unknown> | undefined) ?? {};

  return {
    id: dto.id ?? "",
    slug: dto.slug ?? "",
    label: dto.label?.trim() ?? "",
    labels: pickLabels(attributes),
    description: dto.description?.trim() ?? "",
    isCached: dto.is_cached === true,
    softDelete: dto.soft_delete === true,
    orderBy: dto.order_by === true,
    isLoginTable: dto.is_login_table === true,
    loginStrategies: toStrategies(authInfo["login_strategy"]),
    lastActivity: attributes["last_activity"] === true,
    raw: { ...dto },
  };
}

/** Чужие значения отбрасываются: способ, которого бэкенд не знает, он не заведёт. */
function toStrategies(value: unknown): LoginStrategy[] {
  if (!Array.isArray(value)) return [];

  return value.filter((item): item is LoginStrategy =>
    (LOGIN_STRATEGIES as readonly unknown[]).includes(item),
  );
}
