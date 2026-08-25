import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Свои права — те, которых нет в матрице.
 *
 * Матрица прав отвечает про таблицы и кнопки САМОЙ админки. Приложению
 * заказчика этого мало: «может утверждать счета», «видит склад» — такие
 * права придумывает он, а не мы. Здесь они и заводятся: дерево названий
 * с теми же четырьмя правами, что у пунктов меню.
 *
 * Право принадлежит ТИПУ КЛИЕНТА, а раздаётся роли. Поэтому список
 * читается парой «роль + тип»: у роли тип свой, и права чужой аудитории
 * ей показывать нечего. При заведении права бэкенд сам добавляет строку
 * доступа каждой роли этого типа (`custom_permission.go:96`), а при
 * заведении роли — каждому праву (`permission.go:605`).
 *
 * Дерево грузится по уровню, как и меню: ручка отдаёт детей одного
 * родителя.
 *
 * Только на PostgreSQL: mongo-проектам шлюз отвечает «resource type not
 * supported» (`custom_permission.go:242`).
 */
const CUSTOM_PERMISSIONS = "/v1/custom-permission";

/** Право «да/нет» приезжает строкой — так его хранит колонка (CHECK). */
const YES = "Yes";
const NO = "No";

export const CUSTOM_RIGHTS = ["read", "write", "update", "delete"] as const;

export type CustomRight = (typeof CUSTOM_RIGHTS)[number];

export type CustomPermission = {
  id: string;
  title: string;
  /** Пояснение своими словами. Лежит в `attributes.description`. */
  description: string;
  read: boolean;
  write: boolean;
  update: boolean;
  delete: boolean;
};

type AccessDto = {
  custom_permission_id?: string;
  title?: string;
  attributes?: { description?: unknown } | null;
  read?: string;
  write?: string;
  update?: string;
  delete?: string;
};

export function toCustomPermission(dto: AccessDto): CustomPermission {
  const description = dto.attributes?.description;

  return {
    id: dto.custom_permission_id ?? "",
    title: dto.title?.trim() || dto.custom_permission_id || "",
    description: typeof description === "string" ? description.trim() : "",
    read: dto.read === YES,
    write: dto.write === YES,
    update: dto.update === YES,
    delete: dto.delete === YES,
  };
}

/**
 * Права одного уровня. `parentId` пустой — корень дерева: ручка сама
 * подставляет `parent_id IS NULL` (`custom_permission.go:258`).
 */
export function useCustomPermissions({
  roleId,
  clientTypeId,
  parentId,
}: {
  roleId: string;
  clientTypeId: string;
  parentId: string;
}) {
  const query = useQuery({
    queryKey: keys.settings.customPermissions(roleId, clientTypeId, parentId),
    queryFn: () =>
      api.get<{ permissions?: AccessDto[] }>(`${CUSTOM_PERMISSIONS}/accesses`, {
        params: {
          role_id: roleId,
          client_type_id: clientTypeId,
          ...(parentId ? { parent_id: parentId } : {}),
        },
      }),
    enabled: Boolean(roleId && clientTypeId),
    staleTime: 60_000,
    select: (data): CustomPermission[] =>
      (data.permissions ?? []).filter((dto) => dto.custom_permission_id).map(toCustomPermission),
  });

  return { permissions: query.data ?? NO_PERMISSIONS, isLoading: query.isLoading };
}

const NO_PERMISSIONS: CustomPermission[] = [];

/**
 * Новое право. Тип клиента обязателен — без него ручка отвечает
 * «client_type_id is required» (`custom_permission.go:42`): право
 * заводится сразу всем ролям этой аудитории.
 */
export function useCreateCustomPermission(clientTypeId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      title,
      description,
      parentId,
    }: {
      title: string;
      description: string;
      parentId: string;
    }) =>
      api.post<unknown>(CUSTOM_PERMISSIONS, {
        title: title.trim(),
        client_type_id: clientTypeId,
        ...(parentId ? { parent_id: parentId } : {}),
        ...(description.trim() ? { attributes: { description: description.trim() } } : {}),
      }),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("customRights.created"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.customPermissionsAll() });
    },
  });
}

/**
 * Удаление права. Доступы уходят вместе с ним — на них стоит каскад
 * (`000046_create_custom_permission.up.sql:12`), как и на детей:
 * у них `parent_id` обнуляется, и они всплывают в корень.
 */
export function useDeleteCustomPermission() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => api.delete<unknown>(`${CUSTOM_PERMISSIONS}/${id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("customRights.deleted"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.customPermissionsAll() });
    },
  });
}

/**
 * Запись доступов.
 *
 * Запрос на КАЖДОЕ изменённое право: ручка правит одну строку
 * `custom_permission_access` за раз, и списка она не принимает.
 * Правки копятся и уезжают пачкой по кнопке — как на соседних
 * вкладках, чтобы «сохранено» означало одно и то же везде.
 *
 * Четыре права шлём всегда, а не только изменённое: пустую строку
 * ручка пропускает (`custom_permission.go:311`), а «No» — пишет.
 */
export function useUpdateCustomAccess(roleId: string, clientTypeId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (permissions: CustomPermission[]) =>
      Promise.all(
        permissions.map((permission) =>
          api.put<unknown>(`${CUSTOM_PERMISSIONS}/accesses`, {
            role_id: roleId,
            client_type_id: clientTypeId,
            custom_permission_id: permission.id,
            read: permission.read ? YES : NO,
            write: permission.write ? YES : NO,
            update: permission.update ? YES : NO,
            delete: permission.delete ? YES : NO,
          }),
        ),
      ),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("roles.saved"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.customPermissionsAll() });
    },
  });
}
