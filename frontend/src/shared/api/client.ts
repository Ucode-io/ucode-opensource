import axios, {
  type AxiosInstance,
  type AxiosRequestConfig,
  type InternalAxiosRequestConfig,
} from "axios";
import { session } from "./session";

/**
 * Два хоста — admin и auth, но перехватчики общие. В старом ucode было
 * восемь axios-инстансов (по хосту × версии API), и логика refresh была
 * скопирована в четыре из них. Отсюда случайные разлогины: пять
 * параллельных 401 давали пять refresh, токен ротировался, четыре запроса
 * получали мёртвый токен.
 *
 * Версия API здесь в пути запроса ("/v2/login"), а не в baseURL —
 * иначе инстансы снова начнут размножаться.
 */

/** Стабильная форма ошибки. Наружу axios не протекает. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly body: unknown,
  ) {
    super(`HTTP ${status}`);
    this.name = "ApiError";
  }
}

/**
 * Что показать человеку. null — сказать нечего, и вызывающий подставит
 * свою формулировку.
 *
 * Бэкенд кладёт причину прямо в `data` строкой: «this table is auth
 * table. Auth information not fully given», «not found». Формулировки
 * английские и техничные, но это админка: конкретная причина полезнее
 * вежливого «что-то пошло не так», по которому нельзя понять даже,
 * твоя это ошибка или чужая.
 */
export function errorText(error: unknown): string | null {
  if (!(error instanceof ApiError)) return null;

  if (typeof error.body === "string" && error.body.trim()) return error.body.trim();

  // Иногда причина завёрнута ещё раз: {message} или {description}.
  if (typeof error.body === "object" && error.body !== null) {
    const body = error.body as { message?: unknown; description?: unknown };
    const text = body.message ?? body.description;
    if (typeof text === "string" && text.trim()) return text.trim();
  }

  return null;
}

/** Общий промис обновления токена — один на оба инстанса. */
let refreshing: Promise<boolean> | null = null;

/** Ставится фичей auth при старте приложения. */
let refreshTokens: () => Promise<boolean> = async () => false;
export function setRefreshHandler(fn: () => Promise<boolean>) {
  refreshTokens = fn;
}

export function refreshOnce(): Promise<boolean> {
  refreshing ??= refreshTokens().finally(() => {
    refreshing = null;
  });
  return refreshing;
}

/**
 * Восстанавливает access-токен, если его нет. Access живёт только в памяти,
 * поэтому после перезагрузки страницы его нужно получить заново — до
 * первого запроса, а не после ошибки.
 *
 * Без этого первый запрос уходит без Authorization, а шлюз отвечает на
 * такой 403 (не 401), и обычная логика обновления не срабатывает.
 */
export async function ensureAccessToken(): Promise<boolean> {
  if (session.getAccess()) return true;
  if (!session.getRefresh()) return false;
  return refreshOnce();
}

/**
 * Сам запрос на обновление токена обязан проходить мимо этой логики.
 * Иначе при отказе он получает 401, зовёт refreshOnce() — а тот уже
 * выполняется и возвращает промис, которого этот же запрос и ждёт.
 * Взаимоблокировка: приложение висит с пустым экраном.
 */
declare module "axios" {
  interface AxiosRequestConfig {
    skipAuthRefresh?: boolean;
  }
}

/** Помечает конфиг, чтобы повторить запрос ровно один раз. */
type Retriable = InternalAxiosRequestConfig & { _retried?: boolean };

/**
 * Бэкенд заворачивает каждый ответ в конверт (api/http/response.go):
 *   { status, description, data }
 * Полезная нагрузка — в data. Разворачиваем здесь один раз, чтобы слово
 * "data" не тянулось через все вызовы как data.data.response.
 *
 * Разворачиваем только настоящий конверт, а не всё подряд: иначе ответ
 * без конверта молча превратится в undefined.
 */
type Envelope = { status: unknown; description?: unknown; data: unknown };

function isEnvelope(body: unknown): body is Envelope {
  return (
    typeof body === "object" &&
    body !== null &&
    "status" in body &&
    "data" in body &&
    typeof (body as Envelope).status === "string"
  );
}

export function unwrap(body: unknown): unknown {
  return isEnvelope(body) ? body.data : body;
}

function createClient(baseURL: string): AxiosInstance {
  const instance = axios.create({
    baseURL,
    headers: { "Content-Type": "application/json" },
  });

  instance.interceptors.request.use((config: InternalAxiosRequestConfig) => {
    const access = session.getAccess();
    const environmentId = session.getEnvironmentId();

    if (access) config.headers.set("Authorization", `Bearer ${access}`);
    // Только этот вариант написания. В v1-мидлваре бэка есть ещё
    // environment_id — мы на нём не живём.
    if (environmentId) config.headers.set("Environment-Id", environmentId);

    return config;
  });

  instance.interceptors.response.use(
    (response) => {
      response.data = unwrap(response.data);
      return response;
    },
    async (error: unknown) => {
      // isAxiosError, а не instanceof: instanceof ломается, когда в дереве
      // зависимостей оказывается вторая копия axios.
      if (!axios.isAxiosError(error) || !error.response) throw error;

      /*
       * 401 — токен просрочен, обновляем.
       *
       * 403 — только если токена не было вовсе: шлюз отвечает так на
       * запрос без заголовка Authorization (middleware.go:38). Если токен
       * есть, 403 означает «нет прав на этот эндпоинт», и обновлять
       * нечего — раньше такой ответ запускал лишнее обновление, а сбой
       * в нём выкидывал из системы.
       */
      const config = error.config as Retriable | undefined;
      const status = error.response.status;

      const authFailed =
        !config?.skipAuthRefresh &&
        (status === 401 || (status === 403 && !session.getAccess()));

      if (authFailed && config && !config._retried) {
        config._retried = true;
        if (await refreshOnce()) return instance.request(config);
      }

      throw new ApiError(error.response.status, unwrap(error.response.data));
    },
  );

  return instance;
}

/** Основной API: таблицы, записи, view. */
export const http = createClient(import.meta.env.VITE_API_URL);

/** Отдельный хост авторизации. */
export const httpAuth = createClient(import.meta.env.VITE_AUTH_URL);

const methods = (instance: AxiosInstance) => ({
  get: <T>(url: string, config?: AxiosRequestConfig) =>
    instance.get<T>(url, config).then((r) => r.data),
  post: <T>(url: string, data?: unknown, config?: AxiosRequestConfig) =>
    instance.post<T>(url, data, config).then((r) => r.data),
  put: <T>(url: string, data?: unknown, config?: AxiosRequestConfig) =>
    instance.put<T>(url, data, config).then((r) => r.data),
  delete: <T>(url: string, config?: AxiosRequestConfig) =>
    instance.delete<T>(url, config).then((r) => r.data),
});

export const api = methods(http);
export const authApi = methods(httpAuth);
