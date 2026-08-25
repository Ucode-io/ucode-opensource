import { expect, test } from "vitest";
import {
  baseSlug,
  collapseLanguages,
  fieldLanguage,
  fieldsForLanguage,
  hasMultilanguage,
  localizeKeys,
  localizeSlug,
  stripLanguage,
} from "./multilanguage";
import { localized, tabLabel } from "./types";
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
    locked: false,
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

/*
 * Признак — не флаг, а форма набора: object_builder не пишет
 * enable_multilanguage при вставке (field.go:116), и у только что
 * созданного языкового поля флаг всегда false.
 */
test("одиночное поле с языковым суффиксом языковой группой не считается", () => {
  expect(baseSlug(field("title_en"), LANGS)).toBe("title");
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

test("сведение языковых колонок снимает код языка с подписи", () => {
  const named = (slug: string, label: string): Field => ({
    ...field(slug, true),
    label,
    labels: { en: `${label}`, cyr: `${label}` },
  });

  const [title, price] = collapseLanguages(
    [named("title_en", "Название (en)"), named("title_cyr", "Название (cyr)"), field("price")],
    LANGS,
    "cyr",
  );

  // Место — первого варианта, значение — активного языка, подпись — без кода.
  expect(title?.slug).toBe("title_cyr");
  expect(title?.label).toBe("Название");
  expect(title?.labels["en"]).toBe("Название");
  // Обычное поле проходит нетронутым.
  expect(price?.slug).toBe("price");
});

test("сведение идемпотентно: повторный вызов ничего не меняет", () => {
  const fields = [field("title_en", true), field("title_cyr", true)];
  const once = collapseLanguages(fields, LANGS, "en");
  const twice = collapseLanguages(once, LANGS, "en");

  expect(twice.map((item) => item.slug)).toEqual(once.map((item) => item.slug));
});

test("пара без флага — всё равно языковая группа: флаг база не хранит", () => {
  const fields = [field("naming_en"), field("naming_cyr"), field("price")];

  expect(hasMultilanguage(fields, LANGS)).toBe(true);
  expect(collapseLanguages(fields, LANGS, "cyr").map((item) => item.slug)).toEqual([
    "naming_cyr",
    "price",
  ]);
});

/*
 * Фильтр и сортировка ключуются слагом, а у мультиязычного поля слаг
 * на каждом языке свой: условие обязано переехать на вариант активного
 * языка — иначе после переключения оно молча бьёт по невидимой колонке.
 */
test("слаг условия переезжает на вариант активного языка", () => {
  const fields = [field("title_en", true), field("title_cyr", true), field("price")];

  expect(localizeSlug("title_en", fields, LANGS, "cyr")).toBe("title_cyr");
  expect(localizeSlug("title_cyr", fields, LANGS, "cyr")).toBe("title_cyr");
  // Обычное поле — даже с подчёркиванием — не трогается.
  expect(localizeSlug("price", fields, LANGS, "cyr")).toBe("price");
  expect(localizeSlug("order_id", fields, LANGS, "cyr")).toBe("order_id");
});

test("нет варианта на активном языке — слаг остаётся: показывается он же", () => {
  // Язык добавили в проект позже поля: колонка показывает title_en
  // (см. fieldsForLanguage), и условие должно бить по ней же.
  expect(localizeSlug("title_en", [field("title_en", true)], LANGS, "cyr")).toBe("title_en");
});

test("переезд фильтров сохраняет значения и не давит чужой ключ", () => {
  const fields = [field("title_en", true), field("title_cyr", true)];

  expect(localizeKeys({ title_en: 1, price: 2 }, fields, LANGS, "cyr")).toEqual({
    title_cyr: 1,
    price: 2,
  });
  // В адресе лежали оба варианта: выкинуть заданное условие хуже,
  // чем оставить его на старом языке.
  expect(localizeKeys({ title_en: 1, title_cyr: 2 }, fields, LANGS, "cyr")).toEqual({
    title_en: 1,
    title_cyr: 2,
  });
});

/*
 * Подпись читается по двум осям: локаль интерфейса, затем язык данных.
 * Тест держит именно порядок — из-за него переключение языка интерфейса
 * и переводит колонки, и не ломает проекты с кодами вне ru/en/uz.
 */
test("подпись: локаль интерфейса, потом язык данных, потом запасная", () => {
  // i18n в тестах стартует на ru (см. shared/lib/i18n).
  expect(localized({ ru: "Заказы", cyr: "Буюртмалар" }, "cyr", "orders")).toBe("Заказы");
  expect(localized({ cyr: "Буюртмалар" }, "cyr", "orders")).toBe("Буюртмалар");
  expect(localized({}, "cyr", "orders")).toBe("orders");
  // Пустая строка — не подпись: у половины полей лежит `label_ru: ""`.
  expect(localized({ ru: "  " }, "cyr", "orders")).toBe("orders");
});

/*
 * В карточке порядок обратный: вкладку языка человек нажал только что,
 * и подпись обязана поехать за ней. Пока этого не было, переключатель
 * работал через раз — у поля с заполненным `label_ru` (а коды проекта
 * с ru/en/uz совпадают часто) подпись намертво вставала на локали,
 * у соседнего без `label_ru` — менялась.
 */
test("подпись в карточке: сначала выбранная вкладка", () => {
  expect(tabLabel({ ru: "Заказы", cyr: "Буюртмалар" }, "cyr", "orders", true)).toBe("Буюртмалар");
  // Вкладок нет — правило общее, как в таблице.
  expect(tabLabel({ ru: "Заказы", cyr: "Буюртмалар" }, "cyr", "orders", false)).toBe("Заказы");
  // На выбранной вкладке подписи нет — выручает локаль интерфейса.
  expect(tabLabel({ ru: "Заказы" }, "cyr", "orders", true)).toBe("Заказы");
  expect(tabLabel({ cyr: "  ", ru: "Заказы" }, "cyr", "orders", true)).toBe("Заказы");
  expect(tabLabel({}, "cyr", "orders", true)).toBe("orders");
});
