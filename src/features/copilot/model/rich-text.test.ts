import { expect, test } from "vitest";
import { blocks, inline } from "./rich-text";

/*
 * Ради чего разбор вообще есть: ответ модели не должен показываться
 * со звёздочками и кавычками, а незакрытая ограда кода не должна
 * съедать остаток ответа.
 */

test("абзацы разделяются пустой строкой, перенос внутри остаётся", () => {
  expect(blocks("Первый абзац\nего продолжение\n\nВторой")).toEqual([
    { kind: "text", text: "Первый абзац\nего продолжение" },
    { kind: "text", text: "Второй" },
  ]);
});

test("список собирается целиком, маркер уходит", () => {
  expect(blocks("Создал:\n- таблица product\n- поле price\n\nГотово.")).toEqual([
    { kind: "text", text: "Создал:" },
    { kind: "list", ordered: false, items: ["таблица product", "поле price"] },
    { kind: "text", text: "Готово." },
  ]);

  expect(blocks("1. первое\n2. второе")).toEqual([
    { kind: "list", ordered: true, items: ["первое", "второе"] },
  ]);
});

test("незакрытая ограда кода не съедает ответ", () => {
  expect(blocks("```\nSELECT 1\n")).toEqual([{ kind: "code", text: "SELECT 1" }]);
  expect(blocks("```sql\nSELECT 1\n```\nвсё")).toEqual([
    { kind: "code", text: "SELECT 1" },
    { kind: "text", text: "всё" },
  ]);
});

test("жирный и код разбираются, одинокий маркер остаётся символом", () => {
  expect(inline("Создал **product** с полем `price`")).toEqual([
    { kind: "plain", text: "Создал " },
    { kind: "bold", text: "product" },
    { kind: "plain", text: " с полем " },
    { kind: "code", text: "price" },
  ]);

  expect(inline("2 * 2 и незакрытый **хвост")).toEqual([
    { kind: "plain", text: "2 * 2 и незакрытый **хвост" },
  ]);
});

test("звёздочки внутри кода — это звёздочки", () => {
  expect(inline("`SELECT **` готов")).toEqual([
    { kind: "code", text: "SELECT **" },
    { kind: "plain", text: " готов" },
  ]);
});
