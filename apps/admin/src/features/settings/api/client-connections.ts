import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authApi } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Таблицы аудитории — что приложение заказчика показывает этому типу
 * клиента.
 *
 * В базе это `connections`, и не путать с внешними базами
 * (`api/connections.ts`): там подключение к чужому postgres, здесь —
 * список таблиц ЭТОГО проекта, который уезжает приложению вместе
 * с ответом на вход (`login.go:190`).
 *
 * У связки два дела:
 *
 * - назвать таблицу так, как её видит пользователь приложения (`name`);
 * - сказать, какая строка этой таблицы — «его» (`field_slug`): по нему
 *   ручка `/v2/get-connection-options` находит строку через поле
 *   таблицы входа (`login.go:401`).
 *
 * Чего здесь нет и почему — в docs/PARITY.md: `main_table_slug`
 * не читает ни одна ручка, а `view_slug`, `view_label`, `icon` и `type`
 * ни одна ручка не пишет.
 */
const CONNECTIONS = "/v2/connection";

type ConnectionDto = {
  guid?: string;
  name?: string;
  table_slug?: string;
  field_slug?: string;
};

export type ClientConnection = {
  id: string;
  name: string;
  tableSlug: string;
  fieldSlug: string;
};

export function toClientConnection(dto: ConnectionDto): ClientConnection {
  return {
    id: dto.guid ?? "",
    name: dto.name?.trim() ?? "",
    tableSlug: dto.table_slug?.trim() ?? "",
    fieldSlug: dto.field_slug?.trim() ?? "",
  };
}

export function useClientConnections(clientTypeId: string) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.clientConnections(projectId, clientTypeId),
    queryFn: () =>
      authApi.get<{ data?: { response?: ConnectionDto[] } }>(CONNECTIONS, {
        params: { "project-id": projectId, client_type_id: clientTypeId, limit: 100, offset: 0 },
      }),
    enabled: Boolean(projectId && clientTypeId),
    staleTime: 60_000,
    select: (data): ClientConnection[] =>
      (data.data?.response ?? []).filter((dto) => dto.guid).map(toClientConnection),
  });

  return { connections: query.data ?? NO_CONNECTIONS, isLoading: query.isLoading };
}

const NO_CONNECTIONS: ClientConnection[] = [];

export type ClientConnectionDraft = {
  name: string;
  tableSlug: string;
  fieldSlug: string;
};

export function useSaveClientConnection(clientTypeId: string) {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    /*
     * Одна ручка на создание и правку разная, а тело одно: и POST,
     * и PUT кладут строку в таблицу `connections` целиком. Отличие —
     * `guid`, по которому PUT её и находит.
     */
    mutationFn: ({ id, draft }: { id: string; draft: ClientConnectionDraft }) => {
      const body = {
        ...(id ? { guid: id } : {}),
        name: draft.name.trim(),
        table_slug: draft.tableSlug,
        field_slug: draft.fieldSlug,
        client_type_id: clientTypeId,
      };

      const params = { params: { "project-id": projectId } };

      return id
        ? authApi.put<unknown>(CONNECTIONS, body, params)
        : authApi.post<unknown>(CONNECTIONS, body, params);
    },
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("settings.saved"));
      await queryClient.invalidateQueries({
        queryKey: keys.settings.clientConnections(projectId, clientTypeId),
      });
    },
  });
}

export function useDeleteClientConnection(clientTypeId: string) {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (id: string) =>
      authApi.delete<unknown>(`${CONNECTIONS}/${id}`, {
        params: { "project-id": projectId },
      }),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("clientConnections.deleted"));
      await queryClient.invalidateQueries({
        queryKey: keys.settings.clientConnections(projectId, clientTypeId),
      });
    },
  });
}
