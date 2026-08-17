/**
 * Какие номера страниц показать: первая, последняя, окно вокруг текущей,
 * многоточие вместо пропуска. 1 2 3 4 5 … 16, как в референсе.
 *
 * Чистая функция и отдельный файл, потому что это единственная логика
 * во всей пагинации, а ошибка здесь тихая: полоса просто рисует не те
 * номера, и заметить это можно только посчитав.
 */
export const GAP = "gap";

export type PageItem = number | typeof GAP;

/**
 * Ширина полосы постоянна — семь мест. Иначе кнопки разъезжаются под
 * курсором при каждом переходе: «1 2 3 4 5 … 16» и «1 … 8 … 16» имеют
 * разную длину, и человек промахивается мимо «вперёд».
 */
const SLOTS = 7;

export function pageItems(current: number, total: number): PageItem[] {
  if (total < 1) return [];
  if (total <= SLOTS) return range(1, total);

  const page = clamp(current, 1, total);

  // Пропуск слева нужен, только когда за ним прячется больше одной
  // страницы: «1 … 3» занимает столько же места, сколько «1 2 3».
  const gapLeft = page > 4;
  const gapRight = page < total - 3;

  // У края окно не центрируется, а прижимается: у первой страницы
  // соседей слева нет, и место отдаётся правым.
  if (!gapLeft) return [...range(1, SLOTS - 2), GAP, total];
  if (!gapRight) return [1, GAP, ...range(total - (SLOTS - 3), total)];

  return [1, GAP, page - 1, page, page + 1, GAP, total];
}

/** Сколько всего страниц. Ноль строк — одна пустая страница, а не ноль. */
export function pageCount(total: number, limit: number): number {
  if (limit < 1) return 1;
  return Math.max(1, Math.ceil(total / limit));
}

function range(from: number, to: number): number[] {
  return Array.from({ length: to - from + 1 }, (_, index) => from + index);
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}
