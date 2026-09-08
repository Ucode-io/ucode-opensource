import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authApi } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";
import {
  toRolePermissions,
  type RolePermissions,
  type TablePermission,
} from "../model/permissions";

/**
 * Роли проекта и их права на таблицы.
 *
 * Живут на сервере АВТОРИЗАЦИИ, а не на шлюзе: роль — это про то, кем
 * человек вошёл, и права проверяет тот же сервис, что выдаёт токен.
 * Поэтому запросы идут через authApi.
 *
 * `project-id` уезжает параметром адреса в каждом запросе: старый клиент
 * дописывал его перехватчиком ко всем запросам авторизации, наш — нет,
 * а ручка прав без него отвечает отказом (permission_v2.go:571).
 */
const ROLES = "/v2/role";
const ROLE_PERMISSIONS = "/v2/role-permission/detailed";

type RoleDto = {
  guid?: string;
  name?: string;
  client_type_id?: string;
  is_system?: boolean;
  /** Выключенная роль: в списке она есть, но входить под ней нельзя. */
  status?: boolean;
};

/**
 * Ответ завёрнут ДВАЖДЫ: общий конверт снимает http-клиент, а под ним
 * лежит ещё `{table_slug, data: {count, response}}` — ручка отвечает
 * так же, как ручки строк таблицы. Ключа `roles` в ответе нет вовсе.
 */
type RolesResponseDto = { data?: { count?: number; response?: RoleDto[] } };

export type Role = {
  id: string;
  name: string;
  /** Системную роль бэкенд не даёт ни переименовать, ни удалить. */
  isSystem: boolean;
  /**
   * Тип клиента роли. Роль без него не привязана ни к одной таблице
   * входа; на экране людей по нему же отбираются роли вкладки —
   * человеку из типа «клиент» роль администратора не выдают.
   */
  clientTypeId: string;
};

export function useRoles() {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.roles(projectId),
    queryFn: () =>
      authApi.get<RolesResponseDto>(ROLES, {
        params: { "project-id": projectId, limit: 100, offset: 0 },
      }),
    enabled: Boolean(projectId),
    // Роли заводят раз в жизни проекта, а не по ходу работы.
    staleTime: 5 * 60_000,
    select: toRoles,
  });

  return { roles: query.data ?? NO_ROLES, isLoading: query.isLoading, error: query.error };
}

const NO_ROLES: Role[] = [];

export function toRoles(data: RolesResponseDto): Role[] {
  return (data.data?.response ?? [])
    .filter((dto) => dto.guid)
    .map((dto) => ({
      id: dto.guid ?? "",
      name: dto.name?.trim() || "—",
      isSystem: dto.is_system === true,
      clientTypeId: dto.client_type_id ?? "",
    }));
}

/**
 * Права одной роли: список таблиц, у каждой — права на данные и на то,
 * что видно на экране таблицы.
 */
export function useRolePermissions(roleId: string) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.rolePermissions(projectId, roleId),
    queryFn: () =>
      authApi.get<unknown>(`${ROLE_PERMISSIONS}/${projectId}/${roleId}`, {
        params: { "project-id": projectId },
      }),
    enabled: Boolean(projectId && roleId),
    staleTime: 60_000,
    select: toRolePermissions,
  });

  return { permissions: query.data, isLoading: query.isLoading, error: query.error };
}

/**
 * Запись прав роли.
 *
 * Тело — ВЕСЬ объект, который отдал GET, с нашими правками поверх.
 * Иначе нельзя: бэкенд перезаписывает права роли целиком и отказывается
 * работать без `global_permission` (permission.go:1202), а прав на поля,
 * автофильтров и глобальных прав этот экран не показывает — они уезжают
 * обратно нетронутыми.
 */
export function useUpdateRolePermissions(roleId: string) {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (permissions: RolePermissions) =>
      authApi.put<unknown>(
        ROLE_PERMISSIONS,
        { data: permissions.raw, project_id: projectId, role_id: roleId },
        { params: { "project-id": projectId } },
      ),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("roles.saved"));
      await queryClient.invalidateQueries({
        queryKey: keys.settings.rolePermissions(projectId, roleId),
      });
    },
  });
}

/**
 * Новая роль. Тип клиента обязателен: роль без него не привязана
 * ни к одной таблице входа, и войти под ней нельзя.
 *
 * Права у новой роли пустые — их раздают тут же, в матрице.
 */
export function useCreateRole() {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: ({ name, clientTypeId }: { name: string; clientTypeId: string }) =>
      authApi.post<unknown>(
        ROLES,
        { name: name.trim(), client_type_id: clientTypeId, project_id: projectId },
        { params: { "project-id": projectId } },
      ),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("roles.created"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.roles(projectId) });
    },
  });
}

/**
 * Удаление роли. Права уходят вместе с ней; пользователи, вошедшие
 * под этой ролью, останутся без неё — бэкенд их не трогает.
 */
export function useDeleteRole() {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (roleId: string) =>
      authApi.delete<unknown>(`${ROLES}/${roleId}`, {
        params: { "project-id": projectId },
      }),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("roles.deleted"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.roles(projectId) });
    },
  });
}

/**
 * Права роли на пункты меню — своя пара ручек и свой уровень за раз:
 * дерево бэкенд отдаёт по одному родителю, как и само меню.
 *
 * Значения булевы, в отличие от прав на таблицы.
 */
export type MenuPermission = {
  id: string;
  label: string;
  type: string;
  read: boolean;
  write: boolean;
  update: boolean;
  delete: boolean;
};

type MenuPermissionDto = {
  id?: string;
  label?: string;
  type?: string;
  /** Подписи по языкам: `label_ru`, `label_cyr`. У половины пунктов
      колонка `label` пуста, и без них в списке остаётся голый uuid. */
  attributes?: Record<string, unknown>;
  permission?: { read?: boolean; write?: boolean; update?: boolean; delete?: boolean };
};

/**
 * Имя пункта меню: локаль интерфейса, затем любая заданная подпись,
 * затем колонка `label`. Тот же порядок, что и в сайдбаре, — иначе
 * один и тот же пункт называется в двух местах по-разному.
 */
function menuLabel(dto: MenuPermissionDto): string {
  const labels = Object.entries(dto.attributes ?? {})
    .filter(([key, value]) => key.startsWith("label_") && typeof value === "string" && value.trim())
    .map(([key, value]) => [key.slice("label_".length), (value as string).trim()] as const);

  const byLanguage = new Map(labels);

  return (
    byLanguage.get(i18n.language) ??
    labels[0]?.[1] ??
    dto.label?.trim() ??
    ""
  );
}

export function useMenuPermissions(roleId: string, parentId: string) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.menuPermissions(projectId, roleId, parentId),
    queryFn: () =>
      authApi.get<{ menus?: MenuPermissionDto[] }>(
        `/v2/menu-permission/detailed/${projectId}/${roleId}/${parentId}`,
        { params: { "project-id": projectId } },
      ),
    enabled: Boolean(projectId && roleId && parentId),
    staleTime: 60_000,
    select: (data): MenuPermission[] =>
      (data.menus ?? [])
        .filter((dto) => dto.id)
        .map((dto) => ({
          id: dto.id ?? "",
          label: menuLabel(dto) || dto.id || "",
          type: dto.type ?? "",
          read: dto.permission?.read !== false,
          write: dto.permission?.write !== false,
          update: dto.permission?.update !== false,
          delete: dto.permission?.delete !== false,
        })),
  });

  return { menus: query.data ?? NO_MENUS, isLoading: query.isLoading };
}

const NO_MENUS: MenuPermission[] = [];

/**
 * Запись прав на пункты меню.
 *
 * Уезжают ТОЛЬКО изменённые пункты — так же шлёт их старая админка.
 * Целиком дерево слать нечем: оно грузится по уровню, и того, что
 * человек не раскрывал, у нас на руках нет.
 */
export function useUpdateMenuPermissions(roleId: string) {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (menus: MenuPermission[]) =>
      authApi.put<unknown>(
        "/v2/menu-permission/detailed",
        {
          menus: menus.map((menu) => ({
            id: menu.id,
            permission: {
              read: menu.read,
              write: menu.write,
              update: menu.update,
              delete: menu.delete,
            },
          })),
          project_id: projectId,
          role_id: roleId,
        },
        { params: { "project-id": projectId } },
      ),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("roles.saved"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.menuPermissionsAll() });
    },
  });
}

export type { RolePermissions, TablePermission };
