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
};

type RolesResponseDto = { roles?: RoleDto[]; count?: number };

export type Role = {
  id: string;
  name: string;
  /** Системную роль бэкенд не даёт ни переименовать, ни удалить. */
  isSystem: boolean;
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

function toRoles(data: RolesResponseDto): Role[] {
  return (data.roles ?? [])
    .filter((dto) => dto.guid)
    .map((dto) => ({
      id: dto.guid ?? "",
      name: dto.name?.trim() || "—",
      isSystem: dto.is_system === true,
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

export type { RolePermissions, TablePermission };
