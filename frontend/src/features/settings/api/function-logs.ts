import { useQuery } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import { keys } from "@/shared/lib/query-keys";
import { errorMessage } from "@/shared/lib/toast";

/**
 * Журнал ВЫПОЛНЕНИЯ функций.
 *
 * Предмет другой, чем у журнала изменений: там — кто и что поменял
 * в проекте, здесь — как отработал вызов. Отсюда и вкладка рядом,
 * а не строки в общем списке: колонки у них не совпадают ни одной,
 * кроме даты.
 *
 * Ручка одна на оба вида проектов только по имени: ветка есть лишь
 * у PostgreSQL (`api/handlers/v2/function_logs.go:81`), у mongo-проекта
 * ответ вернётся пустым. Так же, как у таблиц аудитории, только
 * наоборот.
 *
 * Запись несёт и стоимость вызова — «compute», «db_bandwidth»,
 * «file_bandwidth», «vector_bandwidth» (`pg_version_history.proto:70`).
 * Мы их не показываем: это единицы биллинга, а раздела биллинга у нас
 * нет, и четыре колонки чисел без цены за единицу не читаются никак.
 */
export type FunctionLog = {
  id: string;
  functionId: string;
  functionName: string;
  tableSlug: string;
  method: string;
  actionType: string;
  /** Когда вызов ушёл. */
  sentAt: string;
  status: string;
  /** Сколько шёл. В миллисекундах — так его пишет бэкенд. */
  duration: number;
};

type LogDto = {
  id?: string;
  function_id?: string;
  function_name?: string;
  table_slug?: string;
  request_method?: string;
  action_type?: string;
  send_at?: string;
  status?: string;
  duration?: number;
};

type ListDto = { function_logs?: LogDto[] | null; total_count?: number };

export type FunctionLogFilters = {
  /** Поиск по имени функции. Сервер сравнивает сам. */
  search: string;
  /** Слаг таблицы, над которой вызывали. */
  table: string;
  status: string;
  from: string;
  to: string;
};

export const NO_LOG_FILTERS: FunctionLogFilters = {
  search: "",
  table: "",
  status: "",
  from: "",
  to: "",
};

/** Страница журнала. Сотня — то, что бэкенд берёт сам, если не сказать. */
export const FUNCTION_LOGS_PAGE = 50;

export function useFunctionLogs(filters: FunctionLogFilters, page: number) {
  const envId = useSession().getEnvironmentId() ?? "";

  const params = {
    search: filters.search,
    table: filters.table,
    status: filters.status,
    from_date: filters.from,
    to_date: filters.to,
    limit: FUNCTION_LOGS_PAGE,
    offset: (page - 1) * FUNCTION_LOGS_PAGE,
  };

  const query = useQuery({
    queryKey: keys.functions.logs(envId, params),
    queryFn: () => api.get<ListDto>("/v2/functions/log", { params }),
  });

  return {
    logs: (query.data?.function_logs ?? []).map(
      (dto): FunctionLog => ({
        id: dto.id ?? "",
        functionId: dto.function_id ?? "",
        functionName: dto.function_name?.trim() ?? "",
        tableSlug: dto.table_slug ?? "",
        method: dto.request_method ?? "",
        actionType: dto.action_type ?? "",
        sentAt: dto.send_at ?? "",
        status: dto.status ?? "",
        duration: dto.duration ?? 0,
      }),
    ),
    count: query.data?.total_count ?? 0,
    isLoading: query.isLoading,
    error: errorMessage(query.error, "functionLogs.loadFailed"),
  };
}
