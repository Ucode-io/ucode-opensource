import type { Field } from "@/features/table";
import { cellKind } from "./cell-kind";
import { fromDateInput, toDateInput, toDayInput, type DateKind } from "./cell-value";
import type { Item } from "./types";

/**
 * Календарь: те же строки, разложенные по дням и часам.
 *
 * Своих запросов у календаря нет — это обычный get-list с отбором
 * по видимому диапазону. Здесь только расчёт: какие дни показать,
 * в какую клетку попадает строка и как разложить пересекающиеся
 * события внутри дня.
 *
 * Всё считается в МЕСТНОМ времени, в одной системе координат. Значения
 * приходят в трёх временных типах с разным смыслом (см. cell-value),
 * и смешивать их на одной сетке нельзя: DATE — это календарная дата
 * без пояса, DATE_TIME — момент. Поэтому значение сначала приводится
 * к «настенным часам» — к тому, что человек ВИДИТ в ячейке, — и дальше
 * календарь работает только с ними. Обратно в строку оно уезжает тем же
 * путём в обратную сторону, поэтому перенос события не сдвигает время
 * на часовой пояс.
 */

/** Режимы календаря. Порядок — как в переключателе. */
export const CALENDAR_PERIODS = ["MONTH", "WEEK", "DAY"] as const;

export type CalendarPeriod = (typeof CALENDAR_PERIODS)[number];

/** Неизвестное значение — MONTH: с него календарь открывается по умолчанию. */
export function toPeriod(value: string | undefined): CalendarPeriod {
  return CALENDAR_PERIODS.find((period) => period === value) ?? "MONTH";
}

/**
 * Шаг сетки в DAY и WEEK, минуты.
 *
 * Постоянный, хотя настройка есть: `view.time_interval` лежит в базе,
 * но запрос списка view его не выбирает вовсе (view.go, GetAll —
 * этой колонки нет в SELECT), а обновить его не умеет ни одна ручка.
 * То есть прочитать значение неоткуда и задать его нечем. См.
 * docs/backend-notes.md.
 */
export const SLOT_MINUTES = 60;

const MINUTES_PER_DAY = 24 * 60;

/**
 * Насколько «толстым» считается событие без длительности при раскладке
 * пересечений. Две встречи, начатые в одну минуту, обязаны встать
 * соседними колонками, а не одна поверх другой.
 */
const POINT_MINUTES = 15;

/** Событие: строка и её отрезок в настенных часах. */
export type CalendarEvent = {
  row: Item;
  /** Начало. Всегда есть — без него события нет вовсе. */
  from: Date;
  /**
   * Конец. Равен началу, когда поля конца нет или оно не заполнено:
   * такое событие — точка в дне, а не полоса на сутки.
   */
  to: Date;
  /** Поле хранит только дату, без времени: место в сутках неизвестно. */
  allDay: boolean;
};

/**
 * Значение поля → местная дата с теми частями, которые человек видит
 * в ячейке.
 *
 * Через toDateInput, а не через new Date: у значений без пояса части
 * лежат в UTC (см. cell-value), и брать их местными — это сдвиг даты
 * на часовой пояс у половины планеты.
 */
export function toWallClock(value: unknown, kind: DateKind): Date | null {
  const text = toDateInput(value, kind);
  if (!text) return null;

  const [day = "", time = "00:00"] = text.split("T");
  const [year, month, date] = day.split("-").map(Number);
  const [hour, minute] = time.split(":").map(Number);

  if (!year || !month || !date) return null;

  return new Date(year, month - 1, date, hour ?? 0, minute ?? 0);
}

/** Обратно: местная дата → то, что примет бэкенд для этого типа поля. */
export function fromWallClock(date: Date, kind: DateKind): string | null {
  const day = toDayInput(date);
  if (kind === "date") return fromDateInput(day, kind);

  return fromDateInput(`${day}T${pad(date.getHours())}:${pad(date.getMinutes())}`, kind);
}

/**
 * Даты события — значениями полей, готовыми к отправке.
 *
 * Каждое приводится к типу СВОЕГО поля: начало бывает датой без времени,
 * а конец — моментом, и общего формата у них нет. Одна на всех, кто
 * пишет даты: и календарь, и таймлайн переносят события одним и тем же
 * PUT, и расходиться в том, что именно уезжает на сервер, им нельзя.
 */
export function dateValues(
  from: Date,
  to: Date,
  fromField: Field,
  toField: Field | undefined,
): Record<string, unknown> {
  const fromKind = dateKind(fromField);
  const toKind = dateKind(toField);

  return {
    ...(fromKind ? { [fromField.slug]: fromWallClock(from, fromKind) } : {}),
    ...(toField && toKind ? { [toField.slug]: fromWallClock(to, toKind) } : {}),
  };
}

/** Вид даты у поля. Не дата вовсе — null: событий из неё не выйдет. */
export function dateKind(field: Field | undefined): DateKind | null {
  const kind = field && cellKind(field.type);
  return kind === "date" || kind === "datetime" || kind === "datetime_naive" ? kind : null;
}

/**
 * Строки → события.
 *
 * Строка без начала пропускается: место на сетке ей задать нечем.
 * Конец раньше начала — тоже начало: так строку правили руками,
 * и полоса отрицательной длины нарисовалась бы вверх ногами.
 */
export function calendarEvents(
  rows: Item[],
  fromField: Field | undefined,
  toField: Field | undefined,
): CalendarEvent[] {
  const fromKind = dateKind(fromField);
  if (!fromField || !fromKind) return [];

  const toKind = dateKind(toField);
  const events: CalendarEvent[] = [];

  for (const row of rows) {
    const from = toWallClock(row[fromField.slug], fromKind);
    if (!from) continue;

    const to = toField && toKind ? toWallClock(row[toField.slug], toKind) : null;
    events.push({
      row,
      from,
      to: to && to.getTime() > from.getTime() ? to : from,
      allDay: fromKind === "date",
    });
  }

  return events;
}

/** Ключ дня «ГГГГ-ММ-ДД» — им же размечены и нерабочие дни. */
export const dayKey = toDayInput;

/** Полночь того же дня. Границы диапазонов считаются от неё. */
export function startOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

export function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
}

/**
 * Понедельник той же недели.
 *
 * Неделя с понедельника во всех языках — так же, как в календаре выбора
 * даты (shared/ui/calendar): таблица одна на всех пользователей проекта,
 * и начинать её неделю по локали браузера значит показывать двум людям
 * разные календари одних и тех же данных.
 */
export function startOfWeek(date: Date): Date {
  const start = startOfDay(date);
  // getDay(): 0 — воскресенье, оно же конец недели, то есть минус шесть.
  return addDays(start, -((start.getDay() + 6) % 7));
}

/** Сутки в миллисекундах. */
const DAY_MS = 24 * 60 * 60_000;

/**
 * Сколько суток между днями.
 *
 * Округлением, а не делением нацело: в сутки перевода часов 23 или 25
 * часов, и целочисленное деление там врёт на день.
 */
export function daysBetween(from: Date, to: Date): number {
  return Math.round((startOfDay(to).getTime() - startOfDay(from).getTime()) / DAY_MS);
}

/**
 * Границы ленты месяцев: `past` месяцев назад и `future` вперёд от
 * месяца курсора, выровненные по неделям.
 *
 * Лента, а не один месяц: MONTH прокручивается неделями без конца
 * и подгружает соседние месяцы по мере прокрутки — так же устроен
 * и старый экран (useCalendarTemplateProps.jsx: addMorePast,
 * addMoreFuture, начальные ±3 месяца). Поэтому у сетки нет «первого»
 * и «последнего» дня месяца: есть видимая лента, и она же задаёт отбор
 * строк.
 *
 * Целыми неделями, с хвостами соседних месяцев: месяц, начинающийся
 * в четверг, иначе оставлял бы первую строку наполовину пустой.
 */
export function monthSpan(cursor: Date, past: number, future: number): { from: Date; to: Date } {
  const first = new Date(cursor.getFullYear(), cursor.getMonth() - past, 1);
  const last = new Date(cursor.getFullYear(), cursor.getMonth() + future + 1, 0);

  return { from: startOfWeek(first), to: addDays(startOfWeek(last), 6) };
}

/** Целые недели, покрывающие отрезок. Неделя начинается с понедельника. */
export function weeksBetween(from: Date, to: Date): Date[][] {
  const weeks: Date[][] = [];

  for (let day = startOfWeek(from); day <= to; day = addDays(day, 7)) {
    weeks.push(Array.from({ length: 7 }, (_, index) => addDays(day, index)));
  }

  return weeks;
}

/**
 * Диапазон, который уходит в отбор: с полуночи первого дня до конца
 * последнего.
 *
 * Конец — начало следующего дня, а не 23:59: событие в 23:59:30 иначе
 * не попало бы в выборку.
 *
 * У MONTH это вся видимая лента: `past` и `future` растут по мере
 * прокрутки, и строки для новых недель приезжают тем же запросом.
 */
export function periodRange(
  period: CalendarPeriod,
  cursor: Date,
  past = 0,
  future = 0,
): { from: Date; to: Date } {
  if (period === "DAY") return { from: startOfDay(cursor), to: addDays(cursor, 1) };
  if (period === "WEEK") {
    const from = startOfWeek(cursor);
    return { from, to: addDays(from, 7) };
  }

  const span = monthSpan(cursor, past, future);
  return { from: span.from, to: addDays(span.to, 1) };
}

/**
 * Отрезок события внутри одной недели: с какой клетки начинается,
 * сколько занимает и в какой строке лежит.
 *
 * Полосой через клетки, а не чипом в каждом дне: командировка с 3-го
 * по 7-е — одно событие, и растянуть её мышью можно только тогда, когда
 * у неё есть видимый край. Событие длиннее недели разрезается по
 * неделям, и у половинок помечены обрезанные концы — им не рисуют
 * ни скругления, ни ручки.
 */
export type WeekSegment = {
  event: CalendarEvent;
  /** Колонка начала, 0 — понедельник. */
  start: number;
  /** Сколько клеток занимает в этой неделе, 1..7. */
  span: number;
  /** Событие началось до этой недели / кончится после неё. */
  clippedStart: boolean;
  clippedEnd: boolean;
  /** Строка внутри недели: так расходятся пересекающиеся события. */
  lane: number;
};

export function weekSegments(events: CalendarEvent[], week: Date[]): WeekSegment[] {
  const first = week[0];
  if (!first) return [];

  const segments: WeekSegment[] = [];
  /** Занятые клетки по строкам: lanes[lane] — семь признаков. */
  const lanes: boolean[][] = [];

  const ordered = [...events].sort(
    (a, b) =>
      a.from.getTime() - b.from.getTime() ||
      b.to.getTime() - b.from.getTime() - (a.to.getTime() - a.from.getTime()) ||
      String(a.row.guid).localeCompare(String(b.row.guid)),
  );

  for (const event of ordered) {
    const from = daysBetween(first, event.from);
    const to = daysBetween(first, event.to);
    if (to < 0 || from > 6) continue;

    const start = Math.max(from, 0);
    const end = Math.min(to, 6);

    /*
     * Строка ищется снизу вверх: событие встаёт в первую, где его
     * клетки свободны. Иначе два непересекающихся события заняли бы
     * две строки и неделя выросла бы вдвое без причины.
     */
    let lane = lanes.findIndex((busy) => busy.slice(start, end + 1).every((taken) => !taken));
    if (lane === -1) {
      lane = lanes.length;
      lanes.push(Array.from({ length: 7 }, () => false));
    }

    const busy = lanes[lane];
    for (let day = start; day <= end && busy; day += 1) busy[day] = true;

    segments.push({
      event,
      start,
      span: end - start + 1,
      clippedStart: from < 0,
      clippedEnd: to > 6,
      lane,
    });
  }

  return segments;
}

/** Соседний период: назад или вперёд одним шагом переключателя. */
export function shiftPeriod(period: CalendarPeriod, cursor: Date, step: number): Date {
  if (period === "DAY") return addDays(cursor, step);
  if (period === "WEEK") return addDays(cursor, 7 * step);

  return new Date(cursor.getFullYear(), cursor.getMonth() + step, 1);
}

/**
 * События по дням.
 *
 * Событие попадает в КАЖДЫЙ день, который оно задевает: командировка
 * с 3-го по 7-е видна во всех пяти клетках, а не только в первой.
 * Полосой через несколько клеток она не рисуется — это отдельная
 * геометрия ради того же смысла (ponytail: полоса, если по чипу в дне
 * окажется мало).
 */
export function eventsByDay(events: CalendarEvent[]): Map<string, CalendarEvent[]> {
  const byDay = new Map<string, CalendarEvent[]>();

  for (const event of [...events].sort(byStart)) {
    const last = startOfDay(event.to);

    for (let day = startOfDay(event.from); day <= last; day = addDays(day, 1)) {
      const key = dayKey(day);
      const list = byDay.get(key);
      if (list) list.push(event);
      else byDay.set(key, [event]);
    }
  }

  return byDay;
}

/** Событие на сетке часов: доля суток сверху и высотой. */
export type PlacedEvent = {
  event: CalendarEvent;
  /** Доля суток от полуночи до начала: 0 — полночь, 0.5 — полдень. */
  top: number;
  /** Доля суток, которую занимает событие в ЭТОТ день. */
  height: number;
  /** Колонка внутри дня и сколько их всего: так расходятся пересечения. */
  column: number;
  columns: number;
};

/**
 * Раскладка одного дня: пересекающиеся события расходятся соседними
 * колонками.
 *
 * Считается по группам: события, связанные пересечением в цепочку,
 * делят ширину дня поровну. Поэтому две встречи с утра не становятся
 * узкими из-за третьей, которая пересекается с чем-то вечером.
 *
 * `day` нужен, потому что событие бывает длиннее суток: в каждом дне
 * от него видна своя часть, обрезанная полуночью.
 */
export function placeDay(events: CalendarEvent[], day: Date): PlacedEvent[] {
  const start = startOfDay(day).getTime();
  const end = start + MINUTES_PER_DAY * 60_000;

  const visible = events
    .map((event) => {
      const from = Math.max(event.from.getTime(), start);
      /* Точка в дне и событие без конца занимают минимум один отрезок:
         нулевая высота — это невидимое событие. */
      const to = Math.min(Math.max(event.to.getTime(), from + POINT_MINUTES * 60_000), end);
      return { event, from, to };
    })
    .sort((a, b) => a.from - b.from || b.to - b.from - (a.to - a.from));

  const placed: PlacedEvent[] = [];
  /** Конец последнего события в каждой колонке текущей группы. */
  let columnEnds: number[] = [];
  /** События текущей группы: ширину они узнают, когда группа закроется. */
  let group: PlacedEvent[] = [];

  for (const item of visible) {
    // Ни с чем из группы не пересекается — группа закрыта, ширина известна.
    if (columnEnds.every((columnEnd) => columnEnd <= item.from)) {
      for (const member of group) member.columns = columnEnds.length || 1;
      columnEnds = [];
      group = [];
    }

    const free = columnEnds.findIndex((columnEnd) => columnEnd <= item.from);
    const column = free === -1 ? columnEnds.length : free;
    columnEnds[column] = item.to;

    const entry: PlacedEvent = {
      event: item.event,
      top: (item.from - start) / (end - start),
      height: (item.to - item.from) / (end - start),
      column,
      columns: 1,
    };

    group.push(entry);
    placed.push(entry);
  }

  for (const member of group) member.columns = columnEnds.length || 1;

  return placed;
}

/**
 * Куда встанет событие, если его бросили на `target`.
 *
 * Двигаются ОБА конца на одну и ту же разницу: перенос меняет место
 * события, а не его длительность. Растягивание за край — отдельное
 * действие, и там двигается один конец.
 */
export function movedTo(event: CalendarEvent, target: Date): { from: Date; to: Date } {
  const shift = target.getTime() - event.from.getTime();
  return { from: target, to: new Date(event.to.getTime() + shift) };
}

/**
 * Время под курсором: доля высоты дня → часы и минуты, округлённые
 * к ближайшему отрезку сетки.
 *
 * По сетке, а не по пикселю: попасть мышью в «14:00» на колонке высотой
 * в экран невозможно, а бросок в 13:57 — это не то, что человек имел
 * в виду.
 */
export function timeAt(day: Date, ratio: number, step = SLOT_MINUTES): Date {
  const minutes = Math.min(Math.max(ratio, 0), 1) * MINUTES_PER_DAY;
  const snapped = Math.round(minutes / step) * step;

  return new Date(startOfDay(day).getTime() + Math.min(snapped, MINUTES_PER_DAY - step) * 60_000);
}

/** Отрезки сетки суток: подписи слева и линии в колонке дня. */
export function daySlots(step = SLOT_MINUTES): { minutes: number; label: string }[] {
  const slots = [];

  for (let minutes = 0; minutes < MINUTES_PER_DAY; minutes += step) {
    slots.push({ minutes, label: `${pad(Math.floor(minutes / 60))}:${pad(minutes % 60)}` });
  }

  return slots;
}

function byStart(a: CalendarEvent, b: CalendarEvent): number {
  return a.from.getTime() - b.from.getTime() || b.to.getTime() - a.to.getTime();
}

function pad(value: number): string {
  return String(value).padStart(2, "0");
}
