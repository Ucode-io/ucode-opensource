import { expect, test } from "vitest";
import { safeHref } from "./normalize";

test("обычные адреса проходят", () => {
  expect(safeHref("https://docs.u-code.io")).toBe("https://docs.u-code.io");
  expect(safeHref("http://internal.local/page")).toBe("http://internal.local/page");
});

test("javascript: и data: не проходят", () => {
  // Пункт меню рисуется как <a href={…}>: такой адрес выполнил бы
  // чужой код по клику, а значение приходит из поля пользователя.
  expect(safeHref("javascript:alert(1)")).toBeUndefined();
  expect(safeHref("JavaScript:alert(1)")).toBeUndefined();
  expect(safeHref("data:text/html,<script>alert(1)</script>")).toBeUndefined();
  expect(safeHref("vbscript:msgbox(1)")).toBeUndefined();
});

test("относительный путь не считается ссылкой", () => {
  // Это внутренний переход, а не внешняя ссылка: вести себя должен иначе.
  expect(safeHref("/reports")).toBeUndefined();
  expect(safeHref("reports")).toBeUndefined();
});

test("пустое и не-строка не проходят", () => {
  expect(safeHref("")).toBeUndefined();
  expect(safeHref("   ")).toBeUndefined();
  expect(safeHref(null)).toBeUndefined();
  expect(safeHref(42)).toBeUndefined();
});
