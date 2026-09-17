import { expect, test } from "vitest";
import { relativeTime } from "./time";

/* Время передаётся вторым аргументом: тест, зависящий от Date.now(),
   ломался бы сам по себе в полночь и на границе месяца. */
const NOW = new Date("2026-09-14T12:00:00Z").getTime();
const ago = (ms: number) => new Date(NOW - ms).toISOString();

const MINUTE = 60 * 1000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

test("единица — самая крупная из подошедших", () => {
  expect(relativeTime(ago(5 * MINUTE), "en", NOW)).toBe("5 minutes ago");
  expect(relativeTime(ago(3 * HOUR), "en", NOW)).toBe("3 hours ago");
  // Не «26 hours ago»: сутки уже набрались, и днями читается быстрее.
  expect(relativeTime(ago(26 * HOUR), "en", NOW)).toBe("yesterday");
  expect(relativeTime(ago(40 * DAY), "en", NOW)).toBe("last month");
});

test("меньше минуты — «сейчас», а не ноль минут", () => {
  expect(relativeTime(ago(20 * 1000), "en", NOW)).toBe("now");
});

/* Часы браузера и сервера расходятся, и свежая запись регулярно
   приезжает «из будущего» на секунды-минуты. Это не повод молчать. */
test("будущее не отбрасывается", () => {
  expect(relativeTime(new Date(NOW + 2 * MINUTE).toISOString(), "en", NOW)).toBe("in 2 minutes");
});

test("пусто и мусор — пустая строка, а не «Invalid Date»", () => {
  expect(relativeTime("", "en", NOW)).toBe("");
  expect(relativeTime("не дата", "en", NOW)).toBe("");
});
