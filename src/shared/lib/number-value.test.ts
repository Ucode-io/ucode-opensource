import { expect, test } from "vitest";
import { formatNumber } from "./number-value";

/*
 * Разделитель разрядов в ru — неразрывный пробел, узкий или обычный:
 * какой именно, решает версия ICU в браузере. Сравнивать с ним
 * побайтово нельзя, поэтому приводим оба к обычному.
 */
const nbsp = (text: string) => text.replace(/[\u00a0\u202f]/g, " ");

test("разряды разделяются по языку интерфейса", () => {
  expect(nbsp(formatNumber(1234567, "ru"))).toBe("1 234 567");
  expect(formatNumber(1234567, "en")).toBe("1,234,567");
  // Число, пришедшее строкой: колонки VARCHAR отдают его текстом.
  expect(formatNumber("1234.5", "en")).toBe("1,234.5");
});

test("дробная часть не округляется", () => {
  // Умолчание Intl — три знака: без своей настройки показ соврал бы.
  expect(formatNumber(1.23456, "en")).toBe("1.23456");
});

test("не число остаётся как есть", () => {
  expect(formatNumber("", "en")).toBe("");
  expect(formatNumber("абв", "en")).toBe("абв");
  expect(formatNumber(Number.NaN, "en")).toBe("NaN");
});
