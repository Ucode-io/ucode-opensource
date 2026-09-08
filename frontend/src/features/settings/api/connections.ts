import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Внешние базы: чужой postgres, таблицы которого показываются как свои.
 *
 * Подключение заводится строкой соединения, после чего его таблицы
 * приезжают списком с признаком «отслеживается». Отмеченная таблица
 * становится обычной [[Table]] проекта — с полями, связями и view;
 * снятая перестаёт быть видимой, но в чужой базе остаётся нетронутой.
 *
 * Живёт это в ресурсе ОКРУЖЕНИЯ (`resource_environment_id`), а не
 * в проекте: подключение, заведённое в dev, в prod не появится.
 * Отсюда окружение в ключе кэша.
 *
 * Работает только с postgres-ресурсом: у шлюза ветка одна,
 * `case pb.ResourceType_POSTGRESQL` (`table.go:1331`), и на mongo-проекте
 * ручка молча отвечает пустотой. Экран об этом и говорит — пустым
 * списком, а не ошибкой: сказать больше нам неоткуда.
 */
const CONNECTIONS = "/v1/connections";

export type Connection = {
  id: string;
  name: string;
  /** Строка соединения с паролем внутри — на экране её не показываем. */
  connectionString: string;
};

export function useConnections() {
  const envId = useSession().getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.connections(envId),
    queryFn: () =>
      api.get<{ connections?: { id?: string; name?: string; connection_string?: string }[] }>(
        CONNECTIONS,
      ),
    enabled: Boolean(envId),
    staleTime: 60_000,
    select: (data): Connection[] =>
      (data.connections ?? [])
        .filter((dto) => dto.id)
        .map((dto) => ({
          id: dto.id!,
          name: dto.name?.trim() || dto.id!,
          connectionString: dto.connection_string ?? "",
        })),
  });

  return {
    connections: query.data ?? NO_CONNECTIONS,
    isLoading: query.isLoading,
    error: query.error,
  };
}

const NO_CONNECTIONS: Connection[] = [];

export type ExternalTable = {
  id: string;
  name: string;
  tracked: boolean;
  /** Сколько колонок нашлось в чужой таблице. */
  fieldCount: number;
};

export function useExternalTables(connectionId: string) {
  const envId = useSession().getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.connectionTables(envId, connectionId),
    queryFn: () =>
      api.get<{
        tables?: { id?: string; table_name?: string; is_tracked?: boolean; fields?: unknown[] }[];
      }>(`${CONNECTIONS}/${connectionId}/tables`),
    enabled: Boolean(envId && connectionId),
    staleTime: 60_000,
    select: (data): ExternalTable[] =>
      (data.tables ?? [])
        .filter((dto) => dto.id)
        .map((dto) => ({
          id: dto.id!,
          name: dto.table_name?.trim() || dto.id!,
          tracked: dto.is_tracked === true,
          fieldCount: dto.fields?.length ?? 0,
        })),
  });

  return { tables: query.data ?? NO_TABLES, isLoading: query.isLoading };
}

const NO_TABLES: ExternalTable[] = [];

/**
 * Новое подключение. Ручка одновременно читает схему чужой базы,
 * поэтому ответ приходит не сразу, а ошибка в строке соединения
 * выясняется здесь же — своей проверки «доступна ли база» у нас нет
 * и быть не может.
 */
export function useCreateConnection() {
  const invalidate = useInvalidateConnections();

  return useMutation({
    mutationFn: ({ name, connectionString }: { name: string; connectionString: string }) =>
      api.post<unknown>(CONNECTIONS, {
        name: name.trim(),
        connection_string: connectionString.trim(),
      }),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("connections.created"));
      await invalidate();
    },
  });
}

/**
 * Отметить таблицы. Уезжают ТОЛЬКО добавленные: ручка принимает список
 * идентификаторов и включает их (`TrackTables`), а не заменяет им
 * прежний набор — снятие делает другая ручка, по одной таблице.
 *
 * После этого таблицы становятся своими: их видит дерево пунктов
 * и список таблиц проекта, поэтому протухает и он.
 */
export function useTrackTables(connectionId: string) {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateConnections();

  return useMutation({
    mutationFn: (tableIds: string[]) =>
      api.post<unknown>(`${CONNECTIONS}/${connectionId}/tables/track`, { table_ids: tableIds }),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("connections.tracked"));
      await invalidate();
      await queryClient.invalidateQueries({ queryKey: keys.tables.all });
    },
  });
}

/**
 * Снять отметку с таблицы. Ручка — POST, а не DELETE (`api.go:134`):
 * это не удаление чужой таблицы, а отказ от неё у себя.
 */
export function useUntrackTable(connectionId: string) {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateConnections();

  return useMutation({
    mutationFn: (tableId: string) =>
      api.post<unknown>(`${CONNECTIONS}/${connectionId}/tables/${tableId}`),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("connections.untracked"));
      await invalidate();
      await queryClient.invalidateQueries({ queryKey: keys.tables.all });
    },
  });
}

function useInvalidateConnections() {
  const queryClient = useQueryClient();
  const envId = useSession().getEnvironmentId() ?? "";

  return () => queryClient.invalidateQueries({ queryKey: keys.settings.connections(envId) });
}
