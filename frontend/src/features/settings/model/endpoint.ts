/**
 * Свой эндпоинт: короткий публичный адрес вместо длинного внутреннего.
 *
 * Правило — это пара путей. Запрос, пришедший на `/x-api/...`, шлюз
 * сверяет со списком правил и, если путь совпал, ПОДМЕНЯЕТ его на `to`
 * и обрабатывает дальше как свой (`api.go:997`, `pkg/helper/proxy.go`).
 *
 * Три вещи из разбора шлюза, которые определяют форму:
 *
 * 1. **Совпадение посегментное и по длине.** `len(path) != len(from)` —
 *    правило пропускается. То есть `/x-api/order` и `/x-api/order/12`
 *    это два разных правила, а не одно с необязательным хвостом.
 * 2. **`{имя}` — это подстановка.** Сегмент в фигурных скобках совпадает
 *    с чем угодно, а его значение подставляется в `to` вместо такого же
 *    `{имя}`.
 * 3. **Первое совпавшее правило выигрывает** — отсюда порядок в списке
 *    и кнопки «выше/ниже».
 *
 * Имя, которого нет в `from`, остаётся в адресе КАК ЕСТЬ: запрос уйдёт
 * на путь с фигурными скобками и вернёт 404 — молча, без объяснений.
 * Поэтому такие имена ищутся до сохранения.
 */

/** Единственная точка входа, которая проходит через подмену путей. */
export const ENDPOINT_PREFIX = "/x-api/";

/** Имена подстановок пути: `/v1/object/{slug}` → `["slug"]`. */
export function placeholders(path: string): string[] {
  return path
    .split("/")
    .filter((segment) => segment.startsWith("{") && segment.endsWith("}") && segment.length > 2)
    .map((segment) => segment.slice(1, -1));
}

/**
 * Подстановки, которые в `to` есть, а во `from` взять неоткуда.
 * Пустой список — правило рабочее.
 */
export function unmatchedParams(from: string, to: string): string[] {
  const known = new Set(placeholders(from));
  return placeholders(to).filter((name) => !known.has(name));
}

/** Путь из хвоста, введённого человеком: приставка постоянна и не правится. */
export function joinPath(prefix: string, tail: string): string {
  return prefix + tail.replace(/^\/+/, "").trim();
}

/** Хвост из пути, приехавшего с сервера: обратная сторона joinPath. */
export function splitPath(prefix: string, path: string): string {
  return path.startsWith(prefix) ? path.slice(prefix.length) : path.replace(/^\/+/, "");
}
