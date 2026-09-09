/**
 * Значение поля MAP — строка «широта,долгота».
 *
 * Так его пишет и читает ucode: тип MAP лежит в базе как VARCHAR, и
 * старая админка разбирала его тем же `split(",")`. Ни объекта, ни GeoJSON
 * там не бывает — поэтому и здесь строка, а не своя структура.
 */
export type Coords = { lat: number; lon: number };

/** Широта и долгота Земли, а не любые два числа: «200,500» — не точка. */
const LIMITS = { lat: 90, lon: 180 };

export function parseCoords(value: unknown): Coords | null {
  if (typeof value !== "string" && typeof value !== "number") return null;

  const [rawLat, rawLon, extra] = String(value).split(",");
  if (extra !== undefined || rawLat === undefined || rawLon === undefined) return null;

  const lat = Number(rawLat.trim());
  const lon = Number(rawLon.trim());

  const valid =
    rawLat.trim() !== "" &&
    rawLon.trim() !== "" &&
    Number.isFinite(lat) &&
    Number.isFinite(lon) &&
    Math.abs(lat) <= LIMITS.lat &&
    Math.abs(lon) <= LIMITS.lon;

  return valid ? { lat, lon } : null;
}

/**
 * Обратно в строку. Пустые поля дают пустое значение, а не «,» —
 * иначе очистка координат превращалась бы в мусор в колонке.
 */
export function formatCoords(lat: string, lon: string): string {
  const left = lat.trim();
  const right = lon.trim();

  return left && right ? `${left},${right}` : "";
}

/**
 * Ссылка на карту. Яндекс — как в старой админке: у неё осмысленный
 * ответ на «широта,долгота» без ключа API и без единого запроса с нашей
 * стороны, поэтому карта не встраивается, а открывается по ссылке.
 */
export function mapLink({ lat, lon }: Coords): string {
  return `https://yandex.com/maps/?text=${encodeURIComponent(`${lat},${lon}`)}`;
}
