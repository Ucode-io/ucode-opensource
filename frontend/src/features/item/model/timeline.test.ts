import { describe, expect, it } from "vitest";
import type { Item } from "./types";
import type { CalendarEvent } from "./calendar";
import {
  COLUMN_WIDTH,
  dayAt,
  monthGroups,
  timelineBar,
  timelineDays,
  toScale,
  weekGroups,
} from "./timeline";

const event = (from: string, to: string): CalendarEvent => ({
  row: { guid: `${from}/${to}` } as Item,
  from: new Date(from),
  to: new Date(to),
  allDay: true,
});

const day = (value: string) => new Date(`${value}T00:00:00`);

describe("toScale", () => {
  it("непонятное значение — дни: с колонки в двенадцать пикселей начинать нечего", () => {
    expect(toScale(undefined)).toBe("DAY");
    expect(toScale("QUARTER")).toBe("DAY");
    expect(toScale("WEEK")).toBe("WEEK");
    expect(toScale("MONTH")).toBe("MONTH");
  });
});

describe("weekGroups", () => {
  it("подпись на неделю, а не на месяц: неделя начинается с понедельника", () => {
    // 2026-03-01 — воскресенье: оно принадлежит ПРЕДЫДУЩЕЙ неделе,
    // и подписей на этом отрезке две, а не одна.
    const groups = weekGroups(timelineDays(day("2026-02-26"), day("2026-03-03")));

    // 26–28 февраля и 1 марта — одна неделя; 2 и 3 марта — уже другая.
    expect(groups.map((group) => group.span)).toEqual([4, 2]);
    expect(groups[1]?.first.getDate()).toBe(2);
  });

  it("недели соседних месяцев не разрываются подписью месяца", () => {
    // Ровно этим WEEK и отличается от MONTH: колонки те же, подпись другая.
    const days = timelineDays(day("2026-03-30"), day("2026-04-05"));

    expect(weekGroups(days)).toHaveLength(1);
    expect(monthGroups(days)).toHaveLength(2);
  });
});

describe("timelineDays", () => {
  it("включает последний день, а не обрывается перед ним", () => {
    const days = timelineDays(day("2026-03-01"), day("2026-03-03"));

    expect(days).toHaveLength(3);
    expect(days[2]?.getDate()).toBe(3);
  });

  it("переход на летнее время не съедает и не добавляет дня", () => {
    // В зонах с переводом часов сутки бывают 23 и 25 часов: деление
    // нацело там врёт, поэтому счёт идёт по календарным дням.
    const days = timelineDays(day("2026-03-28"), day("2026-03-31"));

    expect(days.map((item) => item.getDate())).toEqual([28, 29, 30, 31]);
  });
});

describe("timelineBar", () => {
  const first = day("2026-03-01");

  it("полоса занимает все дни события, включая последний", () => {
    const bar = timelineBar(event("2026-03-03T00:00", "2026-03-07T00:00"), first, 31);

    expect(bar).toMatchObject({ start: 2, span: 5, clippedStart: false, clippedEnd: false });
  });

  it("событие в один день — одна колонка, а не ноль", () => {
    expect(timelineBar(event("2026-03-03T09:00", "2026-03-03T09:00"), first, 31)?.span).toBe(1);
  });

  it("край за пределами оси помечен обрезанным: тянуть его надо не здесь", () => {
    const bar = timelineBar(event("2026-02-25T00:00", "2026-04-02T00:00"), first, 31);

    expect(bar).toMatchObject({ start: 0, span: 31, clippedStart: true, clippedEnd: true });
  });

  it("событие целиком за краем оси не рисуется вовсе", () => {
    expect(timelineBar(event("2026-01-05T00:00", "2026-01-06T00:00"), first, 31)).toBeNull();
  });
});

describe("monthGroups", () => {
  it("подпись раз на месяц, шириной в его дни", () => {
    const groups = monthGroups(timelineDays(day("2026-01-30"), day("2026-02-02")));

    expect(groups).toHaveLength(2);
    expect(groups[0]?.span).toBe(2);
    expect(groups[1]?.span).toBe(2);
  });

  it("одинаковые месяцы разных лет не слипаются", () => {
    expect(monthGroups(timelineDays(day("2025-12-31"), day("2026-01-01")))).toHaveLength(2);
  });
});

describe("dayAt", () => {
  const days = timelineDays(day("2026-03-01"), day("2026-03-05"));
  const width = COLUMN_WIDTH.DAY;

  it("указатель в середине колонки попадает в её день", () => {
    expect(dayAt(days, width * 2 + width / 2, width)?.getDate()).toBe(3);
  });

  it("за краями оси событие встаёт на крайний день, а не в пустоту", () => {
    expect(dayAt(days, -500, width)?.getDate()).toBe(1);
    expect(dayAt(days, 99_999, width)?.getDate()).toBe(5);
  });

  it("пустая ось не даёт дня вовсе", () => {
    expect(dayAt([], 10, width)).toBeNull();
  });
});
