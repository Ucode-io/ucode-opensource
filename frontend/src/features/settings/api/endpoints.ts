import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Свои эндпоинты — правила подмены пути для входа `/x-api/...`.
 * Что это и почему так устроено — в `model/endpoint`.
 *
 * Правила принадлежат паре «проект + окружение»: и то и другое шлюз
 * берёт из запроса, а не из тела (`redirect.go:40`). Поэтому окружение
 * в ключе кэша: правило, заведённое в dev, в prod не действует.
 */
const ENDPOINTS = "/v1/redirect-url";

type EndpointDto = {
  id?: string;
  from?: string;
  to?: string;
  order?: number;
  created_at?: string;
  updated_at?: string;
};

export type Endpoint = {
  id: string;
  from: string;
  to: string;
  /** Место в очереди совпадений: шлюз берёт первое подошедшее правило. */
  order: number;
  updatedAt: string;
};

export function useEndpoints() {
  const store = useSession();
  const projectId = store.getProjectId() ?? "";
  const envId = store.getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.endpoints(projectId, envId),
    queryFn: () =>
      api.get<{ redirect_urls?: EndpointDto[]; count?: number }>(ENDPOINTS, {
        params: { limit: 100, offset: 0 },
      }),
    enabled: Boolean(projectId && envId),
    staleTime: 60_000,
    select: (data): Endpoint[] =>
      (data.redirect_urls ?? [])
        .filter((dto) => dto.id)
        .map((dto) => ({
          id: dto.id!,
          from: dto.from ?? "",
          to: dto.to ?? "",
          order: dto.order ?? 0,
          updatedAt: dto.updated_at ?? dto.created_at ?? "",
        }))
        /*
         * Сортируем сами: порядок — это и есть правило разрешения
         * совпадений (шлюз берёт ПЕРВОЕ подошедшее, `proxy.go:14`),
         * а список ручки приходит в порядке базы. У правил, заведённых
         * до появления колонки, `order` нулевой — такие уходят вниз
         * вместе, но между собой не перемешиваются: сортировка
         * устойчива.
         */
        .sort((left, right) => left.order - right.order),
  });

  return { endpoints: query.data ?? NO_ENDPOINTS, isLoading: query.isLoading, error: query.error };
}

const NO_ENDPOINTS: Endpoint[] = [];

export type EndpointDraft = { from: string; to: string };

/**
 * Новое правило. Проект и окружение шлюз проставляет сам; порядок —
 * следом за последним, иначе новое правило встанет первым и перехватит
 * то, что уже работало.
 */
export function useCreateEndpoint(count: number) {
  const invalidate = useInvalidateEndpoints();

  return useMutation({
    mutationFn: (draft: EndpointDraft) =>
      api.post<unknown>(ENDPOINTS, { from: draft.from, to: draft.to, order: count + 1 }),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("endpoints.created"));
      await invalidate();
    },
  });
}

export function useUpdateEndpoint() {
  const invalidate = useInvalidateEndpoints();

  return useMutation({
    mutationFn: ({ id, draft }: { id: string; draft: EndpointDraft }) =>
      api.put<unknown>(ENDPOINTS, { id, from: draft.from, to: draft.to }),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("settings.saved"));
      await invalidate();
    },
  });
}

export function useDeleteEndpoint() {
  const invalidate = useInvalidateEndpoints();

  return useMutation({
    mutationFn: (id: string) => api.delete<unknown>(`${ENDPOINTS}/${id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("endpoints.deleted"));
      await invalidate();
    },
  });
}

/**
 * Новый порядок правил. Ручка принимает СПИСОК идентификаторов и
 * расставляет `order` по их позициям (`storage/postgres/redirect.go:226`)
 * — поэтому уезжает весь список, а не пара переставленных.
 */
export function useReorderEndpoints() {
  const invalidate = useInvalidateEndpoints();

  return useMutation({
    mutationFn: (ids: string[]) => api.put<unknown>(`${ENDPOINTS}/re-order`, { ids }),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: () => invalidate(),
  });
}

function useInvalidateEndpoints() {
  const queryClient = useQueryClient();
  const store = useSession();
  const projectId = store.getProjectId() ?? "";
  const envId = store.getEnvironmentId() ?? "";

  return () =>
    queryClient.invalidateQueries({ queryKey: keys.settings.endpoints(projectId, envId) });
}
