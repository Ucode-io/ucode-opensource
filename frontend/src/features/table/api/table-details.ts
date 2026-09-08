import { useQuery } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { keys } from "@/shared/lib/query-keys";
import { errorMessage } from "@/shared/lib/toast";
import type { Field } from "../model/types";
import type { FieldDto } from "./dto";
import { toField } from "./normalize";

/**
 * Подробности таблицы: поля с флагом «участвует в поиске» и view
 * с правами роли на них.
 *
 * Отдельная ручка — не от хорошей жизни. Оба ответа, которыми экран
 * живёт обычно, этих двух вещей не отдают:
 *
 *   `is_search`               живёт на поле, но в SELECT ручки
 *                             GET /v2/fields его нет
 *   права роли на view        колонки `view`, `edit`, `delete` таблицы
 *                             view_permission; список view
 *                             (`/v3/menus/{id}/views`) о них не знает
 *
 * Обе подставляет POST /v1/table-details/{slug}: он читает роль
 * из токена (шлюз кладёт её в тело как `role_id_from_token`,
 * table.go:1047) и дописывает права каждому view
 * (object_builder.go:741) и каждому полю.
 *
 * Поля берутся отсюда же, а не из схемы: наборы полей у двух ручек
 * не совпадают, и поле, которое есть только в /v2/fields (например ID),
 * выглядело бы вечно выключенным — его флага в ответе просто нет.
 */
export type ViewPermissionDto = { view?: boolean; edit?: boolean; delete?: boolean };

type ViewDto = {
  id?: string;
  attributes?: { view_permission?: ViewPermissionDto };
};

export type TableDetailsDto = {
  table_slug?: string;
  /** Конверт клиент снимает один раз, а внутри лежит ещё один `data`. */
  data?: {
    fields?: (FieldDto & { is_search?: boolean })[];
    views?: ViewDto[];
  };
};

/** Что роли позволено делать с одним view. */
export type ViewRights = {
  /** Показывать его вкладкой. */
  view: boolean;
  /** Менять настройки. */
  edit: boolean;
  delete: boolean;
};

/** Права не пришли — значит, никто ничего не запрещал. См. toViewRights. */
export const ALL_VIEW_RIGHTS: ViewRights = { view: true, edit: true, delete: true };

export type TableDetails = {
  fields: Field[];
  /** id полей, по которым бэкенд ищет. */
  enabled: ReadonlySet<string>;
  /** Права роли по id view. Нет записи — нет и ограничения. */
  viewRights: ReadonlyMap<string, ViewRights>;
};

const EMPTY: TableDetails = { fields: [], enabled: new Set(), viewRights: new Map() };

export function useTableDetails(tableSlug: string | undefined) {
  const slug = tableSlug ?? "";

  const { data, isFetching, error } = useQuery({
    queryKey: keys.tables.details(slug),
    // Тело обязательно с ключом data: шлюз пишет в эту карту служебные
    // поля из токена (table.go: objectRequest.Data[...] = ...), и на
    // пустом теле карта nil — ответ 500 без единого слова.
    queryFn: () => api.post<TableDetailsDto>(`/v1/table-details/${slug}`, { data: {} }),
    enabled: Boolean(slug),
    staleTime: 60_000,
    select: toTableDetails,
  });

  return {
    ...(data ?? EMPTY),
    isFetching,
    /** Причина отказа словами. null — всё в порядке. */
    error: errorMessage(error, "table.loadFailed"),
  };
}

export function toTableDetails(dto: TableDetailsDto): TableDetails {
  const fields = dto.data?.fields ?? [];
  const views = dto.data?.views ?? [];

  return {
    fields: fields.map(toField),
    enabled: new Set(
      fields.filter((field) => field.is_search === true).map((field) => field.id ?? ""),
    ),
    viewRights: new Map(
      views
        .filter((view) => view.id && view.attributes?.view_permission)
        .map((view) => [view.id ?? "", toViewRights(view.attributes?.view_permission)]),
    ),
  };
}

/**
 * Запрет строгий, разрешение по умолчанию — то же правило, что и у прав
 * на поля и на действия.
 *
 * Записи прав заводятся при создании view для ролей, какие есть на тот
 * момент (view.go:178); роль, заведённая позже, записи не получает.
 * Хуже того, запрос прав написан через `QueryRow(...).Scan(...)`
 * (object_builder.go:726) — без строки это ошибка, и вся ручка отвечает
 * отказом. То есть «прав нет» доходит до нас либо как отсутствие ключа,
 * либо как отсутствие ответа целиком; запрещённым мы такое не считаем.
 */
function toViewRights(dto: ViewPermissionDto | undefined): ViewRights {
  if (!dto) return ALL_VIEW_RIGHTS;

  return {
    view: dto.view !== false,
    edit: dto.edit !== false,
    delete: dto.delete !== false,
  };
}
