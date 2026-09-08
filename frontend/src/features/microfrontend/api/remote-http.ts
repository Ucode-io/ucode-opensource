import axios, {
  type AxiosInstance,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from "axios";
import { refreshOnce, unwrap } from "@/shared/api/client";
import { session } from "@/shared/api/session";

/**
 * Оси http, которые уезжают ремоуту пропсами `sharedHttpRequest`
 * и `sharedHttpRequestV2`.
 *
 * Это НЕ наши `http`/`httpAuth`, и передавать их напрямую нельзя —
 * ремоуты написаны под контракт старой админки
 * (`utils/httpsRequest.js`, `utils/httpsRequestV2.js`), а он другой
 * в трёх местах:
 *
 * 1. **V2 — это тот же admin-хост с `/v2`,** а не сервис авторизации
 *    (`baseURL = VITE_BASE_URL + "/v2"`). Мы передавали `httpAuth`,
 *    то есть слали запросы данных на `api.auth`.
 * 2. **Наружу отдаётся ПОЛЕЗНАЯ ЧАСТЬ, а не ответ axios.** Старый
 *    перехватчик возвращает `response.data.data`, поэтому ремоут пишет
 *    `const rows = await sharedHttpRequestV2.get(...)`, без `.data`.
 * 3. **`project-id` уезжает параметром запроса в КАЖДОМ запросе.**
 *    Часть ручек шлюза читает проект только оттуда.
 *
 * Что НЕ повторяем — обновление токена. Оно у нас одно на приложение
 * (`refreshOnce`), и второй стек обновления — ровно та ошибка, из-за
 * которой в старой админке случались случайные разлогины: восемь
 * инстансов, четыре копии логики, пять параллельных 401.
 *
 * Заголовка `resource-id` тоже нет: вся новая админка живёт без него,
 * и шлюз берёт ресурс окружения по умолчанию. Понадобится ремоуту
 * не тот ресурс — это будет видно по ответу, а не по молчанию.
 */

const V1 = import.meta.env.VITE_API_URL;
const V2 = `${import.meta.env.VITE_API_URL}/v2`;

function createRemoteClient(baseURL: string): AxiosInstance {
  // Тот же timeout, что в старой админке: отчёты ремоутов бывают долгими.
  const instance = axios.create({ baseURL, timeout: 100_000 });

  instance.interceptors.request.use((config: InternalAxiosRequestConfig) => {
    const access = session.getAccess();
    const environmentId = session.getEnvironmentId();

    if (access) config.headers.set("Authorization", `Bearer ${access}`);
    if (environmentId) config.headers.set("Environment-Id", environmentId);

    const params = (config.params ?? {}) as Record<string, unknown>;
    params["project-id"] ??= session.getProjectId() ?? "";
    config.params = params;

    return config;
  });

  instance.interceptors.response.use(
    /*
     * Наружу уходит полезная часть, а не ответ axios: так делает
     * старый `httpsRequestV2`, и так это читает ремоут. Типам axios
     * такое неизвестно — отсюда приведение, и оно здесь честнее,
     * чем притворяться, что мы отдаём `AxiosResponse`.
     *
     * Выгрузки ремоут просит блобом — там разворачивать нечего.
     */
    (response) =>
      (response.config.responseType === "blob"
        ? response.data
        : unwrap(response.data)) as unknown as AxiosResponse,
    async (error: unknown) => {
      if (!axios.isAxiosError(error) || !error.response) throw error;

      const config = error.config as (InternalAxiosRequestConfig & { _retried?: boolean }) | undefined;

      if (error.response.status === 401 && config && !config._retried) {
        config._retried = true;
        if (await refreshOnce()) return instance.request(config);
      }

      /*
       * Отказ — ответом axios, а не нашим ApiError: ремоуты читают
       * из него `err.data.data` и `err.status`, как отдавала старая
       * админка (`Promise.reject(error.response)`). Наша форма ошибки
       * тут была бы правильнее и бесполезнее.
       */
      throw error.response;
    },
  );

  return instance;
}

/** `sharedHttpRequest` ремоута: admin-хост без версии в адресе. */
export const remoteHttp = createRemoteClient(V1);

/** `sharedHttpRequestV2` ремоута: тот же хост и `/v2`. */
export const remoteHttpV2 = createRemoteClient(V2);
