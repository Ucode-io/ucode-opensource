/**
 * Хранилище сессии. Access-токен живёт только в памяти — в localStorage
 * его нет, потому что XSS достаёт localStorage, но не замыкание.
 * Refresh-токен в localStorage: без него сессия не переживёт перезагрузку.
 *
 * Environment здесь, а не в URL — см. docs/adr/0001. Project тоже здесь,
 * потому что нужен интерцептору и обновлению токена; в URL он дублируется
 * как читаемый адрес, а не как источник правды.
 */
const REFRESH_KEY = "ucode.refresh";
const ENV_KEY = "ucode.env";
const PROJECT_KEY = "ucode.project";
const PROFILE_KEY = "ucode.profile";
const USER_KEY = "ucode.user";
const PERMISSIONS_KEY = "ucode.permissions";
const GLOBAL_KEY = "ucode.global";

/** Что показывать в шапке сайдбара. Не секрет — только подписи. */
export type Profile = {
  name: string;
  role: string;
  company: string;
  /** Фотография человека. Пусто — в шапке буква имени, как и было. */
  photo?: string;
};

/**
 * Права роли на таблицы: слаг → набор флагов. Что значит каждый флаг —
 * знает features/auth; здесь это просто мешок булевых, потому что
 * shared/ не имеет права знать про фичи.
 *
 * Лежит в localStorage, а не только в памяти. Приезжает в ответе логина
 * и обновления токена, то есть после перезагрузки страницы появляется
 * лишь ПОСЛЕ первого 401 → refresh. Без localStorage панель настроек
 * успевала бы моргнуть «всё запрещено» и вернуться. Это не секрет:
 * решение принимает сервер, здесь только то, что рисовать.
 */
export type PermissionMap = Record<string, Record<string, boolean>>;

let accessToken: string | null = null;

/**
 * Слушатели изменений. Сессия живёт вне React, поэтому компоненты обязаны
 * на неё подписываться — иначе после переключения проекта они продолжат
 * рендериться со старым projectId, и ключи запросов не поменяются.
 *
 * version меняется при каждом изменении: это снимок для useSyncExternalStore.
 */
const listeners = new Set<() => void>();
let version = 0;

const notify = () => {
  version += 1;
  listeners.forEach((fn) => fn());
};

export const session = {
  getAccess: () => accessToken,
  getRefresh: () => localStorage.getItem(REFRESH_KEY),
  getEnvironmentId: () => localStorage.getItem(ENV_KEY),
  getProjectId: () => localStorage.getItem(PROJECT_KEY),

  /**
   * Id пользователя. Нужен как параметр запроса (например, список
   * компаний фильтруется по owner_id).
   *
   * Если он не сохранён — достаём из claim'ов access-токена. Подпись
   * при этом не проверяется, и это допустимо: мы читаем собственную
   * личность для параметра запроса, а не принимаем решение о доступе.
   * Проверку делает сервер. Запасной путь нужен, чтобы уже открытые
   * сессии заработали без повторного входа.
   */
  getUserId(): string {
    return localStorage.getItem(USER_KEY) || readClaim(accessToken, "user_id");
  },

  /**
   * Id пользователя в сервисе АВТОРИЗАЦИИ — не тот же, что getUserId.
   *
   * В токене их два, и они про разное: `user_id` — строка в таблице
   * входа проекта (`admins`, `users`), `user_id_auth` — сам пользователь
   * в auth-сервисе. Ручки `/v2/user/...` знают только второй, и запрос
   * с первым отвечает «no rows in result set». Старая админка зовёт их
   * ровно так же (Account/useAccountProps.jsx:152).
   *
   * Запасной путь на `user_id` — для сессий, выданных до появления
   * второго claim'а: пусть лучше запрос уйдёт со старым идентификатором,
   * чем не уйдёт вовсе.
   */
  getAuthUserId(): string {
    return readClaim(accessToken, "user_id_auth") || this.getUserId();
  },

  /**
   * Тип клиента текущего входа. Живёт только в токене: в ответе логина
   * его нет, а ручки профиля без него отвечают «client type id is an
   * invalid uuid» (auth_service, user_v2.go:191).
   */
  getClientTypeId(): string {
    return readClaim(accessToken, "client_type_id");
  },

  /**
   * «Своя строка» в чужих таблицах: слаг → guid.
   *
   * Вход выдаёт список таблиц аудитории и вместе с ним идентификатор
   * строки, которая и ЕСТЬ вошедший в каждой из них — курьер в таблице
   * курьеров, клиент в таблице клиентов (`session_service_v2.go:1818`).
   * Это [[App Table]] из CONTEXT.md, только со стороны сеанса.
   *
   * Живёт лишь в токене: в ответе логина мы этот список не сохраняем,
   * а шлюз читает его оттуда же (`objectRequest.Data["tables"]`).
   */
  getObjectIds(): Record<string, string> {
    const tables = readClaimValue(accessToken, "tables");
    if (!Array.isArray(tables)) return {};

    const ids: Record<string, string> = {};
    for (const table of tables) {
      if (typeof table !== "object" || table === null) continue;
      const { table_slug: slug, object_id: id } = table as Record<string, unknown>;
      if (typeof slug === "string" && typeof id === "string" && slug && id) ids[slug] = id;
    }

    return ids;
  },

  getProfile(): Profile | null {
    const raw = localStorage.getItem(PROFILE_KEY);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as Profile;
    } catch {
      // Испорченное значение — не повод падать: покажем пустую шапку.
      return null;
    }
  },

  setProfile(profile: Profile) {
    localStorage.setItem(PROFILE_KEY, JSON.stringify(profile));
    notify();
  },

  getPermissions(): PermissionMap {
    const raw = localStorage.getItem(PERMISSIONS_KEY);
    if (!raw) return {};
    try {
      return JSON.parse(raw) as PermissionMap;
    } catch {
      return {};
    }
  },

  setPermissions(permissions: PermissionMap) {
    localStorage.setItem(PERMISSIONS_KEY, JSON.stringify(permissions));
    notify();
  },

  /**
   * Глобальные права роли — не про таблицы, а про кнопки приложения
   * (`menu_button`, `menu_drag`, …). Приезжают тем же ответом логина.
   *
   * `null` — записи не было вовсе. Это «не знаем», а не «нельзя»:
   * поля в ответе с `omitempty`, и отличить `false` от отсутствия можно
   * только по наличию самого объекта.
   */
  getGlobalRights(): Record<string, unknown> | null {
    const raw = localStorage.getItem(GLOBAL_KEY);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as Record<string, unknown>;
    } catch {
      return null;
    }
  },

  setGlobalRights(rights: Record<string, unknown>) {
    localStorage.setItem(GLOBAL_KEY, JSON.stringify(rights));
    notify();
  },

  /** Есть чем восстановиться после перезагрузки. */
  isAuthenticated: () => accessToken !== null || localStorage.getItem(REFRESH_KEY) !== null,

  /** Версия для useSyncExternalStore: меняется при любом изменении сессии. */
  getVersion: () => version,

  /**
   * Всё, что меняется вместе, записывается одним вызовом. Иначе между
   * записью токена и записью проекта случится промежуточный рендер,
   * где окружение уже новое, а проект ещё старый.
   */
  set(tokens: {
    access: string;
    refresh: string;
    environmentId?: string;
    projectId?: string;
    userId?: string;
  }) {
    accessToken = tokens.access;
    localStorage.setItem(REFRESH_KEY, tokens.refresh);
    if (tokens.environmentId) localStorage.setItem(ENV_KEY, tokens.environmentId);
    if (tokens.projectId) localStorage.setItem(PROJECT_KEY, tokens.projectId);
    if (tokens.userId) localStorage.setItem(USER_KEY, tokens.userId);
    notify();
  },

  clear() {
    accessToken = null;
    localStorage.removeItem(REFRESH_KEY);
    localStorage.removeItem(ENV_KEY);
    localStorage.removeItem(PROJECT_KEY);
    localStorage.removeItem(PROFILE_KEY);
    localStorage.removeItem(USER_KEY);
    localStorage.removeItem(PERMISSIONS_KEY);
    localStorage.removeItem(GLOBAL_KEY);
    notify();
  },

  subscribe(fn: () => void) {
    listeners.add(fn);
    return () => void listeners.delete(fn);
  },
};

/** Читает одно поле из полезной нагрузки JWT. Подпись не проверяется. */
function readClaim(token: string | null, name: string): string {
  const value = readClaimValue(token, name);
  return typeof value === "string" ? value : "";
}

/** То же, но значением любой формы: в токене бывают и списки. */
function readClaimValue(token: string | null, name: string): unknown {
  if (!token) return undefined;

  try {
    const payload = token.split(".")[1];
    if (!payload) return undefined;

    const json = atob(payload.replace(/-/g, "+").replace(/_/g, "/"));
    return (JSON.parse(json) as Record<string, unknown>)[name];
  } catch {
    return undefined;
  }
}
