import { expect, test } from "vitest";
import { formatCoords, mapLink, parseCoords } from "./coords";

test("координаты разбираются из «широта,долгота»", () => {
  expect(parseCoords("41.311081,69.240562")).toEqual({ lat: 41.311081, lon: 69.240562 });
  expect(parseCoords(" -12.5 , 130.75 ")).toEqual({ lat: -12.5, lon: 130.75 });
});

/*
 * Колонка — обычный VARCHAR, и в ней бывает что угодно: текст из
 * прежнего типа поля, одно число, координаты за пределами глобуса.
 * Всё это — «не точка», и ячейка обязана показать текст как есть,
 * а не ссылку в никуда.
 */
test("что угодно, кроме пары координат, — не точка", () => {
  expect(parseCoords("Ташкент")).toBeNull();
  expect(parseCoords("41.31")).toBeNull();
  expect(parseCoords("41.31,69.24,100")).toBeNull();
  expect(parseCoords("200,500")).toBeNull();
  expect(parseCoords("")).toBeNull();
  expect(parseCoords(null)).toBeNull();
});

test("пустая половина стирает значение целиком", () => {
  expect(formatCoords("41.31", "69.24")).toBe("41.31,69.24");
  expect(formatCoords("41.31", " ")).toBe("");
  expect(formatCoords("", "")).toBe("");
});

test("ссылка на карту", () => {
  expect(mapLink({ lat: 41.31, lon: 69.24 })).toBe("https://yandex.com/maps/?text=41.31%2C69.24");
});
