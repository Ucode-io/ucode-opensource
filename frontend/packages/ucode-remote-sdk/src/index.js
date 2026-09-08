export { meta } from "../meta.js";

/**
 * Клиент API поверх осей, которые передал хост.
 *
 * Это НЕ новый http-клиент: `sharedHttpRequest` и `sharedHttpRequestV2`
 * уже несут токен, `Environment-Id`, `project-id` и общее с админкой
 * обновление токена. Обёртка нужна ровно за тем, чтобы не заводить
 * своё — и чтобы не выяснять отладкой, что ответ уже развёрнут:
 * конверт бэкенда (`{status, description, data}`) хост снимает сам,
 * наружу приходит полезная часть.
 *
 * @param {{ sharedHttpRequest?: unknown, sharedHttpRequestV2?: unknown }} props
 *   пропсы, которые хост передал странице.
 */
export function createApi(props) {
  const v1 = props?.sharedHttpRequest;
  const v2 = props?.sharedHttpRequestV2;

  // На экране входа осей нет вовсе — сессии ещё не существует.
  // Возвращаем заглушку, чтобы общий код не падал на её отсутствии.
  const call = (axis, method) => (url, ...rest) => {
    if (!axis) return Promise.reject(new Error("ucode: http доступен только после входа"));
    return axis[method](url, ...rest);
  };

  const methods = (axis) => ({
    get: call(axis, "get"),
    post: call(axis, "post"),
    put: call(axis, "put"),
    patch: call(axis, "patch"),
    delete: call(axis, "delete"),
  });

  return { ...methods(v1), v2: methods(v2) };
}
