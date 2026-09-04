import { useQuery } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import { keys } from "@/shared/lib/query-keys";

/**
 * Расход API-запросов проекта: остаток месячного лимита и какие
 * маршруты его съедают (`GET /v1/pricing/api-call/breakdown`).
 *
 * Ручка нарочно живёт на админской группе, а не на клиентской:
 * проект с исчерпанным лимитом всё равно может открыть этот экран
 * и увидеть, что произошло, — клиентская группа ответила бы 402.
 * Проект берётся из токена, параметра project_id у ручки нет.
 *
 * Гарантия ручки: сумма строк `top` плюс `other` равна `used` —
 * разбивка и счётчик пишутся одним подсчётом в одной транзакции.
 * В `other` попадает и трафик до выката разбивки: backfill не делался,
 * поэтому в первый месяц `used` большой, а `top` почти пустой.
 */
const BREAKDOWN = "/v1/pricing/api-call/breakdown";

/** Больше строк не просим: хвост ручка сама складывает в `other`. */
const TOP = 10;

type RowDto = {
  source?: string;
  method?: string;
  route?: string;
  collection?: string;
  count?: number;
  percent?: number;
};

type BreakdownDto = {
  limit?: number;
  used?: number;
  remaining?: number | null;
  unlimited?: boolean;
  blocked?: boolean;
  top?: RowDto[];
  other?: number;
};

export type UsageRow = {
  /** `admin` — работа в билдере: лимит расходует, но не блокируется. */
  source: "admin" | "client";
  /** `GET /v2/items/deal`: шаблон маршрута с подставленной таблицей. */
  label: string;
  count: number;
  percent: number;
};

export type Usage = {
  limit: number;
  used: number;
  unlimited: boolean;
  /**
   * Блокируется ли клиентское API прямо сейчас. Флаг из Redis, счётчик
   * из базы: расходиться на четверть часа им можно. Флаг — факт,
   * процент — оценка, поэтому баннер рисуется по флагу.
   */
  blocked: boolean;
  /** Израсходовано, 0–100. null — тариф без лимита, делить не на что. */
  percentUsed: number | null;
  top: UsageRow[];
  other: number;
};

export function useUsage() {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.usage(projectId),
    queryFn: () => api.get<BreakdownDto>(BREAKDOWN, { params: { limit: TOP } }),
    enabled: Boolean(projectId),
    /*
     * До базы цифры доезжают раз в десять минут, а каждый запрос сюда
     * сам расходует лимит: перезапрашивать чаще минуты нет смысла.
     */
    staleTime: 60_000,
    select: toUsage,
  });

  return { usage: query.data, isLoading: query.isLoading, error: query.error };
}

export function toUsage(dto: BreakdownDto): Usage {
  const limit = dto.limit ?? 0;
  const used = dto.used ?? 0;
  // Безлимит приходит и флагом, и нулевым лимитом — верим любому из них.
  const unlimited = Boolean(dto.unlimited) || !limit;

  return {
    limit,
    used,
    unlimited,
    blocked: Boolean(dto.blocked),
    percentUsed: unlimited ? null : Math.min(100, (used / limit) * 100),
    top: (dto.top ?? []).map(toRow),
    other: dto.other ?? 0,
  };
}

function toRow(dto: RowDto): UsageRow {
  const route = dto.route ?? "";
  const collection = dto.collection ?? "";

  return {
    source: dto.source === "admin" ? "admin" : "client",
    /*
     * В `route` лежит шаблон (`/v2/items/:collection`), таблица —
     * отдельным полем: иначе на каждый id заводилась бы своя строка.
     * Человеку показываем уже собранный адрес.
     */
    label: [dto.method, collection ? route.replace(":collection", collection) : route]
      .filter(Boolean)
      .join(" "),
    count: dto.count ?? 0,
    percent: dto.percent ?? 0,
  };
}
