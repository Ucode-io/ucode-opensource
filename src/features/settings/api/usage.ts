import { useQuery } from "@tanstack/react-query";
import { api, authApi } from "@/shared/api/client";
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

/*
 * До базы цифры доезжают раз в десять минут, а каждый запрос сюда
 * сам расходует лимит: перезапрашивать чаще минуты нет смысла.
 */
const FRESH_FOR = 60_000;

type RowDto = {
  source?: string;
  auth_type?: string;
  method?: string;
  route?: string;
  collection?: string;
  actor_id?: string;
  actor_name?: string;
  /** Временное ведро при `group_by=time`: `2026-09-05 13:15:00`, UTC. */
  bucket?: string;
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
  /** `admin` — работа в админке: лимит расходует, но не блокируется. */
  source: "admin" | "client";
  /** `GET /v2/items/deal`: шаблон маршрута с подставленной таблицей. */
  label: string;
  /** Сырые части маршрута — ими строка раскрывается в список отправителей. */
  method: string;
  route: string;
  collection: string;
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

export function useUsage(clientOnly: boolean) {
  const projectId = useSession().getProjectId() ?? "";

  const params = {
    group_by: "route",
    limit: TOP,
    ...(clientOnly ? { source: "client" } : {}),
  };

  const query = useQuery({
    queryKey: keys.settings.usage(projectId, params),
    queryFn: () => api.get<BreakdownDto>(BREAKDOWN, { params }),
    enabled: Boolean(projectId),
    staleTime: FRESH_FOR,
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
    top: groupRows(dto.top ?? []),
    other: dto.other ?? 0,
  };
}

/**
 * Один маршрут ручка отдаёт несколькими строками — по строке на тип
 * авторизации. Человек спрашивает «кто ест лимит», а не «каким токеном»,
 * поэтому строки складываются в одну; кто вызывал — по раскрытию строки.
 */
function groupRows(dtos: RowDto[]): UsageRow[] {
  const groups = new Map<string, UsageRow>();

  for (const dto of dtos) {
    const source = dto.source === "admin" ? "admin" : "client";
    const route = dto.route ?? "";
    const collection = dto.collection ?? "";
    /*
     * В `route` лежит шаблон (`/v2/items/:collection`), таблица —
     * отдельным полем: иначе на каждый id заводилась бы своя строка.
     * Человеку показываем уже собранный адрес.
     */
    const label = [dto.method, collection ? route.replace(":collection", collection) : route]
      .filter(Boolean)
      .join(" ");

    const group = groups.get(`${source} ${label}`);
    if (group) {
      group.count += dto.count ?? 0;
      // Складываются готовые доли: до сотых, чтобы не тащить хвост float.
      group.percent = Math.round((group.percent + (dto.percent ?? 0)) * 100) / 100;
    } else {
      groups.set(`${source} ${label}`, {
        source,
        label,
        method: dto.method ?? "",
        route,
        collection,
        count: dto.count ?? 0,
        percent: dto.percent ?? 0,
      });
    }
  }

  return [...groups.values()].sort((a, b) => b.count - a.count);
}

export type UsageActor = {
  /** Чем авторизован запрос: `bearer`, `api_key`. Пусто — не записано. */
  authType: string;
  /** Кто: имя ключа или имя пользователя; пусто — автор не записан. */
  name: string;
  count: number;
  /** Доля от запросов ЭТОГО маршрута, а не от всего месяца. */
  percent: number;
};

/**
 * Кто вызывал один маршрут: та же ручка с `group_by=actor` и фильтром
 * по маршруту. Запрос уходит при раскрытии строки — свой хук, чтобы
 * закрытые строки ничего не тянули.
 */
export function useUsageActors(row: UsageRow | null, clientOnly: boolean) {
  const projectId = useSession().getProjectId() ?? "";
  const names = useSenderNames();

  const params = {
    group_by: "actor",
    limit: TOP,
    method: row?.method ?? "",
    route: row?.route ?? "",
    ...(row?.collection ? { collection: row.collection } : {}),
    ...(clientOnly ? { source: "client" } : {}),
  };

  const query = useQuery({
    queryKey: keys.settings.usageActors(projectId, params),
    queryFn: () => api.get<BreakdownDto>(BREAKDOWN, { params }),
    enabled: Boolean(projectId && row),
    staleTime: FRESH_FOR,
    select: toActors,
  });

  return { actors: withNames(query.data ?? NO_ACTORS, names), isLoading: query.isLoading };
}

const NO_ACTORS: RawActor[] = [];

type RawActor = UsageActor & {
  /** Идентификатор пользователя auth-сервиса — по нему ищется имя. */
  actorId: string;
};

export function toActors(dto: BreakdownDto): RawActor[] {
  return (dto.top ?? []).map((row) => ({
    authType: row.auth_type ?? "",
    name: row.actor_name?.trim() ?? "",
    actorId: row.actor_id ?? "",
    count: row.count ?? 0,
    percent: row.percent ?? 0,
  }));
}

/**
 * Имя отправителя: своё из записи (ключи), иначе из списка пользователей
 * проекта, иначе — узнаваемый кусок идентификатора. Пусто — автора нет.
 */
export function withNames(actors: RawActor[], names: Map<string, string>): UsageActor[] {
  return actors.map(({ actorId, ...actor }) => ({
    ...actor,
    name:
      actor.name ||
      names.get(actorId) ||
      (actorId ? `${actorId.slice(0, 8)}…` : ""),
  }));
}

export type UsageDay = {
  /** UTC-день: `2026-09-05`. */
  day: string;
  count: number;
};

/**
 * Когда вызывали раскрытый маршрут: та же ручка с `group_by=time`.
 * Ручка отдаёт 15-минутные вёдра — человеку это шум, складываем в дни.
 * `limit` не передаётся: для времени бэк сам ставит потолок в месяц точек.
 */
export function useUsageTimeline(row: UsageRow | null, clientOnly: boolean) {
  const projectId = useSession().getProjectId() ?? "";

  const params = {
    group_by: "time",
    method: row?.method ?? "",
    route: row?.route ?? "",
    ...(row?.collection ? { collection: row.collection } : {}),
    ...(clientOnly ? { source: "client" } : {}),
  };

  const query = useQuery({
    queryKey: keys.settings.usageTimeline(projectId, params),
    queryFn: () => api.get<BreakdownDto>(BREAKDOWN, { params }),
    enabled: Boolean(projectId && row),
    staleTime: FRESH_FOR,
    select: toTimeline,
  });

  return { days: query.data ?? NO_DAYS, isLoading: query.isLoading };
}

const NO_DAYS: UsageDay[] = [];

// ponytail: день считается по UTC-границе ведра — вечерние запросы могут
// уехать на соседний день; переход на локальные сутки, если это начнёт мешать.
export function toTimeline(dto: BreakdownDto): UsageDay[] {
  const byDay = new Map<string, number>();
  for (const row of dto.top ?? []) {
    const day = (row.bucket ?? "").slice(0, 10);
    if (day) byDay.set(day, (byDay.get(day) ?? 0) + (row.count ?? 0));
  }

  const known = [...byDay.keys()].sort();
  const first = known[0];
  const last = known[known.length - 1];
  if (!first || !last) return [];

  // Пропуски заполняются нулями: дыра в графике читается как баг, а не как тишина.
  const days: UsageDay[] = [];
  for (
    let at = Date.parse(`${first}T00:00:00Z`);
    at <= Date.parse(`${last}T00:00:00Z`);
    at += 86_400_000
  ) {
    const day = new Date(at).toISOString().slice(0, 10);
    days.push({ day, count: byDay.get(day) ?? 0 });
  }
  return days;
}

type UserDto = { id?: string; name?: string; email?: string; login?: string };

/**
 * Пользователи проекта одним списком: id → имя.
 *
 * Резолвим на фронте, потому что ручка разбивки отдаёт для bearer только
 * идентификатор: auth-сервис не селектит `name` в GetUserByID, и бэк
 * не ходит за ним на каждый запрос. Список маленький и меняется редко —
 * пять минут кэша достаточно. Упал запрос — подписи откатываются
 * к короткому идентификатору, ошибкой это не считается.
 */
function useSenderNames(): Map<string, string> {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.usageSenderNames(projectId),
    queryFn: () =>
      authApi.get<{ users?: UserDto[] }>("/v2/user", {
        params: { "project-id": projectId, limit: 1000 },
      }),
    enabled: Boolean(projectId),
    staleTime: 300_000,
    select: (dto) =>
      new Map(
        (dto.users ?? []).flatMap((user) =>
          user.id
            ? [[user.id, user.name?.trim() || user.email?.trim() || user.login?.trim() || ""] as const]
            : [],
        ),
      ),
  });

  return query.data ?? NO_NAMES;
}

const NO_NAMES = new Map<string, string>();
