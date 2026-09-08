import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Подключённые аккаунты репозиториев: GitHub, GitLab, Bitbucket через
 * OAuth.
 *
 * **Это НЕ ресурс GITHUB из списка выше, хотя выглядит похоже.** Ресурс
 * проекта (`project_resource`) хранит пару «логин + токен», которую
 * вбивают руками. Здесь — другая сущность и другая служба:
 * `integration_resource` в company_service, куда токен кладёт сам
 * бэкенд после OAuth (`integration_resource_service.proto:19`). Их
 * читают разные потребители: синхронизация микрофронтенда с чужим
 * репозиторием ходит во вторую.
 *
 * Старая админка их путает: её форма ресурса открывает страницу
 * авторизации GitHub, собранную ПРЯМО В БРАУЗЕРЕ из
 * `VITE_GITHUB_CLIENT_ID` (`ResourcesDetail/Form.jsx:60`), а токен
 * потом всё равно оказывается в поле формы ресурса. Мы этого
 * не повторяем: обмен кода на токен делает бэкенд, и токен сюда
 * не приезжает вовсе — в ответе только имя пользователя.
 *
 * **Возврат мы не обслуживаем.** GitHub возвращает пользователя
 * на `/v1/<провайдер>/callback`, тот сохраняет интеграцию и уводит
 * на адрес из настроек БЭКЕНДА (`GithubFrontendSuccessURL`). Куда
 * именно — нам неизвестно и неважно: интеграция к тому моменту уже
 * сохранена. Поэтому окно авторизации открывается отдельной вкладкой,
 * а состояние перечитывается, когда человек возвращается к нашей.
 */
export const PROVIDERS = [
  { id: "github", label: "GitHub" },
  /* Именно `ext-gitlab`: `/v1/gitlab/*` — это ВНУТРЕННИЙ GitLab
     платформы, тот, в котором лежат форки шаблонов функций. */
  { id: "ext-gitlab", label: "GitLab" },
  { id: "bitbucket", label: "Bitbucket" },
] as const;

export type ProviderId = (typeof PROVIDERS)[number]["id"];

export type Integration = { id: string; username: string; name: string };

type IntegrationDto = { id?: string; username?: string; name?: string };

export function useIntegration(provider: ProviderId) {
  const envId = useSession().getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.integration(envId, provider),
    queryFn: () => api.get<IntegrationDto>(`/v1/${provider}/integration`),
    /*
     * «Не подключено» приезжает кодом 404, а не пустым телом. Это
     * не отказ и повторять его незачем — превращаем в отсутствие
     * значения прямо здесь.
     */
    retry: false,
  });

  const missing = query.error instanceof ApiError && query.error.status === 404;
  const dto = query.data;

  return {
    integration:
      dto?.id && !missing
        ? ({ id: dto.id, username: dto.username ?? "", name: dto.name ?? "" } as Integration)
        : undefined,
    isLoading: query.isLoading,
    /** Перечитать: после возврата из окна авторизации. */
    refetch: query.refetch,
  };
}

/**
 * Начать подключение: бэкенд отдаёт адрес страницы авторизации
 * (обычной строкой), мы его открываем.
 *
 * Состояние OAuth (`state`) кладёт в Redis тот же обработчик
 * (`github.go:107`), поэтому собирать адрес самим не только незачем,
 * но и нельзя: наш `state` бэкенд не признает.
 */
export function useConnectIntegration() {
  return useMutation({
    mutationFn: (provider: ProviderId) => api.get<string>(`/v1/${provider}/connect`),
    onError: (error) => reportError(error, "integrations.connectFailed"),
    onSuccess: (url) => {
      if (typeof url === "string" && url) window.open(url, "_blank", "noopener,noreferrer");
    },
  });
}

export function useDisconnectIntegration() {
  const queryClient = useQueryClient();
  const envId = useSession().getEnvironmentId() ?? "";

  return useMutation({
    mutationFn: ({ provider, id }: { provider: ProviderId; id: string }) =>
      api.delete<unknown>(`/v1/${provider}/integration/${id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async (_data, { provider }) => {
      toast.success(i18n.t("integrations.disconnected"));
      await queryClient.invalidateQueries({
        queryKey: keys.settings.integration(envId, provider),
      });
    },
  });
}
