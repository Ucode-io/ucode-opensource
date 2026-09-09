/**
 * Адреса, заданные в настройках view: куда уводит щелчок по строке,
 * куда ведёт «новая запись» и где лежит PDF записи.
 *
 * Это способ встроить в админку чужой экран: у проекта есть своя
 * страница заказа — щелчок по строке должен открывать её, а не карточку.
 *
 * Адрес — шаблон: `{{$slug}}` подставляется значением поля строки.
 * Такая же запись у параметров запроса, поэтому подстановка одна на всё.
 */

/** Адрес и его параметры запроса. Пустой url — адрес не задан. */
export type UrlTemplate = {
  url: string;
  /** Пары «ключ=значение» для строки запроса. Оба конца — шаблоны. */
  params: { key: string; value: string }[];
};

export const EMPTY_URL_TEMPLATE: UrlTemplate = { url: "", params: [] };

/** Задан ли адрес. Одни параметры без адреса никуда не ведут. */
export const hasUrl = (template: UrlTemplate | null | undefined): boolean =>
  Boolean(template?.url.trim());

const VARIABLE = /\{\{\$([\w.]+)\}\}/g;

/**
 * Шаблон → строка. Неизвестная переменная превращается в пустоту, а не
 * остаётся в адресе: `/order/{{$guid}}` без guid — это ссылка на страницу
 * «не найдено», и лучше уж короткий адрес, чем адрес с фигурными скобками.
 *
 * Значения кодируются: в поле лежит текст, который человек набрал руками,
 * и первый же пробел или амперсанд разломал бы адрес.
 */
export function fillTemplate(template: string, row: Record<string, unknown>): string {
  return template.replace(VARIABLE, (_, name: string) => {
    const value = row[name];
    return value === undefined || value === null ? "" : encodeURIComponent(String(value));
  });
}

/**
 * Полный адрес со строкой запроса. Параметр без ключа отбрасывается:
 * это строка, которую добавили и не заполнили.
 */
export function fillUrl(template: UrlTemplate, row: Record<string, unknown>): string {
  const url = fillTemplate(template.url.trim(), row);

  const query = template.params
    .filter((param) => param.key.trim())
    .map((param) => `${fillTemplate(param.key.trim(), row)}=${fillTemplate(param.value, row)}`)
    .join("&");

  if (!query) return url;

  // У адреса уже может быть свой знак вопроса: `/report?year=2026`.
  return `${url}${url.includes("?") ? "&" : "?"}${query}`;
}

/**
 * Внешний адрес открывается новой вкладкой, внутренний — переходом
 * внутри приложения. Разделение по протоколу, а не по «начинается со
 * слэша»: `//example.com` — тоже чужой сайт.
 */
export const isExternal = (url: string): boolean => /^(https?:)?\/\//i.test(url.trim());

/**
 * Переход по адресу из настроек.
 *
 * Чужой сайт открывается новой вкладкой, свой — заменяет страницу.
 * `noopener` обязателен: без него открытая страница получает доступ
 * к нашему window через opener.
 */
export function openUrl(url: string): void {
  if (!url) return;

  if (isExternal(url)) window.open(url, "_blank", "noopener,noreferrer");
  else window.location.assign(url);
}

/**
 * Открыть строку так, как задал админ (`attributes.navigate`).
 * Вернул true — переход состоялся, и карточку открывать не нужно.
 */
export function openRowUrl(
  view: { navigate: UrlTemplate } | undefined,
  row: Record<string, unknown> | undefined,
): boolean {
  if (!view || !row || !hasUrl(view.navigate)) return false;

  openUrl(fillUrl(view.navigate, row));
  return true;
}

/**
 * Завести запись так, как задал админ (`attributes.url_object`).
 * Вернул true — ушли на страницу проекта, черновик заводить не нужно.
 *
 * Подставлять в адрес нечего: строки ещё нет. Значения, которыми экран
 * заполнил бы черновик — колонка доски, даты из клетки календаря, —
 * до чужой страницы не доедут; так же было и в старой админке
 * (useViewsProps.jsx:300, navigateCreatePage: адрес получает только свои
 * параметры, а значения уходили в собственный drawer).
 *
 * Одна функция на все экраны, а не ветка внутри каждого: настройка,
 * про которую помнит только таблица, — это настройка, которой нет.
 */
export function openCreateUrl(view: { objectUrl: UrlTemplate } | undefined): boolean {
  if (!view || !hasUrl(view.objectUrl)) return false;

  openUrl(fillUrl(view.objectUrl, {}));
  return true;
}

/**
 * Сырые attributes → адрес.
 *
 * Читается и объект `{url, params}`, и голая строка: настройки старой
 * админки пишут объект (ViewSettingsModal), а её же экраны читают
 * `attributes.url_object` как строку и передают объект в navigate().
 * У кого-то в проектах поэтому лежит и то, и другое.
 */
export function toUrlTemplate(value: unknown): UrlTemplate {
  if (typeof value === "string") return { url: value, params: [] };
  if (typeof value !== "object" || value === null) return EMPTY_URL_TEMPLATE;

  const raw = value as { url?: unknown; params?: unknown };

  return {
    url: typeof raw.url === "string" ? raw.url : "",
    params: Array.isArray(raw.params)
      ? raw.params
          .filter((item): item is Record<string, unknown> => typeof item === "object" && item !== null)
          .map((item) => ({
            key: typeof item["key"] === "string" ? item["key"] : "",
            value: typeof item["value"] === "string" ? item["value"] : "",
          }))
      : [],
  };
}
