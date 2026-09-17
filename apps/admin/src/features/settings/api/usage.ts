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

/**
 * Занятое место в базе окружения.
 *
 * Ручка `/v1/pricing/all` отдаёт тринадцать чисел — функции,
 * микрофронтенды, файлы, строки, таблицы, ключи, токены, — а читаем
 * мы одно. Это не расточительство: запрос всё равно один, а экрана
 * «сколько чего в проекте» у нас нет. Появится — он начнётся здесь,
 * и добавятся поля, а не второй запрос.
 *
 * Считается это `pg_database_size(current_database())`
 * (`object_builder.go:3337`) по ресурсу ТЕКУЩЕГО окружения
 * (`pricing.go:63` — `GetResourceUsage(resourceEnvId)`), а лимит
 * приходит из тарифа ПРОЕКТА. Отсюда окружение в ключе кэша: в prod
 * и dev занято разное.
 *
 * Байты в мегабайты переводит шлюз (`pricing.go:131`), лимит он же
 * разбирает из строки тарифа («5GB», «500MB») — всё остальное
 * превращается в ноль (`parseStorageLimitToMB:22`). Ноль поэтому
 * значит «лимит неизвестен», а не «места нет».
 *
 * Ноль в `current` двусмыслен по той же причине: каждый поход внутри
 * ручки глотает свою ошибку и возвращает пустой ответ (`pricing.go:59`
 * и ещё шесть таких же), так что пустая база и упавший подсчёт
 * выглядят одинаково. Врать про это нечем — показываем, что пришло.
 */
const PRICING = "/v1/pricing/all";

export type Storage = {
  /** Занято, МБ. */
  used: number;
  /** Потолок тарифа, МБ. 0 — тариф его не называет. */
  limit: number;
  /** Доля лимита. null — лимита нет, сравнивать не с чем. */
  percentUsed: number | null;
};

export function useDatabaseSize() {
  const store = useSession();
  const projectId = store.getProjectId() ?? "";
  const envId = store.getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.databaseSize(projectId, envId),
    queryFn: () => api.get<{ database_size?: { current?: number; limit?: number } }>(PRICING),
    enabled: Boolean(projectId && envId),
    /* Объём базы меняется медленнее, чем счётчик запросов: минуты
       хватало бы, но и пять — не устаревшая цифра на этом экране. */
    staleTime: 5 * FRESH_FOR,
    select: (dto): Storage => {
      const used = dto.database_size?.current ?? 0;
      const limit = dto.database_size?.limit ?? 0;

      return { used, limit, percentUsed: limit ? Math.min(100, (used / limit) * 100) : null };
    },
  });

  return { storage: query.data, isLoading: query.isLoading };
}

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
  /** Идентификатор пользователя auth-сервиса: имя и фильтр времени. */
  actorId: string;
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

const NO_ACTORS: UsageActor[] = [];

export function toActors(dto: BreakdownDto): UsageActor[] {
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
export function withNames(actors: UsageActor[], names: Map<string, string>): UsageActor[] {
  return actors.map((actor) => ({
    ...actor,
    name:
      actor.name ||
      names.get(actor.actorId) ||
      (actor.actorId ? `${actor.actorId.slice(0, 8)}…` : ""),
  }));
}

export type UsageTime = {
  /** Начало 15-минутного ведра, ISO с зоной: `2026-09-05T13:15:00Z`. */
  bucket: string;
  count: number;
  /** Доля от запросов выбранного отправителя, а не от всего месяца. */
  percent: number;
};

/**
 * Когда выбранный отправитель вызывал выбранный маршрут: та же ручка
 * с `group_by=time`. `limit` не передаётся: для времени бэк сам ставит
 * потолок в месяц 15-минутных точек.
 */
export function useUsageTimes(row: UsageRow | null, actor: UsageActor | null, clientOnly: boolean) {
  const projectId = useSession().getProjectId() ?? "";

  const params = {
    group_by: "time",
    method: row?.method ?? "",
    route: row?.route ?? "",
    ...(row?.collection ? { collection: row.collection } : {}),
    ...(actor?.actorId ? { actor_id: actor.actorId } : {}),
    ...(actor?.authType ? { auth_type: actor.authType } : {}),
    ...(clientOnly ? { source: "client" } : {}),
  };

  const query = useQuery({
    queryKey: keys.settings.usageTimes(projectId, params),
    queryFn: () => api.get<BreakdownDto>(BREAKDOWN, { params }),
    enabled: Boolean(projectId && row && actor),
    staleTime: FRESH_FOR,
    select: toTimes,
  });

  return { times: query.data ?? NO_TIMES, isLoading: query.isLoading };
}

const NO_TIMES: UsageTime[] = [];

export function toTimes(dto: BreakdownDto): UsageTime[] {
  return (
    (dto.top ?? [])
      .map((row) => ({
        // Ведро приходит без таймзоны, но оно в UTC — дописываем зону,
        // иначе браузер прочитает его как локальное время и сдвинет ряд.
        bucket: row.bucket ? `${row.bucket.replace(" ", "T")}Z` : "",
        count: row.count ?? 0,
        percent: row.percent ?? 0,
      }))
      .filter((row) => row.bucket)
      // Время читают как ленту: свежее сверху, а не «самое частое сверху».
      .sort((a, b) => b.bucket.localeCompare(a.bucket))
  );
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
