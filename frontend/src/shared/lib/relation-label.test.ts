import { expect, test } from "vitest";
import { relationLabel } from "./relation-label";

const row = {
  number: "A-17",
  created_at: "2026-01-06T00:00:00Z",
  title_en: "Order",
  title_cyr: "Заказ",
};

test("дата в подписи форматируется, а не показывается сырой", () => {
  // Связанная строка приезжает как row_to_json, без обработки:
  // без форматирования в чипе висело бы «2026-01-06T00:00:00Z».
  const label = relationLabel(row, [
    { slug: "number", type: "SINGLE_LINE" },
    { slug: "created_at", type: "DATE" },
  ]);

  expect(label.startsWith("A-17 ")).toBe(true);
  expect(label).not.toContain("T00:00:00Z");
  // Календарная дата не съезжает на день: печатается как есть, в UTC.
  expect(label).toContain("6");
  expect(label).toContain("2026");
});

test("из мультиязычных колонок остаётся одна — на языке данных", () => {
  const fields = [
    { slug: "title_en", type: "SINGLE_LINE" },
    { slug: "title_cyr", type: "SINGLE_LINE" },
  ];

  expect(relationLabel(row, fields, "cyr")).toBe("Заказ");
  expect(relationLabel(row, fields, "en")).toBe("Order");
  // Языка нет среди выбранных колонок — оставляем всё, как было.
  expect(relationLabel(row, fields, "uz")).toBe("Order Заказ");
  expect(relationLabel(row, fields)).toBe("Order Заказ");
});

test("соседнее поле с общим началом слага не выбрасывается", () => {
  const label = relationLabel({ title_en: "Order", title: "Заказ", number: "7" }, [
    { slug: "number", type: "SINGLE_LINE" },
    { slug: "title_en", type: "SINGLE_LINE" },
  ], "en");

  // `number` с `title` не однокоренной — он на месте.
  expect(label).toBe("7 Order");
});

test("пустое и объекты в подпись не идут", () => {
  const label = relationLabel({ a: "", b: null, c: { x: 1 }, d: "есть" }, [
    { slug: "a", type: "SINGLE_LINE" },
    { slug: "b", type: "SINGLE_LINE" },
    { slug: "c", type: "SINGLE_LINE" },
    { slug: "d", type: "SINGLE_LINE" },
  ]);

  expect(label).toBe("есть");
});
