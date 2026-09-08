import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { keys } from "@/shared/lib/query-keys";
import { errorMessage, reportError } from "@/shared/lib/toast";

/**
 * Дашборды ресурса METABASE.
 *
 * Списком и ссылкой заведуют две ручки, и обе — POST, хотя ничего
 * не создают в НАШЕЙ базе: `POST /v1/metabase/dashboard` спрашивает
 * Metabase списком по логину и паролю (`visualization.proto:11`),
 * `POST /v1/metabase/public-url` просит у него публичную ссылку.
 *
 * Вторая всё же с последствием, и оно на стороне Metabase: обработчик
 * шлёт `POST /api/dashboard/{id}/public_link`
 * (`company_service/pkg/metabase/metabase.go:198`) — то есть ПУБЛИКУЕТ
 * дашборд, а не читает готовый адрес. Поэтому она mutation, а не query:
 * повторять её кэшем или фоновым перезапросом нельзя.
 */
export type MetabaseDashboard = { id: number; name: string };

type DashboardsDto = { dashboards?: { id?: number; name?: string }[] | null };

export function useMetabaseDashboards(username: string, password: string) {
  const query = useQuery({
    queryKey: keys.settings.metabaseDashboards(username),
    queryFn: () =>
      api.post<DashboardsDto>("/v1/metabase/dashboard", { username, password }),
    enabled: Boolean(username && password),
    // Дашборды заводят в самом Metabase, а не здесь.
    staleTime: 5 * 60_000,
    select: (dto): MetabaseDashboard[] =>
      (dto.dashboards ?? [])
        .filter((item): item is { id: number; name?: string } => typeof item.id === "number")
        .map((item) => ({ id: item.id, name: item.name?.trim() ?? "" })),
  });

  return {
    dashboards: query.data ?? [],
    isLoading: query.isLoading,
    error: errorMessage(query.error, "resources.metabaseFailed"),
  };
}

/**
 * Публичная ссылка на дашборд.
 *
 * Адрес собирается от ПЛАТФОРМЕННОГО Metabase (`Cfg.MetabaseBaseUrl`),
 * а не от того, что записан в поле `url` ресурса: ручка про ресурс
 * ничего не знает — она принимает только `dashboard_id`.
 */
export function useMetabasePublicUrl() {
  return useMutation({
    mutationFn: (dashboardId: number) =>
      api.post<{ url?: string }>("/v1/metabase/public-url", { dashboard_id: dashboardId }),
    onError: (error) => reportError(error, "resources.metabaseFailed"),
  });
}
