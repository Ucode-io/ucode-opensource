import { describe, expect, it } from "vitest";
import type { Field } from "@/features/table";
import {
  calendarEvents,
  eventsByDay,
  fromWallClock,
  monthSpan,
  movedTo,
  periodRange,
  placeDay,
  timeAt,
  toPeriod,
  toWallClock,
  weekSegments,
  weeksBetween,
} from "./calendar";

const field = (slug: string, type: string): Field =>
  ({ id: slug, slug, type, label: slug, labels: {}, options: new Map() }) as unknown as Field;

const from = field("starts_at", "DATE_TIME_WITHOUT_TIME_ZONE");
const to = field("ends_at", "DATE_TIME_WITHOUT_TIME_ZONE");
const day = field("day", "DATE");

describe("toWallClock", () => {
  it("читает время без пояса ровно так, как оно записано", () => {
    // Значение лежит в UTC-частях, а показывается как есть: сдвига
    // на часовой пояс браузера быть не должно ни в одну сторону.
    const date = toWallClock("2026-03-08T10:30:00Z", "datetime_naive");

    expect(date?.getHours()).toBe(10);
    expect(date?.getDate()).toBe(8);
  });

  it("дата без времени — это полночь того же дня, а не вчера", () => {
    expect(toWallClock("2026-01-06", "date")?.getDate()).toBe(6);
  });

  it("возвращается тем же значением, каким пришло", () => {
    expect(fromWallClock(new Date(2026, 0, 6, 10, 30), "datetime_naive")).toBe(
      "2026-01-06T10:30:00Z",
    );
    expect(fromWallClock(new Date(2026, 0, 6), "date")).toBe("2026-01-06");
  });
});

describe("calendarEvents", () => {
  it("строка без начала на сетку не попадает", () => {
    const events = calendarEvents([{ guid: "a" }, { guid: "b", starts_at: "2026-01-06T09:00:00Z" }], from, to);

    expect(events).toHaveLength(1);
    expect(events[0]?.row.guid).toBe("b");
  });

  it("без поля конца событие — точка, а не полоса на сутки", () => {
    const [event] = calendarEvents([{ guid: "a", starts_at: "2026-01-06T09:00:00Z" }], from, undefined);

    expect(event?.to.getTime()).toBe(event?.from.getTime());
  });

  it("конец раньше начала не рисуется наоборот", () => {
    const [event] = calendarEvents(
      [{ guid: "a", starts_at: "2026-01-06T09:00:00Z", ends_at: "2026-01-06T08:00:00Z" }],
      from,
      to,
    );

    expect(event?.to.getTime()).toBe(event?.from.getTime());
  });

  it("поле-дата помечает событие как «весь день»", () => {
    const [event] = calendarEvents([{ guid: "a", day: "2026-01-06" }], day, undefined);

    expect(event?.allDay).toBe(true);
  });
});

describe("monthSpan и weeksBetween", () => {
  it("месяц выложен целыми неделями с понедельника", () => {
    const span = monthSpan(new Date(2026, 0, 15), 0, 0);
    const weeks = weeksBetween(span.from, span.to);

    expect(weeks[0]?.[0]?.getDay()).toBe(1);
    expect(weeks.every((week) => week.length === 7)).toBe(true);
    // Январь 2026 начинается в четверг: первая неделя приходит хвостом
    // декабря, иначе строка была бы наполовину пустой.
    expect(weeks[0]?.[0]?.getMonth()).toBe(11);
  });

  it("лента растёт месяцами в обе стороны", () => {
    // Июнь, два месяца назад и три вперёд: апрель… сентябрь целиком.
    const span = monthSpan(new Date(2026, 5, 15), 2, 3);

    expect(span.from.getTime()).toBeLessThanOrEqual(new Date(2026, 3, 1).getTime());
    expect(span.to.getTime()).toBeGreaterThanOrEqual(new Date(2026, 8, 30).getTime());
    // Границы всё так же на неделе: понедельник в начале, воскресенье в конце.
    expect(span.from.getDay()).toBe(1);
    expect(span.to.getDay()).toBe(0);
  });
});

describe("weekSegments", () => {
  const week = Array.from({ length: 7 }, (_, index) => new Date(2026, 0, 5 + index));
  const events = (...pairs: [string, string][]) =>
    calendarEvents(
      pairs.map(([starts_at, ends_at], index) => ({ guid: String(index), starts_at, ends_at })),
      from,
      to,
    );

  it("событие в неделе — одна полоса от начала до конца", () => {
    const [segment] = weekSegments(
      events(["2026-01-06T09:00:00Z", "2026-01-08T18:00:00Z"]),
      week,
    );

    // Понедельник — 5 января, значит вторник это колонка 1.
    expect(segment).toMatchObject({ start: 1, span: 3, clippedStart: false, clippedEnd: false });
  });

  it("событие длиннее недели обрезано по её краям", () => {
    const [segment] = weekSegments(
      events(["2026-01-02T09:00:00Z", "2026-01-20T18:00:00Z"]),
      week,
    );

    expect(segment).toMatchObject({ start: 0, span: 7, clippedStart: true, clippedEnd: true });
  });

  it("события чужой недели не попадают в неё вовсе", () => {
    expect(weekSegments(events(["2026-02-02T09:00:00Z", "2026-02-03T09:00:00Z"]), week)).toEqual([]);
  });

  it("пересекающиеся расходятся строками, непересекающиеся делят одну", () => {
    const overlapping = weekSegments(
      events(
        ["2026-01-05T09:00:00Z", "2026-01-07T09:00:00Z"],
        ["2026-01-06T09:00:00Z", "2026-01-08T09:00:00Z"],
      ),
      week,
    );
    expect(overlapping.map((segment) => segment.lane)).toEqual([0, 1]);

    const apart = weekSegments(
      events(
        ["2026-01-05T09:00:00Z", "2026-01-06T09:00:00Z"],
        ["2026-01-08T09:00:00Z", "2026-01-09T09:00:00Z"],
      ),
      week,
    );
    expect(apart.map((segment) => segment.lane)).toEqual([0, 0]);
  });
});

describe("periodRange", () => {
  it("верхняя граница — начало следующего дня, а не 23:59", () => {
    const range = periodRange("DAY", new Date(2026, 0, 6, 13, 0));

    expect(range.from.getHours()).toBe(0);
    expect(range.to.getDate()).toBe(7);
  });

  it("у месяца диапазон — вся видимая лента, а не один месяц", () => {
    const range = periodRange("MONTH", new Date(2026, 5, 15), 1, 1);

    // Май и июль попадают в отбор целиком, иначе их недели приехали бы пустыми.
    expect(range.from.getTime()).toBeLessThanOrEqual(new Date(2026, 4, 1).getTime());
    expect(range.to.getTime()).toBeGreaterThan(new Date(2026, 6, 31).getTime());
  });
});

describe("eventsByDay", () => {
  it("событие видно в каждом дне, который оно задевает", () => {
    const events = calendarEvents(
      [{ guid: "a", starts_at: "2026-01-06T09:00:00Z", ends_at: "2026-01-08T18:00:00Z" }],
      from,
      to,
    );
    const byDay = eventsByDay(events);

    expect([...byDay.keys()]).toEqual(["2026-01-06", "2026-01-07", "2026-01-08"]);
  });
});

describe("placeDay", () => {
  const events = (...pairs: [string, string][]) =>
    calendarEvents(
      pairs.map(([starts_at, ends_at], index) => ({ guid: String(index), starts_at, ends_at })),
      from,
      to,
    );

  it("пересекающиеся события расходятся колонками", () => {
    const placed = placeDay(
      events(
        ["2026-01-06T09:00:00Z", "2026-01-06T11:00:00Z"],
        ["2026-01-06T10:00:00Z", "2026-01-06T12:00:00Z"],
      ),
      new Date(2026, 0, 6),
    );

    expect(placed.map((item) => item.column)).toEqual([0, 1]);
    expect(placed.every((item) => item.columns === 2)).toBe(true);
  });

  it("непересекающиеся занимают всю ширину дня", () => {
    const placed = placeDay(
      events(
        ["2026-01-06T09:00:00Z", "2026-01-06T10:00:00Z"],
        ["2026-01-06T10:00:00Z", "2026-01-06T11:00:00Z"],
      ),
      new Date(2026, 0, 6),
    );

    expect(placed.every((item) => item.columns === 1 && item.column === 0)).toBe(true);
  });

  it("утренняя пара не сужается из-за вечерней", () => {
    const placed = placeDay(
      events(
        ["2026-01-06T09:00:00Z", "2026-01-06T11:00:00Z"],
        ["2026-01-06T10:00:00Z", "2026-01-06T12:00:00Z"],
        ["2026-01-06T18:00:00Z", "2026-01-06T19:00:00Z"],
      ),
      new Date(2026, 0, 6),
    );

    expect(placed.map((item) => item.columns)).toEqual([2, 2, 1]);
  });

  it("событие длиннее суток обрезается полуночью", () => {
    const [placed] = placeDay(
      events(["2026-01-06T22:00:00Z", "2026-01-07T04:00:00Z"]),
      new Date(2026, 0, 6),
    );

    expect(placed?.top).toBeCloseTo(22 / 24);
    expect(placed?.height).toBeCloseTo(2 / 24);
  });

  it("событие без длительности всё равно видно", () => {
    const [placed] = placeDay(
      calendarEvents([{ guid: "a", starts_at: "2026-01-06T09:00:00Z" }], from, undefined),
      new Date(2026, 0, 6),
    );

    expect(placed?.height).toBeGreaterThan(0);
  });
});

describe("timeAt", () => {
  it("время округляется к шагу сетки", () => {
    // 13:57 из середины экрана — не то, что человек имел в виду.
    const time = timeAt(new Date(2026, 0, 6), (13 * 60 + 57) / (24 * 60));

    expect(time.getHours()).toBe(14);
    expect(time.getMinutes()).toBe(0);
  });

  it("бросок ниже последнего отрезка остаётся в тех же сутках", () => {
    expect(timeAt(new Date(2026, 0, 6), 1).getDate()).toBe(6);
  });
});

describe("movedTo", () => {
  it("перенос двигает оба конца, длительность не меняется", () => {
    const [event] = calendarEvents(
      [{ guid: "a", starts_at: "2026-01-06T09:00:00Z", ends_at: "2026-01-06T11:00:00Z" }],
      from,
      to,
    );
    const moved = movedTo(event!, new Date(2026, 0, 7, 15, 0));

    expect(moved.to.getDate()).toBe(7);
    expect(moved.to.getHours()).toBe(17);
  });
});

describe("toPeriod", () => {
  it("незнакомая настройка открывает месяц", () => {
    expect(toPeriod("QUARTER")).toBe("MONTH");
    expect(toPeriod("WEEK")).toBe("WEEK");
  });
});
