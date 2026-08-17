import { expect, test } from "vitest";
import {
  baseSlug,
  fieldLanguage,
  fieldsForLanguage,
  hasMultilanguage,
  stripLanguage,
} from "./multilanguage";
import type { Field } from "./types";

const LANGS = ["en", "cyr"];

const field = (slug: string, multilanguage = false): Field => ({
  id: slug,
  slug,
  label: slug,
  labels: {},
  type: "SINGLE_LINE",
  relationId: null,
  options: new Map(),
  multilanguage,
  hasColor: false,
  required: false,
  validation: null,
  editable: true,
  attributes: {},
  raw: {},
});

test("код языка отрезается только по языкам проекта", () => {
  expect(baseSlug(field("title_en", true), LANGS)).toBe("title");
  expect(baseSlug(field("title_cyr", true), LANGS)).toBe("title");
  expect(fieldLanguage(field("title_cyr", true), LANGS)).toBe("cyr");
});

/*
 * Старая админка резала слаг по последнему подчёркиванию: `order_id`
 * превращался в базу `order` с «языком» id. Отсюда обязательная проверка
 * по списку языков проекта.
 */
test("подчёркивание в слаге не делает поле языковым", () => {
  expect(baseSlug(field("order_id", true), LANGS)).toBeNull();
  expect(baseSlug(field("created_by", true), LANGS)).toBeNull();
});

test("поле без признака мультиязычности не разбирается вовсе", () => {
  expect(baseSlug(field("title_en"), LANGS)).toBeNull();
  expect(hasMultilanguage([field("title_en"), field("price")], LANGS)).toBe(false);
});

test("языковые варианты схлопываются в один, на месте первого", () => {
  const fields = [
    field("code"),
    field("title_en", true),
    field("title_cyr", true),
    field("price"),
  ];

  const shown = fieldsForLanguage(fields, LANGS, "cyr");

  // Место первого варианта, а не варианта на активном языке: иначе поле
  // прыгало бы по карточке при переключении языка.
  expect(shown.map((item) => item.slug)).toEqual(["code", "title_cyr", "price"]);
  expect(fieldsForLanguage(fields, LANGS, "en").map((item) => item.slug)).toEqual([
    "code",
    "title_en",
    "price",
  ]);
});

test("нет варианта на активном языке — показывается первый существующий", () => {
  const fields = [field("title_en", true)];

  // Язык добавили в проект позже, чем поле. Пустая строка вместо значения
  // хуже, чем значение не на том языке: второе хотя бы видно.
  expect(fieldsForLanguage(fields, LANGS, "cyr").map((item) => item.slug)).toEqual(["title_en"]);
});

test("код языка убирается из подписи, честная подпись не режется", () => {
  expect(stripLanguage("Название (en)", "en")).toBe("Название");
  expect(stripLanguage("Название en", "en")).toBe("Название");
  expect(stripLanguage("Название", "en")).toBe("Название");
  // «Регион» не должно стать «Реги» из-за случайного совпадения хвоста.
  expect(stripLanguage("Регион", "он")).toBe("Регион");
});
