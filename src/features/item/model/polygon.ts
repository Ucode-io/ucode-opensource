import type { Coords } from "./coords";

/**
 * Значение поля POLYGON — область на карте.
 *
 * В базе это строка с JSON: `[[41.31,69.24],[41.32,69.25],...]`, пара —
 * `[широта, долгота]` (порядок Яндекс-карт, на которых область рисовали).
 *
 * Форм в данных две, и обе настоящие: старая админка сохраняла
 * координаты геометрии Яндекса как есть, а у полигона это список
 * КОНТУРОВ — `[[[lat,lon],...]]`. Поэтому разбираем и плоский список
 * точек, и вложенный; внутренние контуры (дырки) отбрасываем — рисовать
 * их всё равно нечем, а внешний контур и есть область.
 */
export function parsePolygon(value: unknown): Coords[] {
  const data = typeof value === "string" ? safeParse(value) : value;
  if (!Array.isArray(data) || data.length === 0) return [];

  const first = data[0];
  const ring: unknown[] = Array.isArray(first) && Array.isArray(first[0]) ? first : data;

  const points: Coords[] = [];

  for (const pair of ring) {
    if (!Array.isArray(pair)) continue;

    const lat = Number(pair[0]);
    const lon = Number(pair[1]);

    // Точка вне Земли — это не точка: в колонке POLYGON лежит varchar,
    // и туда попадало всякое.
    if (!Number.isFinite(lat) || !Number.isFinite(lon)) continue;
    if (Math.abs(lat) > 90 || Math.abs(lon) > 180) continue;

    points.push({ lat, lon });
  }

  return points;
}

function safeParse(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

/**
 * Центр области — середина её рамки, а не центр тяжести. Он нужен ровно
 * для ссылки «показать на карте», и разница между ними там незаметна.
 */
export function polygonCenter(points: Coords[]): Coords | null {
  if (!points.length) return null;

  const lats = points.map((point) => point.lat);
  const lons = points.map((point) => point.lon);

  return {
    lat: round((Math.min(...lats) + Math.max(...lats)) / 2),
    lon: round((Math.min(...lons) + Math.max(...lons)) / 2),
  };
}

/** Шесть знаков — примерно метр. Больше в ссылке на карту бессмысленно. */
function round(value: number): number {
  return Math.round(value * 1e6) / 1e6;
}

/**
 * Точки для `<polygon points>`: область вписана в квадрат `size`.
 *
 * Рамка растягивается по большей стороне, поэтому форма не плющится,
 * а маленькая область не превращается в точку. Искажение по долготе
 * (у полюсов градус короче) не поправляем — это миниатюра фигуры,
 * а не карта.
 */
export function polygonPoints(points: Coords[], size: number, padding = 1): string {
  if (!points.length) return "";

  const lats = points.map((point) => point.lat);
  const lons = points.map((point) => point.lon);

  const minLat = Math.min(...lats);
  const maxLat = Math.max(...lats);
  const minLon = Math.min(...lons);
  const maxLon = Math.max(...lons);

  const box = size - padding * 2;
  // Одна точка или прямая линия: делить на ноль нечем, рисуем в центре.
  const span = Math.max(maxLat - minLat, maxLon - minLon) || 1;
  const scale = box / span;

  const offsetX = padding + (box - (maxLon - minLon) * scale) / 2;
  const offsetY = padding + (box - (maxLat - minLat) * scale) / 2;

  return points
    .map((point) => {
      const x = offsetX + (point.lon - minLon) * scale;
      // Широта растёт на север, координата svg — вниз.
      const y = offsetY + (maxLat - point.lat) * scale;

      return `${trim(x)},${trim(y)}`;
    })
    .join(" ");
}

function trim(value: number): number {
  return Math.round(value * 100) / 100;
}
