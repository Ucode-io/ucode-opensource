import { expect, test } from "vitest";
import {
  fromDateInput,
  sameValue,
  toDateInput,
  toDateValue,
  toList,
  toNumber,
} from "./cell-value";

test("календарная дата не зависит от часового пояса", () => {
  // Ровно эта строка, прочитанная как момент времени, к западу от Гринвича
  // становится предыдущим днём. Дата — не момент, поэтому день тот же.
  const parsed = toDateValue("2026-01-06T00:00:00Z", "date");

  expect(parsed?.naive).toBe(true);
  expect(parsed?.date.getUTCDate()).toBe(6);
  expect(toDateInput("2026-01-06T00:00:00Z", "date")).toBe("2026-01-06");
  expect(toDateInput("2026-01-06", "date")).toBe("2026-01-06");
});

test("время без пояса показывается как записано", () => {
  expect(toDateInput("2026-01-06T10:13:00", "datetime_naive")).toBe("2026-01-06T10:13");
  // Пробел вместо T — так отдаёт postgres.
  expect(toDateInput("2026-01-06 10:13:00.000000", "datetime_naive")).toBe("2026-01-06T10:13");
});

test("время без пояса приходит в обратном порядке — так его отдаёт бэкенд", () => {
  // items.go форматирует DATE_TIME_WITHOUT_TIME_ZONE как «02.01.2006 15:04»,
  // и new Date() такую строку не разбирает вовсе.
  expect(toDateInput("24.12.2025 08:49", "datetime_naive")).toBe("2025-12-24T08:49");
  expect(toDateInput("24.12.2025", "date")).toBe("2025-12-24");
});

test("момент времени переводится в пояс браузера", () => {
  const parsed = toDateValue("2026-01-06T10:13:00Z", "datetime");

  expect(parsed?.naive).toBe(false);
  expect(parsed?.date.toISOString()).toBe("2026-01-06T10:13:00.000Z");
});

test("нечитаемая дата не притворяется датой", () => {
  expect(toDateValue("завтра", "datetime")).toBeNull();
  expect(toDateValue("", "date")).toBeNull();
  expect(toDateValue(null, "date")).toBeNull();
});

test("обратно уходит формат, который разбирает бэкенд", () => {
  // ConvertTimestamp2DB знает ровно этот шаблон и молча стирает значение,
  // если пришли миллисекунды.
  expect(fromDateInput("2026-01-06T10:13", "datetime_naive")).toBe("2026-01-06T10:13:00Z");
  expect(fromDateInput("2026-01-06", "date")).toBe("2026-01-06");
  expect(fromDateInput("", "datetime")).toBeNull();
});

test("пустое число — это null, а не ноль", () => {
  expect(toNumber("")).toBeNull();
  expect(toNumber("  ")).toBeNull();
  expect(toNumber("0")).toBe(0);
  expect(toNumber("12,5")).toBe(12.5);
  expect(toNumber("не число")).toBeNull();
});

test("все виды пустоты равны между собой", () => {
  // Иначе открытая и закрытая пустая ячейка отправляла бы правку.
  expect(sameValue(undefined, "")).toBe(true);
  expect(sameValue(null, [])).toBe(true);
  expect(sameValue("", "0")).toBe(false);
  expect(sameValue(["a", "b"], ["a", "b"])).toBe(true);
  expect(sameValue(["a"], ["b"])).toBe(false);
});

test("значение мультиселекта всегда список", () => {
  expect(toList(["a", "b"])).toEqual(["a", "b"]);
  expect(toList("a")).toEqual(["a"]);
  expect(toList(null)).toEqual([]);
  expect(toList([null, "a", ""])).toEqual(["a"]);
});
