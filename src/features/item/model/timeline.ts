import { addDays, daysBetween, startOfDay, startOfWeek, type CalendarEvent } from "./calendar";

/**
 * Таймлайн: те же строки, но каждая — своя полоса на общей оси дней.
 *
 * Своих запросов нет и здесь: это обычный get-list с отбором по видимому
 * диапазону — тот же, что у календаря. Разница только в раскладке:
 * в календаре день это клетка, в которую попадают все события, а здесь
 * день это КОЛОНКА, и строк столько же, сколько записей.
 *
 * Поэтому и считать почти нечего: у полосы нет соседей, с которыми она
 * могла бы разойтись строками, — своя строка есть у каждой. Расчёт один:
 * с какой колонки полоса начинается и сколько их занимает.
 */

/** Масштабы оси. Порядок — как в переключателе, от крупного к мелкому. */
export const TIMELINE_SCALES = ["DAY", "WEEK", "MONTH"] as const;

export type TimelineScale = (typeof TIMELINE_SCALES)[number];

/** Неизвестное значение — DAY: с колонки в двенадцать пикселей начинать нечего. */
export function toScale(value: string | undefined): TimelineScale {
  return TIMELINE_SCALES.find((scale) => scale === value) ?? "DAY";
}

/**
 * Ширина колонки дня, пиксели. Единственное, что меняет масштаб.
 *
 * Единица оси всегда сутки: месяц — это не другая сетка, а та же,
 * сжатая до обзора квартала. Иначе у полосы, начатой 3-го, пропал бы
 * край — тянуть её стало бы не за что.
 */
export const COLUMN_WIDTH: Record<TimelineScale, number> = { DAY: 48, WEEK: 24, MONTH: 12 };

/** Дни оси, включая последний. */
export function timelineDays(from: Date, to: Date): Date[] {
  const first = startOfDay(from);
  const count = daysBetween(first, to) + 1;

  return Array.from({ length: Math.max(count, 0) }, (_, index) => addDays(first, index));
}

/** Полоса события на оси: с какой колонки, сколько колонок, где обрезана. */
export type TimelineBar = {
  /** Колонка начала, 0 — первый день оси. */
  start: number;
  /** Сколько колонок занимает, минимум одна. */
  span: number;
  /** Событие началось до оси / кончится после неё. */
  clippedStart: boolean;
  clippedEnd: boolean;
};

/**
 * Куда встанет событие на оси из `count` дней, начатой днём `first`.
 *
 * null — событие целиком за краями оси: рисовать нечего. Обрезанный край
 * помечен, и ручки ему не рисуют — настоящий край лежит за экраном,
 * и тянуть надо там.
 */
export function timelineBar(event: CalendarEvent, first: Date, count: number): TimelineBar | null {
  const from = daysBetween(first, event.from);
  const to = daysBetween(first, event.to);
  if (count <= 0 || to < 0 || from > count - 1) return null;

  const start = Math.max(from, 0);
  const end = Math.min(to, count - 1);

  return {
    start,
    span: end - start + 1,
    clippedStart: from < 0,
    clippedEnd: to > count - 1,
  };
}

/** Отрезок оси под одной подписью: с какого дня и сколько колонок. */
export type AxisGroup = { first: Date; span: number };

/**
 * Месяцы над осью: подпись раз на месяц, а не над каждым днём.
 *
 * Число помещается не в каждую колонку, и без этой полосы ось
 * превращается в безымянную линейку.
 */
export function monthGroups(days: Date[]): AxisGroup[] {
  return axisGroups(
    days,
    (a, b) => a.getMonth() === b.getMonth() && a.getFullYear() === b.getFullYear(),
  );
}

/**
 * Недели над осью — подпись масштаба WEEK.
 *
 * Тот же приём, что и в старой админке: колонки остаются днями,
 * а подпись сверху меняется с месяца на неделю (TimeLineDatesRow.jsx:96,
 * `computedWeekList`). Это и есть весь смысл среднего масштаба: видно
 * квартал целиком, но границы недель никуда не деваются.
 */
export function weekGroups(days: Date[]): AxisGroup[] {
  return axisGroups(days, (a, b) => startOfWeek(a).getTime() === startOfWeek(b).getTime());
}

/** Подряд идущие дни, у которых подпись одна. */
function axisGroups(days: Date[], together: (a: Date, b: Date) => boolean): AxisGroup[] {
  const groups: AxisGroup[] = [];

  for (const day of days) {
    const last = groups[groups.length - 1];

    if (last && together(last.first, day)) last.span += 1;
    else groups.push({ first: day, span: 1 });
  }

  return groups;
}

/**
 * День под курсором: смещение от левого края оси → её день.
 *
 * Обрезается краями, а не отдаёт null: указатель уводят и за пределы
 * оси — отпускать событие в пустоту нельзя, оно должно встать на
 * последний день, который видно.
 */
export function dayAt(days: Date[], offset: number, width: number): Date | null {
  if (!days.length || width <= 0) return null;

  const index = Math.min(Math.max(Math.floor(offset / width), 0), days.length - 1);
  return days[index] ?? null;
}
