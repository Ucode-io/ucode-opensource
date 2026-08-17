import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authApi } from "@/shared/api/client";
import { session } from "@/shared/api/session";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Профиль человека и его сессии.
 *
 * Живут на сервере авторизации (VITE_AUTH_URL), а не на шлюзе: это
 * настройки пользователя, а не проекта. Отсюда authApi вместо api.
 */

type UserDto = {
  id?: string;
  guid?: string;
  name?: string;
  login?: string;
  email?: string;
  phone?: string;
  photo_url?: string;
  role_id?: string;
  client_type_id?: string;
};

export type Profile = {
  id: string;
  name: string;
  login: string;
  email: string;
  phone: string;
  photoUrl: string;
  /** Ответ целиком: PUT перезаписывает строку, и терять чужие поля нельзя. */
  raw: Record<string, unknown>;
};

export function useProfile() {
  const store = useSession();
  const userId = store.getUserId();

  /*
   * Тип клиента обязателен и живёт только в токене: без него ручка
   * отвечает «client type id is an invalid uuid» (auth_service,
   * user_v2.go:191). В ответе логина его нет вовсе.
   */
  const clientTypeId = store.getClientTypeId();

  const query = useQuery({
    queryKey: keys.settings.profile(userId),
    queryFn: () =>
      authApi.get<UserDto>(`/v2/user/${userId}`, {
        params: { "client-type-id": clientTypeId },
      }),
    enabled: Boolean(userId) && Boolean(clientTypeId),
    // Свой профиль человек меняет раз в год.
    staleTime: 5 * 60_000,
    select: toProfile,
  });

  return { profile: query.data, isLoading: query.isLoading };
}

export type ProfileDraft = { name: string; login: string; email: string; phone: string };

/**
 * Правка профиля.
 *
 * Тело — поверх ответа: PUT /v2/user принимает строку целиком, и
 * собранное заново тело обнулило бы роль, тип клиента и проект —
 * человек перестал бы существовать в своей же роли.
 *
 * Имя дублируется в сессию: в сайдбаре оно берётся оттуда, и без этого
 * шапка показывала бы прежнее до следующего входа.
 */
export function useUpdateProfile() {
  const queryClient = useQueryClient();
  const store = useSession();
  const userId = store.getUserId();

  return useMutation({
    mutationFn: ({ profile, draft }: { profile: Profile; draft: ProfileDraft }) =>
      authApi.put<unknown>("/v2/user", {
        ...profile.raw,
        id: profile.id,
        name: draft.name.trim(),
        login: draft.login.trim(),
        email: draft.email.trim(),
        phone: draft.phone.trim(),
      }),

    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async (_data, { draft }) => {
      const current = session.getProfile();
      if (current) session.setProfile({ ...current, name: draft.name.trim() });

      toast.success(i18n.t("settings.saved"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.profile(userId) });
    },
  });
}

/**
 * Смена пароля. Отдельная ручка и отдельная форма: пароль не правится
 * вместе с телефоном — для него нужен прежний, и ошибка в нём означает
 * не «не сохранилось», а «это не вы».
 */
export function useChangePassword() {
  const store = useSession();

  return useMutation({
    mutationFn: ({ oldPassword, password }: { oldPassword: string; password: string }) =>
      authApi.put<unknown>("/v2/user/reset-password", {
        user_id: store.getUserId(),
        client_type_id: store.getClientTypeId(),
        old_password: oldPassword,
        password,
      }),

    onError: (error) => reportError(error, "settings.passwordFailed"),
    onSuccess: () => toast.success(i18n.t("settings.passwordChanged")),
  });
}

export type UserSession = {
  id: string;
  /** Откуда вошли: платформа и устройство, как их записал сервер. */
  device: string;
  ip: string;
  updatedAt: string;
  /** Текущая сессия — та, из которой смотрят. Её не закрывают кнопкой. */
  current: boolean;
};

type SessionDto = {
  id?: string;
  ip?: string;
  data?: string;
  platform?: string;
  device?: string;
  updated_at?: string;
  is_current?: boolean;
};

type SessionsResponse = { sessions?: SessionDto[] };

/** Открытые сессии человека — где он ещё залогинен. */
export function useSessions(enabled = true) {
  const store = useSession();
  const userId = store.getUserId();

  const query = useQuery({
    queryKey: keys.settings.sessions(userId),
    queryFn: () =>
      authApi.get<SessionsResponse>("/v2/session", {
        params: {
          user_id: userId,
          client_type_id: store.getClientTypeId(),
          limit: 50,
          offset: 0,
        },
      }),
    enabled: enabled && Boolean(userId),
    // Список коротких живых записей: держим свежее остальных настроек.
    staleTime: 30_000,
    select: (data) =>
      (data.sessions ?? [])
        .filter((dto) => dto.id)
        .map(
          (dto): UserSession => ({
            id: dto.id!,
            device: [dto.platform, dto.device, dto.data].map((part) => part?.trim()).filter(Boolean).join(" · "),
            ip: dto.ip ?? "",
            updatedAt: dto.updated_at ?? "",
            current: dto.is_current === true,
          }),
        ),
  });

  return { sessions: query.data ?? [], isLoading: query.isLoading };
}

/** Закрыть чужую сессию: выход с того устройства. */
export function useDeleteSession() {
  const queryClient = useQueryClient();
  const userId = useSession().getUserId();

  return useMutation({
    mutationFn: (id: string) => authApi.delete<unknown>(`/v2/session/${id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("settings.sessionClosed"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.sessions(userId) });
    },
  });
}

export function toProfile(dto: UserDto): Profile {
  return {
    id: dto.id || dto.guid || "",
    name: dto.name?.trim() ?? "",
    login: dto.login?.trim() ?? "",
    email: dto.email?.trim() ?? "",
    phone: dto.phone?.trim() ?? "",
    photoUrl: dto.photo_url ?? "",
    raw: { ...dto },
  };
}
