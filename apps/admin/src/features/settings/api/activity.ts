import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import { keys } from "@/shared/lib/query-keys";
import { reportError } from "@/shared/lib/toast";

/**
 * Журнал изменений проекта: кто, когда и что поменял.
 *
 * В бэкенде это `version_history` — туда пишет каждая правящая ручка
 * шлюза (`handler.go:190`), включая заведение окружения, эндпоинта,
 * таблицы, поля и строки. Запись хранит и «было», и «стало», поэтому
 * журнал отвечает не только «кто трогал», но и «что именно изменилось».
 *
 * Журнал ЛЕЖИТ В ОКРУЖЕНИИ: запрос идёт в базу его ресурса
 * (`psqlpool.Get(resource_environment_id)`), и записи соседнего
 * окружения в нём не видны — как и его данные.
 *
 * Отбор — целиком на сервере (`version_history.go:109`), включая поиск
 * по подстроке; фильтровать страницу в памяти значило бы отбирать
 * из двадцати строк, а не из всех.
 */
const HISTORY = "/v2/version/history";

export const ACTIVITY_PAGE = 20;

export type ActivityFilters = {
  /** Тип действия: CREATE, UPDATE ITEM, LOGIN… Сравнение по подстроке. */
  action: string;
  /** Таблица: слаг или подпись. */
  table: string;
  /** Кто: логин или телефон — журнал хранит их, а не идентификатор. */
  user: string;
  /** Границы по дню, `YYYY-MM-DD`: другой формат ручка отвергает. */
  from: string;
  to: string;
};

export const NO_FILTERS: ActivityFilters = {
  action: "",
  table: "",
  user: "",
  from: "",
  to: "",
};

type EntryDto = {
  id?: string;
  action_type?: string;
  action_source?: string;
  table_slug?: string;
  table_label?: string;
  user_info?: string;
  api_key?: string;
  date?: string;
  method_api?: string;
  status_code?: number;
  duration?: number;
  request?: string;
  response?: string;
  /** Опечатка не наша: так поле названо в proto (`previus`). */
  previus?: string;
  previous?: string;
  current?: string;
};

type HistoryResponse = { histories?: EntryDto[]; count?: number };

export type ActivityEntry = {
  id: string;
  action: string;
  /** Что менялось: подпись таблицы, если она есть, иначе слаг. */
  table: string;
  user: string;
  date: string;
  method: string;
  statusCode: number;
  /** Сколько заняло, мс. 0 — ручка не замеряла. */
  duration: number;
  request: string;
  response: string;
  before: string;
  after: string;
};

/** Отбор так, как его ждёт ручка. Один и тот же у списка и у выгрузки. */
function toParams(filters: ActivityFilters) {
  return {
    action_type: filters.action,
    collection: filters.table,
    user_info: filters.user,
    from_date: filters.from,
    to_date: filters.to,
  };
}

export function useActivity(filters: ActivityFilters, page: number, limit: number) {
  const envId = useSession().getEnvironmentId() ?? "";

  const params = {
    limit,
    offset: (page - 1) * limit,
    ...toParams(filters),
  };

  const query = useQuery({
    queryKey: keys.settings.activity(envId, params),
    queryFn: () => api.get<HistoryResponse>(`${HISTORY}/${envId}`, { params }),
    enabled: Boolean(envId),
    /*
     * Журнал дописывается непрерывно, но читают его как отчёт: держим
     * минуту, чтобы возврат из открытой записи не перезапрашивал список.
     */
    staleTime: 60_000,
    select: (data) => ({
      count: data.count ?? 0,
      entries: (data.histories ?? []).filter((dto) => dto.id).map(toEntry),
    }),
  });

  return {
    entries: query.data?.entries ?? NO_ENTRIES,
    count: query.data?.count ?? 0,
    isLoading: query.isLoading,
    error: query.error,
  };
}

const NO_ENTRIES: ActivityEntry[] = [];

/**
 * Одна запись целиком. Список отдаёт те же поля, но своим запросом
 * запись переживает смену отбора и страницы: открытую запись листание
 * не должно захлопывать.
 */
export function useActivityEntry(id: string) {
  const envId = useSession().getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.activityEntry(envId, id),
    queryFn: () => api.get<EntryDto>(`${HISTORY}/${envId}/${id}`),
    enabled: Boolean(envId && id),
    // Запись в журнале неизменна: её незачем перезапрашивать вовсе.
    staleTime: Infinity,
    select: toEntry,
  });

  return { entry: query.data, isLoading: query.isLoading };
}

/**
 * Выгрузка журнала в Excel.
 *
 * Выгружается то, что отобрано на экране, а не весь журнал: выгрузка
 * «всего» из экрана с фильтром выглядит как потеря фильтра. Страница
 * при этом не в счёт — файл делают, чтобы посмотреть шире экрана,
 * поэтому предел свой и большой; тот же 20 000 стоит и в самой ручке
 * для mongo-проектов (`version_history.go:445`).
 *
 * Бэкенд не отдаёт файл потоком: он кладёт его в хранилище и отвечает
 * ссылкой без схемы («cdn.host/report_1.xlsx»). Дописываем схему
 * и скачиваем обычной ссылкой — как выгрузка таблицы.
 */
const EXPORT_LIMIT = 20_000;

export function useExportActivity() {
  const envId = useSession().getEnvironmentId() ?? "";

  return useMutation({
    mutationFn: async (filters: ActivityFilters) => {
      const dto = await api.get<{ link?: string }>(`${HISTORY}/${envId}/excel`, {
        params: { limit: EXPORT_LIMIT, offset: 0, ...toParams(filters) },
      });

      const link = dto.link ?? "";
      if (!link) throw new Error("Бэкенд не вернул ссылку на файл");

      return /^https?:\/\//i.test(link) ? link : `https://${link}`;
    },
    onError: (error) => reportError(error, "activity.exportFailed"),
    onSuccess: (link) => {
      const anchor = document.createElement("a");
      anchor.href = link;
      anchor.download = "";
      anchor.click();
    },
  });
}

function toEntry(dto: EntryDto): ActivityEntry {
  return {
    id: dto.id ?? "",
    action: dto.action_type?.trim() || dto.action_source?.trim() || "—",
    table: dto.table_label?.trim() || dto.table_slug?.trim() || "",
    /*
     * В журнале лежит ЛОГИН, а не идентификатор: шлюз меняет одно
     * на другое, когда пишет запись (`handler.go:212`). Для запросов
     * по ключу вместо человека там пусто — показываем сам ключ.
     */
    user: dto.user_info?.trim() || dto.api_key?.trim() || "",
    date: dto.date ?? "",
    method: dto.method_api?.trim() ?? "",
    statusCode: dto.status_code ?? 0,
    duration: dto.duration ?? 0,
    request: dto.request ?? "",
    response: dto.response ?? "",
    before: dto.previus ?? dto.previous ?? "",
    after: dto.current ?? "",
  };
}
