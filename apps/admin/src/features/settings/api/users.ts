import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authApi } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Люди проекта: кто и под какой ролью в него входит.
 *
 * Живут в сервисе АВТОРИЗАЦИИ — как роли и типы клиентов: «пользователь»
 * здесь не строка таблицы, а личность с логином, паролем и ролью.
 * Отсюда authApi и `project-id` параметром в каждом запросе: наш клиент
 * не дописывает его перехватчиком, а ручки без него отвечают отказом.
 *
 * Список приходит по ОДНОМУ типу клиента (`client-type-id`), и это
 * не наша прихоть: у каждого типа своя таблица входа, и общего списка
 * «все люди проекта» в ручках нет вовсе. Поэтому вкладки по типам —
 * не украшение, а форма ответа.
 */
const USERS = "/v2/user";

type UserDto = {
  id?: string;
  name?: string;
  login?: string;
  email?: string;
  phone?: string;
  photo_url?: string;
  role_id?: string;
  client_type_id?: string;
  /** 0 — выключен, 1 — активен. Число, а не булево: так в proto. */
  active?: number;
};

type UsersResponse = { count?: number; users?: UserDto[] };

export type ProjectUser = {
  id: string;
  name: string;
  login: string;
  email: string;
  phone: string;
  roleId: string;
  clientTypeId: string;
  active: boolean;
  /** Ответ целиком: PUT перезаписывает строку, чужие поля терять нельзя. */
  raw: Record<string, unknown>;
};

export const USERS_PAGE = 20;

export function useUsers({
  clientTypeId,
  search,
  page,
  limit,
}: {
  clientTypeId: string;
  search: string;
  page: number;
  limit: number;
}) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.users(projectId, clientTypeId, search, page, limit),
    queryFn: () =>
      authApi.get<UsersResponse>(USERS, {
        params: {
          "project-id": projectId,
          "client-type-id": clientTypeId,
          search,
          limit,
          offset: (page - 1) * limit,
        },
      }),
    enabled: Boolean(projectId && clientTypeId),
    // Людей заводят и выключают по ходу работы — держим свежее справочников.
    staleTime: 30_000,
    select: (data) => ({
      count: data.count ?? 0,
      users: (data.users ?? []).filter((dto) => dto.id).map(toUser),
    }),
  });

  return {
    users: query.data?.users ?? NO_USERS,
    count: query.data?.count ?? 0,
    isLoading: query.isLoading,
    error: query.error,
  };
}

const NO_USERS: ProjectUser[] = [];

function toUser(dto: UserDto): ProjectUser {
  return {
    id: dto.id ?? "",
    name: dto.name?.trim() ?? "",
    login: dto.login?.trim() ?? "",
    email: dto.email?.trim() ?? "",
    phone: dto.phone?.trim() ?? "",
    roleId: dto.role_id ?? "",
    clientTypeId: dto.client_type_id ?? "",
    // Признак приходит числом и не всегда: отсутствие — это «активен».
    active: dto.active !== 0,
    raw: { ...dto },
  };
}

export type UserDraft = {
  name: string;
  login: string;
  email: string;
  phone: string;
  password: string;
  roleId: string;
};

/**
 * Новый человек. Пароль задаётся здесь же и больше не показывается —
 * сервер хранит хеш, и прочитать его назад нельзя ни нам, ни ему.
 *
 * Проект уезжает в теле, окружение бэкенд берёт из заголовка запроса:
 * человек заводится в ресурсе того окружения, в котором мы сейчас.
 */
export function useCreateUser(clientTypeId: string) {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (draft: UserDraft) =>
      authApi.post<unknown>(
        USERS,
        {
          name: draft.name.trim(),
          login: draft.login.trim(),
          email: draft.email.trim(),
          phone: draft.phone.trim(),
          password: draft.password,
          role_id: draft.roleId,
          client_type_id: clientTypeId,
          project_id: projectId,
        },
        { params: { "project-id": projectId } },
      ),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("users.created"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.usersAll() });
    },
  });
}

/**
 * Правка человека. Тело — поверх ответа, как у профиля: PUT принимает
 * строку целиком, и собранное заново тело обнулило бы тип клиента,
 * компанию и проект — человек перестал бы существовать в своей же роли.
 *
 * Пароль здесь не меняется: для него отдельная ручка и отдельная форма
 * (`/v2/user/reset-password`), где спрашивают прежний.
 */
export function useUpdateUser() {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: ({ user, draft }: { user: ProjectUser; draft: UserDraft }) =>
      authApi.put<unknown>(
        USERS,
        {
          ...user.raw,
          id: user.id,
          name: draft.name.trim(),
          login: draft.login.trim(),
          email: draft.email.trim(),
          phone: draft.phone.trim(),
          role_id: draft.roleId,
        },
        { params: { "project-id": projectId } },
      ),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("settings.saved"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.usersAll() });
    },
  });
}

/**
 * Удаление человека. Тип клиента обязателен параметром: без него ручка
 * отвечает «client type id is required» (auth_service, user_v2.go:507)
 * — по нему она находит таблицу входа, из которой удалять.
 */
export function useDeleteUser(clientTypeId: string) {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (userId: string) =>
      authApi.delete<unknown>(`${USERS}/${userId}`, {
        params: { "project-id": projectId, "client-type-id": clientTypeId },
      }),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("users.deleted"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.usersAll() });
    },
  });
}
