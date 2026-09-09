import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authApi } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * API-ключи проекта: чем чужая программа входит вместо человека.
 *
 * Ключ выдаётся паре «роль + тип клиента» и живёт в ОДНОМ окружении:
 * `environment_id` шлюз берёт из заголовка запроса (api_keys.go:36),
 * то есть из того окружения, в котором мы сейчас. Отсюда и окружение
 * в ключе кэша, и предупреждение на экране: ключ, заведённый в dev,
 * в prod не работает.
 *
 * Живут в сервисе авторизации — как роли и люди.
 */
const API_KEYS = "/v2/api-key";

type ApiKeyDto = {
  id?: string;
  name?: string;
  app_id?: string;
  app_secret?: string;
  role_id?: string;
  client_type_id?: string;
  environment_id?: string;
  status?: string;
  disable?: boolean;
  created_at?: string;
  client_platform?: { id?: string; name?: string };
};

type ApiKeysResponse = { data?: ApiKeyDto[]; count?: number };

export type ApiKey = {
  id: string;
  name: string;
  /** Логин ключа: его отдают наружу вместе с секретом. */
  appId: string;
  appSecret: string;
  roleId: string;
  clientTypeId: string;
  platformName: string;
  /** Выключенным ключом войти нельзя, но он остаётся в списке. */
  active: boolean;
  createdAt: string;
};

export const API_KEYS_PAGE = 20;

export function useApiKeys({ search, page }: { search: string; page: number }) {
  const store = useSession();
  const projectId = store.getProjectId() ?? "";
  const envId = store.getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.apiKeys(projectId, envId, search, page),
    queryFn: () =>
      authApi.get<ApiKeysResponse>(`${API_KEYS}/${projectId}`, {
        params: {
          "project-id": projectId,
          search,
          limit: API_KEYS_PAGE,
          offset: (page - 1) * API_KEYS_PAGE,
        },
      }),
    enabled: Boolean(projectId),
    staleTime: 60_000,
    select: (data) => ({
      count: data.count ?? 0,
      items: (data.data ?? []).filter((dto) => dto.id).map(toApiKey),
    }),
  });

  return {
    apiKeys: query.data?.items ?? NO_KEYS,
    count: query.data?.count ?? 0,
    isLoading: query.isLoading,
    error: query.error,
  };
}

const NO_KEYS: ApiKey[] = [];

function toApiKey(dto: ApiKeyDto): ApiKey {
  return {
    id: dto.id ?? "",
    name: dto.name?.trim() || "—",
    appId: dto.app_id ?? "",
    appSecret: dto.app_secret ?? "",
    roleId: dto.role_id ?? "",
    clientTypeId: dto.client_type_id ?? "",
    platformName: dto.client_platform?.name ?? "",
    /*
     * Выключенность записана ДВУМЯ полями, и пишутся они разными
     * ручками: `disable` — только при создании, `status` — только
     * при правке (storage/postgres/api_keys.go:26 и :336). Поэтому
     * читаем оба: ключ выключен, если так сказало хоть одно.
     */
    active: dto.disable !== true && dto.status !== "INACTIVE",
    createdAt: dto.created_at ?? "",
  };
}

/** Платформы клиента — общий справочник на четыре строки, не проектный. */
export type ClientPlatform = { id: string; name: string };

export function useClientPlatforms(enabled = true) {
  const query = useQuery({
    queryKey: keys.settings.clientPlatforms(),
    queryFn: () =>
      authApi.get<{ client_platforms?: { id?: string; name?: string }[] }>("/v2/client-platform", {
        params: { limit: 50, offset: 0 },
      }),
    enabled,
    // Справочник заполнен миграцией и не меняется.
    staleTime: 30 * 60_000,
    select: (data): ClientPlatform[] =>
      (data.client_platforms ?? [])
        .filter((dto) => dto.id)
        .map((dto) => ({ id: dto.id!, name: dto.name?.trim() || dto.id! })),
  });

  return { platforms: query.data ?? NO_PLATFORMS, isLoading: query.isLoading };
}

const NO_PLATFORMS: ClientPlatform[] = [];

export type ApiKeyDraft = {
  name: string;
  roleId: string;
  clientTypeId: string;
  platformId: string;
  active: boolean;
};

/**
 * Новый ключ. Секрет виден ОДИН раз — сразу после создания; дальше он
 * лежит в ответе списка, но показывать его в таблице рядом с логином
 * значит держать пароль на экране у всякого, кто зашёл в настройки.
 *
 * Платформа обязательна: колонка `client_platform_id` — это uuid
 * со ссылкой на справочник, и пустая строка отвечает «invalid input
 * syntax for type uuid», а не «возьму значение по умолчанию».
 */
export function useCreateApiKey() {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (draft: ApiKeyDraft) =>
      authApi.post<{ app_id?: string; app_secret?: string }>(
        `${API_KEYS}/${projectId}`,
        {
          name: draft.name.trim(),
          role_id: draft.roleId,
          client_type_id: draft.clientTypeId,
          client_platform_id: draft.platformId,
          disable: !draft.active,
        },
        { params: { "project-id": projectId } },
      ),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("apiKeys.created"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.apiKeysAll() });
    },
  });
}

/**
 * Правка ключа. Пишутся ровно четыре поля — имя, роль, тип клиента
 * и состояние (`api_keys.go:336`); остального в UPDATE нет вовсе,
 * поэтому и в форме нет. Платформа ключа после создания не меняется.
 */
export function useUpdateApiKey() {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: ({ id, draft }: { id: string; draft: ApiKeyDraft }) =>
      authApi.put<unknown>(
        `${API_KEYS}/${projectId}/${id}`,
        {
          id,
          name: draft.name.trim(),
          role_id: draft.roleId,
          client_type_id: draft.clientTypeId,
          // Единственное, чем ключ выключается после создания.
          status: draft.active ? "ACTIVE" : "INACTIVE",
        },
        { params: { "project-id": projectId } },
      ),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("settings.saved"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.apiKeysAll() });
    },
  });
}

export function useDeleteApiKey() {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (id: string) =>
      authApi.delete<unknown>(`${API_KEYS}/${projectId}/${id}`, {
        params: { "project-id": projectId },
      }),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("apiKeys.deleted"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.apiKeysAll() });
    },
  });
}
